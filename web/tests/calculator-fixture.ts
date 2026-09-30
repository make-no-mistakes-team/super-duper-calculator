import { spawn } from 'node:child_process';
import { mkdtemp, rm } from 'node:fs/promises';
import { tmpdir } from 'node:os';
import { join } from 'node:path';
import { fileURLToPath } from 'node:url';
import { test as base, expect, type Page, type Route } from '@playwright/test';
import type { CalculationResponse } from '../src/contracts';

const root = fileURLToPath(new URL('../../', import.meta.url));

export const test = base.extend<{ calculatorURL: string }>({
  calculatorURL: async ({}, use, testInfo) => {
    const state = await mkdtemp(join(tmpdir(), 'calculator-browser-'));
    const server = spawn(join(root, 'bin', 'calculator'), [], {
      cwd: state,
      env: {
        ...process.env,
        HTTP_ADDR: '127.0.0.1:0',
        DATABASE_PATH: join(state, 'calculator.sqlite'),
        WEB_ASSETS_DIR: join(root, 'bin', 'web'),
        PUBLIC_ORIGIN: '',
        STATISTICS_ENABLED: 'true',
        ACHIEVEMENTS_ENABLED: 'true',
      },
      stdio: ['ignore', 'pipe', 'pipe'],
    });
    let logs = '';
    const closed = new Promise<void>((resolve) => server.once('close', () => resolve()));
    try {
      const url = await new Promise<string>((resolve, reject) => {
        const timeout = setTimeout(() => reject(new Error(`Go server startup timed out.\n${logs}`)), 10_000);
        const finish = (error?: Error, address?: string) => {
          clearTimeout(timeout);
          if (error) reject(error);
          else resolve(address!);
        };
        server.once('error', (error) => finish(error));
        server.once('exit', (code, signal) => finish(new Error(`Go server exited: ${code ?? signal}.\n${logs}`)));
        const observe = (chunk: Buffer) => {
          logs += chunk.toString();
          const address = logs.match(/HTTP listening on (http:\/\/127\.0\.0\.1:\d+)/)?.[1];
          if (address) finish(undefined, address);
        };
        server.stdout.on('data', observe);
        server.stderr.on('data', observe);
      });
      await expect.poll(async () => {
        const response = await fetch(`${url}/health/ready`);
        return response.status;
      }).toBe(200);
      await use(url);
    } finally {
      if (server.exitCode === null && server.signalCode === null) server.kill('SIGTERM');
      const force = setTimeout(() => server.kill('SIGKILL'), 6_000);
      try {
        await closed;
      } finally {
        clearTimeout(force);
        await rm(state, { recursive: true, force: true });
        if (testInfo.status !== testInfo.expectedStatus) {
          await testInfo.attach('go-server.log', { body: logs, contentType: 'text/plain' });
        }
      }
    }
  },
  baseURL: async ({ calculatorURL }, use) => use(calculatorURL),
});

export { expect };

export async function openCalculator(page: Page) {
  const history = page.waitForResponse((response) => new URL(response.url()).pathname === '/api/history');
  const capabilities = page.waitForResponse((response) => new URL(response.url()).pathname === '/api/capabilities');
  await page.goto('/');
  expect((await history).status()).toBe(200);
  expect((await capabilities).status()).toBe(200);
  await expect(page.locator('#expression')).toBeEditable();
}

export async function calculate(page: Page, expression: string): Promise<CalculationResponse> {
  const input = page.locator('#expression');
  if (await input.inputValue() !== expression) await input.fill(expression);
  const reply = page.waitForResponse((response) =>
    new URL(response.url()).pathname === '/api/calculations' && response.request().method() === 'POST');
  await input.press('Enter');
  const response = await reply;
  expect(response.status()).toBe(200);
  const data: CalculationResponse = await response.json();
  await expect(page.locator('.result-source code')).toHaveText(expression);
  if (data.calculation.outcome.kind === 'success') await expect(page.locator('.result-value')).toBeVisible();
  else await expect(page.locator('.result-error')).toBeVisible();
  return data;
}

// Delay delivery, not evaluation: every held body and header comes from the Go service.
export async function holdReply(page: Page, pattern: string, matches: (route: Route) => boolean = () => true) {
  let release!: () => void;
  const gate = new Promise<void>((resolve) => { release = resolve; });
  let arrived!: () => void;
  const received = new Promise<void>((resolve) => { arrived = resolve; });
  let delivered!: () => void;
  const settled = new Promise<void>((resolve) => { delivered = resolve; });
  let held = false;
  await page.route(pattern, async (route) => {
    if (held || !matches(route)) return route.continue();
    held = true;
    try {
      const response = await route.fetch();
      expect(response.status()).toBe(200);
      arrived();
      await gate;
      await route.fulfill({ response });
    } finally {
      delivered();
    }
  });
  return {
    received,
    release,
    async deliver() {
      release();
      await settled;
      // Allow the browser's response handler and React render to complete before inspecting stale state.
      await page.evaluate(() => new Promise<void>((resolve) => requestAnimationFrame(() => requestAnimationFrame(() => resolve()))));
    },
  };
}
