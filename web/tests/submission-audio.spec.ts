import type { Page } from '@playwright/test';
import type { CalculationRequest, CalculationResponse } from '../src/contracts';
import { calculate, expect, openCalculator, test } from './calculator-fixture';

type VoiceSchedule = { start: number | null; stops: number[]; ended: boolean };

declare global {
  interface Window {
    calculatorAudioProbe: { contexts: AudioContext[]; voices: VoiceSchedule[] };
  }
}

// Observe scheduling on real native nodes. Neither the context nor its output is mocked.
async function observeAudio(page: Page) {
  await page.addInitScript(() => {
    const probe = { contexts: [] as AudioContext[], voices: [] as VoiceSchedule[] };
    window.calculatorAudioProbe = probe;
    const createOscillator = AudioContext.prototype.createOscillator;
    AudioContext.prototype.createOscillator = function () {
      const voice = createOscillator.call(this);
      if (!probe.contexts.includes(this)) probe.contexts.push(this);
      const schedule: VoiceSchedule = { start: null, stops: [], ended: false };
      probe.voices.push(schedule);
      const start = voice.start.bind(voice);
      const stop = voice.stop.bind(voice);
      voice.start = (at = 0) => {
        start(at);
        schedule.start = at;
      };
      voice.stop = (at = 0) => {
        stop(at);
        schedule.stops.push(at || this.currentTime);
      };
      voice.addEventListener('ended', () => { schedule.ended = true; });
      return voice;
    };
  });
}

async function voices(page: Page) {
  return page.evaluate(() => window.calculatorAudioProbe.voices);
}

test('intentional dispatch sounds once across controls, but blank, composition, browsing, mute and request retry stay silent', async ({ page }) => {
  await observeAudio(page);
  await openCalculator(page);
  const editor = page.locator('#expression');
  // A trusted non-submission gesture unlocks the native device before assertions.
  await page.locator('#tool-functions').click();
  await page.locator('#tool-bay-heading').press('Escape');
  await editor.press('Enter');
  await editor.fill('2+3');
  await editor.press('Shift+Enter');
  await editor.dispatchEvent('keydown', { key: 'Enter', isComposing: true, bubbles: true });
  await editor.dispatchEvent('keydown', { key: 'Enter', keyCode: 229, bubbles: true });
  expect(await voices(page)).toEqual([]);

  await calculate(page, '2+3');
  await expect.poll(async () => (await voices(page)).length).toBe(1);
  const [first] = await voices(page);
  if (!first) throw new Error('Expected a real submission oscillator');
  expect(first.start).not.toBeNull();
  expect(await page.evaluate(() => window.calculatorAudioProbe.contexts.length)).toBe(1);
  const duration = first.stops[0]! - first.start!;
  expect(duration).toBeGreaterThanOrEqual(.05);
  expect(duration).toBeLessThanOrEqual(.08);

  await editor.fill('3+4');
  let response = page.waitForResponse((reply) => new URL(reply.url()).pathname === '/api/calculations');
  await page.locator('.editor-console .calculator-submit').click();
  expect((await response).status()).toBe(200);
  await expect(page.locator('.result-value')).toHaveText('= 7');
  expect((await voices(page)).length).toBe(2);

  await editor.fill('4+5');
  await page.locator('#tool-keypad').click();
  response = page.waitForResponse((reply) => new URL(reply.url()).pathname === '/api/calculations');
  await page.locator('#tool-bay .calculator-submit').click();
  expect((await response).status()).toBe(200);
  await expect(page.locator('.result-value')).toHaveText('= 9');
  expect((await voices(page)).length).toBe(3);
  expect((await calculate(page, '1/0')).calculation.outcome.kind).toBe('error');
  expect((await voices(page)).length).toBe(4);

  const attempts: CalculationRequest[] = [];
  let accepted: CalculationResponse | undefined;
  await page.route('**/api/calculations', async (route) => {
    const request = route.request().postDataJSON() as CalculationRequest;
    attempts.push(request);
    if (attempts.length !== 1) return route.continue();
    // Lose delivery after the real service accepted the request: retry must reuse it.
    const reply = await route.fetch();
    expect(reply.status()).toBe(200);
    accepted = await reply.json();
    await route.abort('failed');
  });
  await editor.fill('9+10');
  await editor.press('Enter');
  const retry = page.locator('.calculation-result .secondary-button');
  await expect(retry).toBeVisible();
  expect((await voices(page)).length).toBe(5);
  response = page.waitForResponse((reply) => new URL(reply.url()).pathname === '/api/calculations');
  await retry.click();
  const repeated: CalculationResponse = await (await response).json();
  expect(repeated.calculation.id).toBe(accepted?.calculation.id);
  expect(attempts).toHaveLength(2);
  expect(attempts[1]).toEqual(attempts[0]);
  await expect(page.locator('.result-value')).toHaveText('= 19');
  expect((await voices(page)).length).toBe(5);
  await page.unroute('**/api/calculations');

  await page.locator('#header-achievements').click();
  await page.keyboard.press('Escape');
  await page.locator('#sound-toggle').click();
  await calculate(page, '7+8');
  expect((await voices(page)).length).toBe(5);
  await page.locator('#sound-toggle').click();
  expect((await voices(page)).length).toBe(5);
});

test('rapid dispatch replaces its short voice; award audio suppresses submissions and global mute stops all voices without replay', async ({ page }) => {
  await observeAudio(page);
  await openCalculator(page);
  await page.locator('#tool-functions').click();
  await page.locator('#tool-bay-heading').press('Escape');
  await page.locator('#expression').fill('5+6');
  // Two intentional Enter actions in the same browser turn exercise the overlap boundary.
  await page.locator('#expression').evaluate((editor) => {
    for (let action = 0; action < 2; action++) {
      editor.dispatchEvent(new KeyboardEvent('keydown', { key: 'Enter', bubbles: true }));
    }
  });
  await expect(page.locator('.result-value')).toHaveText('= 11');
  const rapid = await voices(page);
  expect(rapid).toHaveLength(2);
  const [previous, current] = rapid;
  if (!previous || !current) throw new Error('Expected both rapid dispatch voices');
  expect(previous.stops).toHaveLength(2);
  expect(previous.stops[1]).toBeLessThanOrEqual(current.start!);

  await calculate(page, '6*7');
  await expect(page.getByRole('complementary', { name: 'Новое достижение', exact: true })).toBeVisible();
  await expect.poll(async () => (await voices(page)).length).toBe(8);
  await calculate(page, '2+8');
  expect((await voices(page)).length).toBe(8);
  await page.locator('#sound-toggle').click();
  await expect.poll(async () => (await voices(page)).every((voice) => voice.ended)).toBe(true);
  const muted = await voices(page);
  const fanfare = muted.slice(3);
  expect(fanfare).toHaveLength(5);
  const finalNote = fanfare[4];
  if (!finalNote) throw new Error('Expected the final award note');
  expect(finalNote.stops).toHaveLength(2);
  await page.locator('#sound-toggle').click();
  // A subsequent foreground action must not replay the still-visible award signal.
  await page.locator('#tool-functions').click();
  expect(await voices(page)).toEqual(muted);
});
