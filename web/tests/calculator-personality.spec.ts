import { randomUUID } from 'node:crypto';
import type { Page } from '@playwright/test';
import type { CalculationResponse, Capabilities } from '../src/contracts';
import { PHRASE_CATALOG } from '../src/features/discoveries/phraseCatalog';
import type { PhraseContext } from '../src/features/discoveries/phraseCatalog';
import { calculate, expect, holdReply, openCalculator, test } from './calculator-fixture';

declare global {
  interface Window {
    calculatorSpeechViolations: Set<string>;
    calculatorUnwantedSpeech: boolean;
  }
}

async function deterministicSpeech(page: Page, viewport = { width: 1440, height: 1000 }) {
  // Keep room for subsequent, differently sized phrases; cramped-layout behavior
  // is exercised separately rather than requiring chatter where no slot fits.
  await page.setViewportSize(viewport);
  // Exercise the real selection path, not injected phrases or fabricated API replies.
  await page.addInitScript(() => { Math.random = () => 0; });
  await page.clock.install();
}

async function watchSpeechSafety(page: Page) {
  await page.evaluate(() => {
    const violations = new Set<string>();
    window.calculatorSpeechViolations = violations;
    const visible = (element: Element) => {
      if (element.closest('[data-exit-snapshot]')) return false;
      const rect = element.getBoundingClientRect();
      const style = getComputedStyle(element);
      return element.getClientRects().length && rect.width > 0 && rect.height > 0 &&
        style.visibility !== 'hidden' && style.opacity !== '0';
    };
    const inspect = () => {
      const bubble = document.querySelector('.calculator-speech');
      if (bubble && bubble.getAttribute('aria-hidden') !== 'true' && visible(bubble)) {
        const bounds = bubble.getBoundingClientRect();
        const viewport = window.visualViewport;
        const left = viewport?.offsetLeft ?? 0;
        const top = viewport?.offsetTop ?? 0;
        const right = left + (viewport?.width ?? innerWidth);
        const bottom = top + (viewport?.height ?? innerHeight);
        if (bounds.left < left || bounds.top < top || bounds.right > right || bounds.bottom > bottom) {
          violations.add('speech outside the visible viewport');
        }
        // Query actual controls rather than trusting data-speech-protected wiring.
        for (const element of document.querySelectorAll('#expression, .editor-options, .editor-actions, .calculation-result > div, .calculation-result > .result-pending, button, .workspace-brand, .workspace-header-actions, #tool-bay')) {
          if (!visible(element)) continue;
          const protectedBounds = element.getBoundingClientRect();
          if (bounds.left < protectedBounds.right && bounds.right > protectedBounds.left &&
            bounds.top < protectedBounds.bottom && bounds.bottom > protectedBounds.top) {
            violations.add(`speech overlaps ${element.id || element.className || element.tagName}`);
          }
        }
        if (document.querySelector('.achievement-ceremony[data-achievement-id], .comic-incident, dialog[open], [aria-modal="true"]')) {
          violations.add('speech competes with an award, scene or modal');
        }
      }
      requestAnimationFrame(inspect);
    };
    requestAnimationFrame(inspect);
  });
}

async function expectSafeSpeech(page: Page) {
  const violations = await page.evaluate(() => [...window.calculatorSpeechViolations]);
  expect(violations).toEqual([]);
}

async function watchUnwantedSpeech(page: Page) {
  await page.evaluate(() => {
    window.calculatorUnwantedSpeech = false;
    const inspect = () => {
      const bubble = document.querySelector('.calculator-speech');
      if (bubble && bubble.getAttribute('aria-hidden') !== 'true') window.calculatorUnwantedSpeech = true;
    };
    new MutationObserver(inspect).observe(document.body, {
      childList: true, attributes: true, subtree: true,
    });
    const frame = () => { inspect(); requestAnimationFrame(frame); };
    requestAnimationFrame(frame);
  });
}

async function expectNoSpeech(page: Page) {
  await expect(page.locator('.calculator-speech')).toHaveCount(0);
  expect(await page.evaluate(() => window.calculatorUnwantedSpeech)).toBe(false);
}

async function speechDraws(page: Page, values: number[]) {
  await page.evaluate((draws) => {
    Math.random = () => draws.shift() ?? 0;
  }, values);
}

async function expectContextSpeech(page: Page, context: PhraseContext) {
  const speech = page.locator('.calculator-speech');
  await expect(speech).toBeVisible();
  // This is a semantic eligibility assertion against the authored pool, not a
  // wording snapshot: an error must never pick a success or unrelated diagnosis.
  const text = await speech.innerText();
  expect(PHRASE_CATALOG.filter((phrase) => phrase.contexts.includes(context)).map((phrase) => phrase.ru)).toContain(text);
  return text;
}

async function seedAccepted(page: Page, expressions: string[]) {
  expect((await page.request.get('/api/session')).status()).toBe(200);
  for (const expression of expressions) {
    const reply = await page.request.post('/api/calculations', {
      data: { requestId: randomUUID(), expression },
    });
    expect(reply.status()).toBe(200);
  }
}

async function acceptThroughLostReply(page: Page, expression: string) {
  const accepted: { reply?: CalculationResponse } = {};
  await page.route('**/api/calculations', async (route) => {
    const reply = await route.fetch();
    expect(reply.status()).toBe(200);
    accepted.reply = await reply.json();
    await route.abort('failed');
  });
  try {
    await page.locator('#expression').fill(expression);
    await page.locator('#expression').press('Enter');
    const retry = page.locator('.calculation-result .secondary-button');
    await expect(retry).toBeEnabled();
    await page.unroute('**/api/calculations');
    const reply = page.waitForResponse((response) => new URL(response.url()).pathname === '/api/calculations');
    await retry.click();
    const response = await reply;
    expect(response.status()).toBe(200);
    const retried: CalculationResponse = await response.json();
    expect(retried.calculation).toEqual(accepted.reply?.calculation);
    if (retried.calculation.outcome.kind === 'success') await expect(page.locator('.result-value')).toBeVisible();
    else await expect(page.locator('.result-error')).toBeVisible();
    return retried;
  } finally {
    await page.unroute('**/api/calculations');
  }
}

// Chromium's headless tab focus is not a reliable document.hidden switch.
// Deliver the browser's visibility boundary with a deterministic clock epoch;
// calculation bodies and acceptance still come exclusively from the Go API.
async function visibilityBoundary(page: Page, foreground: boolean) {
  await page.evaluate((visible) => {
    Object.defineProperty(document, 'hidden', { configurable: true, get: () => !visible });
    document.dispatchEvent(new Event('visibilitychange'));
  }, foreground);
}

for (const { expression, code, stage, context, hasFacts } of [
  { expression: 'unknown(1)', code: 'UNKNOWN_IDENTIFIER', stage: 'parse', context: 'math_error', hasFacts: false },
  { expression: 'sin(1,2)', code: 'WRONG_ARITY', stage: 'parse', context: 'math_error', hasFacts: false },
  { expression: '1e400', code: 'NUMERIC_OVERFLOW', stage: 'parse', context: 'math_error', hasFacts: false },
  { expression: 'exp(1000)', code: 'NUMERIC_OVERFLOW', stage: 'evaluate', context: 'math_error', hasFacts: true },
  { expression: '1+', code: 'SYNTAX_ERROR', stage: 'parse', context: 'syntax', hasFacts: false },
  { expression: '9/0', code: 'DIVISION_BY_ZERO', stage: 'evaluate', context: 'division_zero', hasFacts: true },
  { expression: 'sqrt(-1)', code: 'DOMAIN_ERROR', stage: 'evaluate', context: 'domain', hasFacts: true },
] as const) {
  test(`${code} at ${stage} can speak honestly while preserving the real error`, async ({ page }) => {
    await deterministicSpeech(page);
    await openCalculator(page);
    await watchSpeechSafety(page);
    const reply = await calculate(page, expression);
    expect(reply.calculation.outcome).toMatchObject({ kind: 'error', error: { code, stage } });
    if (hasFacts) expect(reply.calculation.facts).toBeDefined();
    else expect(reply.calculation.facts).toBeUndefined();
    await expectContextSpeech(page, context);
    await expect(page.locator('.result-error')).toBeVisible();
    await expect(page.locator('.result-value')).toHaveCount(0);
    await expect(page.locator('#expression')).toBeFocused();
    await page.clock.runFor(6_000);
    await expect(page.locator('.calculator-speech')).toHaveCount(0);
    await expect(page.locator('.result-error')).toBeVisible();
    await expectSafeSpeech(page);
  });
}

for (const { context, expression, value, draws, seed } of [
  { context: 'ordinary', expression: '8*9', value: '72', draws: [0.5, 0.5, 0], seed: [] },
  { context: 'ordinary', expression: '2+3+4+5+6+7+8+9', value: '44', draws: [0.5, 0.5, 0.9], seed: [] },
  { context: 'ordinary', expression: '((8*9))', value: '72', draws: [0.5, 0.5, 0.9], seed: [] },
  { context: 'ordinary', expression: 'pi', value: '3.141592653589793', draws: [0.5, 0.5, 0.9], seed: [] },
  { context: 'ordinary', expression: '1e12', value: '1e+12', draws: [0.5, 0.5, 0.9], seed: [] },
  { context: 'ordinary', expression: '1e-12', value: '1e-12', draws: [0.5, 0.5, 0.9], seed: [] },
  { context: 'complex', expression: '2+3+4+5+6+7+8+9+10', value: '54', draws: [0.5, 0.5, 0.5], seed: [] },
  { context: 'functions', expression: 'abs(-12)', value: '12', draws: [0.5, 0.5, 0.5], seed: [] },
  { context: 'nesting', expression: '(((8*9)))', value: '72', draws: [0.5, 0.5, 0.5], seed: [] },
  { context: 'zero', expression: '8-8', value: '0', draws: [0.5, 0.5, 0.5], seed: [] },
  { context: 'zero', expression: '1e-400', value: '0', draws: [0.5, 0.5, 0.9], seed: [] },
  { context: 'zero', expression: '0*-1', value: '0', draws: [0.5, 0.5, 0.9], seed: [] },
  { context: 'negative', expression: '8-9', value: '-1', draws: [0.5, 0.5, 0.5], seed: [] },
  { context: 'huge', expression: '-1e13', value: '-1e+13', draws: [0.5, 0.5, 0.9], seed: ['1e13'] },
  { context: 'tiny', expression: '-1e-13', value: '-1e-13', draws: [0.5, 0.5, 0.9], seed: ['1e-13'] },
  { context: 'philosophy', expression: '8*9', value: '72', draws: [0.01, 0.5, 0.9], seed: [] },
  { context: 'hype', expression: '8*9', value: '72', draws: [0.5, 0.01, 0.9], seed: [] },
] as const) {
  test(`${context} commentary for ${expression} follows real result and parser evidence`, async ({ page }) => {
    await deterministicSpeech(page);
    // Quietly own the size award before opening the UI; its ceremony must not
    // be disabled just to make an ordinary commentary test pass.
    await seedAccepted(page, [...seed]);
    await openCalculator(page);
    await speechDraws(page, [...draws, 0, 0]);
    const reply = await calculate(page, expression);
    expect(reply.calculation.outcome).toEqual({ kind: 'success', value });
    if (context === 'complex') expect(reply.calculation.facts?.operationCount).toBeGreaterThanOrEqual(8);
    if (context === 'functions') expect(reply.calculation.facts?.functions.abs).toBe(1);
    if (context === 'nesting') expect(reply.calculation.facts?.depth).toBe(3);
    await expectContextSpeech(page, context);
    await expect(page.locator('.result-source code')).toHaveText(expression);
    await expect(page.locator('#expression')).toBeFocused();
  });
}

test('repeat commentary recognizes the engine-normalized expression without claiming repetition of unrelated math', async ({ page }) => {
  await deterministicSpeech(page);
  await openCalculator(page);
  const first = await calculate(page, '8*9');
  await expectContextSpeech(page, 'ordinary');
  await page.clock.runFor(6_000);
  await expect(page.locator('.calculator-speech')).toHaveCount(0);
  await speechDraws(page, [0.5, 0.5, 0.5, 0, 0]);
  const repeated = await calculate(page, '((8 * 9))');
  expect(repeated.calculation.facts?.normalizedExpression).toBe(first.calculation.facts?.normalizedExpression);
  await expectContextSpeech(page, 'repeat');
  await page.clock.runFor(6_000);
  await expect(page.locator('.calculator-speech')).toHaveCount(0);
  await speechDraws(page, [0.5, 0.5, 0.5, 0, 0]);
  const distinct = await calculate(page, '9*8');
  expect(distinct.calculation.facts?.normalizedExpression).not.toBe(first.calculation.facts?.normalizedExpression);
  await expectContextSpeech(page, 'ordinary');
});

test('recovery requires a preceding accepted mathematical error, including codes without parser facts', async ({ page }) => {
  await deterministicSpeech(page);
  await seedAccepted(page, ['1+', '10']);
  await openCalculator(page);
  const error = await calculate(page, 'unknown(1)');
  expect(error.calculation.outcome).toMatchObject({ kind: 'error', error: { code: 'UNKNOWN_IDENTIFIER' } });
  await expectContextSpeech(page, 'math_error');
  await page.clock.runFor(6_000);
  await expect(page.locator('.calculator-speech')).toHaveCount(0);
  await speechDraws(page, [0.5, 0.5, 0.5, 0, 0]);
  const recovered = await calculate(page, '8*9');
  expect(recovered.calculation.outcome).toEqual({ kind: 'success', value: '72' });
  await expectContextSpeech(page, 'recovery');
});

test('a quiet accepted retry updates recovery and normalized-repeat context without speaking', async ({ page }) => {
  await deterministicSpeech(page);
  await seedAccepted(page, ['1+', '10']);
  await openCalculator(page);
  await calculate(page, 'unknown(1)');
  await expectContextSpeech(page, 'math_error');
  await page.clock.runFor(6_000);
  await expect(page.locator('.calculator-speech')).toHaveCount(0);
  await watchUnwantedSpeech(page);
  const retried = await acceptThroughLostReply(page, '8*9');
  expect(retried.calculation.outcome).toEqual({ kind: 'success', value: '72' });
  await page.clock.runFor(6_000);
  await expectNoSpeech(page);
  await speechDraws(page, [0.5, 0.5, 0.8, 0, 0]);
  await calculate(page, '3*9');
  await expectContextSpeech(page, 'ordinary');
  await page.clock.runFor(6_000);
  await expect(page.locator('.calculator-speech')).toHaveCount(0);
  await speechDraws(page, [0.5, 0.5, 0.5, 0, 0]);
  await calculate(page, '((8 * 9))');
  await expectContextSpeech(page, 'repeat');
});

test('an accepted mathematical error delivered by quiet retry still makes the next success a recovery', async ({ page }) => {
  await deterministicSpeech(page);
  await seedAccepted(page, ['1+', '10']);
  await openCalculator(page);
  await calculate(page, '8*9');
  await expectContextSpeech(page, 'ordinary');
  await page.clock.runFor(6_000);
  await expect(page.locator('.calculator-speech')).toHaveCount(0);
  await watchUnwantedSpeech(page);
  const retried = await acceptThroughLostReply(page, 'unknown(1)');
  expect(retried.calculation.outcome).toMatchObject({ kind: 'error', error: { code: 'UNKNOWN_IDENTIFIER' } });
  await page.clock.runFor(6_000);
  await expectNoSpeech(page);
  await speechDraws(page, [0.5, 0.5, 0.5, 0, 0]);
  await calculate(page, '3*9');
  await expectContextSpeech(page, 'recovery');
});

for (const { delay, speaks } of [
  // Leave room for real browser dispatch around the controlled clock advance.
  { delay: 14_000, speaks: true },
  { delay: 15_000, speaks: false },
]) {
  test(`a held real response at ${delay}ms respects speech freshness without losing accepted context`, async ({ page }) => {
    await deterministicSpeech(page);
    await openCalculator(page);
    const delayed = await holdReply(page, '**/api/calculations');
    try {
      await page.locator('#expression').fill('8*9');
      await page.locator('#expression').press('Enter');
      await delayed.received;
      await page.clock.runFor(delay);
      if (!speaks) await watchUnwantedSpeech(page);
      await delayed.deliver();
      await expect(page.locator('.result-value')).toHaveText('= 72');
      if (speaks) await expectContextSpeech(page, 'ordinary');
      else await expectNoSpeech(page);
      await page.clock.runFor(6_000);
      await expect(page.locator('.calculator-speech')).toHaveCount(0);
      await speechDraws(page, [0.5, 0.5, 0.5, 0, 0]);
      await calculate(page, '((8 * 9))');
      await expectContextSpeech(page, 'repeat');
    } finally {
      delayed.release();
    }
  });
}

test('hidden outcomes and pre-resume held replies stay silent, but accepted context remains current', async ({ page }) => {
  await deterministicSpeech(page);
  await seedAccepted(page, ['1+', '10']);
  await openCalculator(page);
  const delayed = await holdReply(page, '**/api/calculations', (route) => route.request().postDataJSON().expression === '8*9');
  try {
    await page.locator('#expression').fill('8*9');
    await page.locator('#expression').press('Enter');
    await delayed.received;
    await page.clock.runFor(1);
    await visibilityBoundary(page, false);
    await watchUnwantedSpeech(page);
    const hidden = await calculate(page, 'unknown(1)');
    expect(hidden.calculation.outcome).toMatchObject({ kind: 'error', error: { code: 'UNKNOWN_IDENTIFIER' } });
    await page.clock.runFor(1);
    await visibilityBoundary(page, true);
    await delayed.deliver();
    await page.clock.runFor(6_000);
    await expectNoSpeech(page);
    await expect(page.locator('.result-error')).toBeVisible();
    await speechDraws(page, [0.5, 0.5, 0.5, 0, 0]);
    await calculate(page, '3*9');
    await expectContextSpeech(page, 'recovery');
  } finally {
    delayed.release();
  }
});

test('a newest reply accepted before the visibility epoch cannot speak after resume', async ({ page }) => {
  await deterministicSpeech(page);
  await openCalculator(page);
  const delayed = await holdReply(page, '**/api/calculations');
  try {
    await page.locator('#expression').fill('8*9');
    await page.locator('#expression').press('Enter');
    await delayed.received;
    await page.clock.runFor(1);
    await visibilityBoundary(page, false);
    await page.clock.runFor(1);
    await visibilityBoundary(page, true);
    await watchUnwantedSpeech(page);
    await delayed.deliver();
    await expect(page.locator('.result-value')).toHaveText('= 72');
    await page.clock.runFor(6_000);
    await expectNoSpeech(page);
    await speechDraws(page, [0.5, 0.5, 0.5, 0, 0]);
    await calculate(page, '((8 * 9))');
    await expectContextSpeech(page, 'repeat');
  } finally {
    delayed.release();
  }
});

for (const { expression, code, context } of [
  { expression: '1+', code: 'SYNTAX_ERROR', context: 'syntax' },
  { expression: 'unknown(1)', code: 'UNKNOWN_IDENTIFIER', context: 'math_error' },
] as const) {
  test(`${code} does not repeat recent phrases or replay a temporarily exhausted pool`, async ({ page }) => {
    await deterministicSpeech(page);
    await openCalculator(page);
    const seen = new Set<string>();
    const eligible = PHRASE_CATALOG.filter((phrase) => phrase.contexts.includes(context));
    for (let turn = 0; turn < eligible.length; turn++) {
      const reply = await calculate(page, expression);
      expect(reply.calculation.outcome).toMatchObject({ kind: 'error', error: { code } });
      const text = await expectContextSpeech(page, context);
      expect(seen.has(text)).toBe(false);
      seen.add(text);
      await expect(page.locator('.result-error')).toBeVisible();
      await page.clock.runFor(6_000);
      await expect(page.locator('.calculator-speech')).toHaveCount(0);
    }
    await watchUnwantedSpeech(page);
    const exhausted = await calculate(page, expression);
    expect(exhausted.calculation.outcome).toMatchObject({ kind: 'error', error: { code } });
    await page.clock.runFor(6_000);
    await expectNoSpeech(page);
    await expect(page.locator('.result-error')).toBeVisible();
  });
}

test('rapid pending and accepted computations keep the same speech until its original deadline while the face follows current math', async ({ page }) => {
  await deterministicSpeech(page);
  await seedAccepted(page, ['1+', '10']);
  await openCalculator(page);
  // Pause at a future boundary so dispatch cannot race the running clock.
  await page.clock.pauseAt(await page.evaluate(() => Date.now() + 1_000));
  await calculate(page, '8*9');
  const text = await expectContextSpeech(page, 'ordinary');
  const speech = page.locator('.calculator-speech');
  await expect(speech).toHaveAttribute('data-phase', 'entering');
  const continuity = await page.evaluateHandle(() => {
    const bubble = document.querySelector('.calculator-speech')!;
    const text = bubble.textContent;
    const state = { changed: false };
    new MutationObserver((records) => {
      if (document.querySelector('.calculator-speech') !== bubble || bubble.textContent !== text ||
        records.some((record) => [...record.removedNodes].some((node) => node === bubble || node.contains(bubble)))) {
        state.changed = true;
      }
    }).observe(document.body, { subtree: true, childList: true, characterData: true });
    return state;
  });
  const delayed = await holdReply(page, '**/api/calculations', (route) => route.request().postDataJSON().expression === 'unknown(1)');
  try {
    await page.clock.runFor(500);
    await page.locator('#expression').fill('unknown(1)');
    await page.locator('#expression').press('Enter');
    await delayed.received;
    await expect(page.locator('.result-pending code')).toHaveText('unknown(1)');
    await expect(page.locator('.calculator-personality__pixels')).toHaveAttribute('data-mood', 'loading');
    await expect(speech).toHaveText(text);
    await page.clock.runFor(500);
    await expect(speech).toBeVisible();
    delayed.release();
    await expect(page.locator('.result-error')).toBeVisible();
    await expect(page.locator('.calculator-personality__pixels')).toHaveAttribute('data-mood', 'error');
    await expect(speech).toHaveText(text);
    await page.clock.runFor(500);
    await calculate(page, '3*9');
    await expect(page.locator('.calculator-personality__pixels')).toHaveAttribute('data-mood', 'recovery');
    await expect(speech).toHaveText(text);
    await page.clock.runFor(500);
    await calculate(page, 'unknown(1)');
    await expect(page.locator('.calculator-personality__pixels')).toHaveAttribute('data-mood', 'error');
    await expect(speech).toHaveText(text);
    await page.clock.runFor(2_499);
    await expect(speech).toBeVisible();
    await expect(speech).toHaveAttribute('data-phase', 'entering');
    expect(await continuity.evaluate((state) => state.changed)).toBe(false);
    await page.clock.runFor(1);
    await expect(speech).toHaveAttribute('data-phase', 'leaving');
    await page.clock.runFor(240);
    await expect(speech).toHaveCount(0);
    await watchUnwantedSpeech(page);
    await page.clock.runFor(1_000);
    await expectNoSpeech(page);
    // Outcomes observed while the old bubble was active still drive recovery
    // and normalized repetition, without having queued their own utterances.
    await speechDraws(page, [0.5, 0.5, 0.5, 0, 0]);
    await calculate(page, '5*9');
    await expectContextSpeech(page, 'recovery');
    await page.clock.runFor(6_000);
    await expect(speech).toHaveCount(0);
    await speechDraws(page, [0.5, 0.5, 0.5, 0, 0]);
    await calculate(page, '((3 * 9))');
    await expectContextSpeech(page, 'repeat');
  } finally {
    delayed.release();
    await continuity.dispose();
  }
});

for (const { expression, nextExpression, context } of [
  { expression: '3*9', nextExpression: '((3 * 9))', context: 'repeat' },
  { expression: 'unknown(1)', nextExpression: '5*9', context: 'recovery' },
] as const) {
  test(`transport failure and quiet retry of ${expression} preserve active speech without extending or replaying it`, async ({ page }) => {
    await deterministicSpeech(page);
    await seedAccepted(page, ['1+', '10']);
    await openCalculator(page);
    // All deadline advances below begin after this clock boundary is frozen.
    await page.clock.pauseAt(await page.evaluate(() => Date.now() + 1_000));
    await calculate(page, '8*9');
    const text = await expectContextSpeech(page, 'ordinary');
    const speech = page.locator('.calculator-speech');
    const original = await speech.elementHandle();
    let accepted: CalculationResponse | undefined;
    await page.route('**/api/calculations', async (route) => {
      const response = await route.fetch();
      expect(response.status()).toBe(200);
      accepted = await response.json();
      await route.abort('failed');
    });
    try {
      await page.clock.runFor(500);
      await page.locator('#expression').fill(expression);
      await page.locator('#expression').press('Enter');
      const retry = page.locator('.calculation-result .secondary-button');
      await expect(retry).toBeEnabled();
      await expect(speech).toBeVisible();
      await expect(speech).toHaveText(text);
      await expect(page.locator('.calculator-personality__pixels')).toHaveAttribute('data-mood', 'idle');
      await page.unroute('**/api/calculations');
      await page.clock.runFor(500);
      const reply = page.waitForResponse((response) => new URL(response.url()).pathname === '/api/calculations');
      await retry.click();
      const response = await reply;
      expect(response.status()).toBe(200);
      const retried: CalculationResponse = await response.json();
      expect(retried.calculation).toEqual(accepted?.calculation);
      await expect(page.locator('.result-source code')).toHaveText(expression);
      if (retried.calculation.outcome.kind === 'success') await expect(page.locator('.result-value')).toHaveText('= 27');
      else await expect(page.locator('.result-error')).toBeVisible();
      await expect(speech).toHaveText(text);
      expect(await original!.evaluate((bubble) => bubble === document.querySelector('.calculator-speech'))).toBe(true);
      await expect(page.locator('.calculator-personality__pixels')).toHaveAttribute('data-mood', 'idle');
      await page.clock.runFor(3_499);
      await expect(speech).toBeVisible();
      await expect(speech).toHaveAttribute('data-phase', 'entering');
      await page.clock.runFor(1);
      await expect(speech).toHaveAttribute('data-phase', 'leaving');
      await page.clock.runFor(240);
      await expect(speech).toHaveCount(0);
      await watchUnwantedSpeech(page);
      await page.clock.runFor(1_000);
      await expectNoSpeech(page);
      await speechDraws(page, [0.5, 0.5, 0.5, 0, 0]);
      await calculate(page, nextExpression);
      await expectContextSpeech(page, context);
    } finally {
      await page.unroute('**/api/calculations');
      await original?.dispose();
    }
  });
}

test('speech probability excludes the 70 percent boundary and never queues skipped outcomes', async ({ page }) => {
  await deterministicSpeech(page);
  await openCalculator(page);
  await watchUnwantedSpeech(page);
  await speechDraws(page, [0, 0, 0.7]);
  const skipped = await calculate(page, 'unknown(1)');
  expect(skipped.calculation.outcome).toMatchObject({ kind: 'error', error: { code: 'UNKNOWN_IDENTIFIER' } });
  await page.clock.runFor(6_000);
  await expectNoSpeech(page);
  await speechDraws(page, [0, 0, 0.699999]);
  await calculate(page, 'sin(1,2)');
  await expectContextSpeech(page, 'math_error');
});

test('a real expression-limit rejection is not a mathematical reaction or recovery', async ({ page }) => {
  await deterministicSpeech(page);
  const capabilitiesReply = page.waitForResponse((response) => new URL(response.url()).pathname === '/api/capabilities');
  await openCalculator(page);
  const capabilities: Capabilities = await (await capabilitiesReply).json();
  await calculate(page, '8*9');
  const text = await expectContextSpeech(page, 'ordinary');
  const depth = capabilities.limits.nesting + 1;
  const expression = `${'('.repeat(depth)}8${')'.repeat(depth)}`;
  await page.locator('#expression').fill(expression);
  const rejected = page.waitForResponse((response) => new URL(response.url()).pathname === '/api/calculations');
  await page.locator('#expression').press('Enter');
  const response = await rejected;
  expect(response.status()).toBe(413);
  await expect(page.locator('.result-error')).toBeVisible();
  await expect(page.locator('.error-stage')).toHaveCount(0);
  await expect(page.locator('.result-value')).toHaveCount(0);
  await expect(page.locator('.calculator-speech')).toHaveText(text);
  await page.clock.runFor(6_000);
  await expect(page.locator('.calculator-speech')).toHaveCount(0);
  await watchUnwantedSpeech(page);
  await page.clock.runFor(6_000);
  await expectNoSpeech(page);
  await speechDraws(page, [0.5, 0.5, 0.5, 0, 0]);
  await calculate(page, '3*9');
  await expectContextSpeech(page, 'ordinary');
  await expect(page.locator('.result-value')).toHaveText('= 27');
});

for (const { viewport, requireSpeech } of [
  { viewport: { width: 1440, height: 1000 }, requireSpeech: true },
  { viewport: { width: 390, height: 1000 }, requireSpeech: true },
  { viewport: { width: 1280, height: 720 }, requireSpeech: false },
]) {
  test(`speech stays clear of the editor, result and controls at ${viewport.width}×${viewport.height}, including tool disclosure and resize`, async ({ page }) => {
    await deterministicSpeech(page, viewport);
    await openCalculator(page);
    await page.evaluate(() => document.fonts.ready.then(() => undefined));
    await watchSpeechSafety(page);
    await calculate(page, '8*9');
    if (requireSpeech) await expect(page.locator('.calculator-speech')).toBeVisible();
    await expect(page.locator('#expression')).toBeFocused();
    await page.clock.runFor(700);
    await page.locator('#tool-keypad').click();
    await expect(page.locator('#tool-bay')).toBeVisible();
    // Safe relocation or dismissal is valid; obstruction of a newly opened tool is not.
    await page.clock.runFor(300);
    await page.locator('#tool-bay-heading').press('Escape');
    await page.setViewportSize({ width: viewport.width, height: 640 });
    await page.clock.runFor(5_500);
    await expect(page.locator('.calculator-speech')).toHaveCount(0);
    await page.setViewportSize(viewport);
    await calculate(page, '3*9');
    // A different unused phrase may need more room. Standard/cramped surfaces
    // may skip it; the roomy desktop and phone must actually present it safely.
    if (requireSpeech) await expect(page.locator('.calculator-speech')).toBeVisible();
    await page.clock.runFor(5_500);
    await expect(page.locator('.calculator-speech')).toHaveCount(0);
    await expectSafeSpeech(page);
    await expect(page.locator('.result-value')).toHaveText('= 27');
  });
}

test('a cramped phone presents only safe speech or skips it without covering essential controls', async ({ page }) => {
  await deterministicSpeech(page, { width: 390, height: 640 });
  await openCalculator(page);
  await page.evaluate(() => document.fonts.ready.then(() => undefined));
  await watchSpeechSafety(page);
  await calculate(page, '8*9');
  await page.clock.runFor(6_000);
  await expect(page.locator('.calculator-speech')).toHaveCount(0);
  await expectSafeSpeech(page);
  await expect(page.locator('.result-value')).toHaveText('= 72');
  await expect(page.locator('#expression')).toBeFocused();
});

test('active and queued real awards preempt speech without replaying blocked comments after the queue drains', async ({ page }) => {
  await deterministicSpeech(page);
  await openCalculator(page);
  await watchSpeechSafety(page);
  await calculate(page, '8*9');
  await expect(page.locator('.calculator-speech')).toBeVisible();
  const awarded = await calculate(page, 'sqrt(16)/abs(-4)+ln(1)');
  expect(awarded.achievements?.map((award) => award.id)).toEqual(['scientific_method', 'paper_tiger']);
  const ceremony = page.getByRole('complementary', { name: 'Новое достижение', exact: true });
  await expect(ceremony).toHaveAttribute('data-achievement-id', 'scientific_method');
  await expect(page.locator('.calculator-speech')).toHaveCount(0);
  await watchUnwantedSpeech(page);
  await calculate(page, '3+4');
  await page.clock.runFor(1_000);
  await expect(page.locator('.calculator-speech')).toHaveCount(0);
  await ceremony.getByRole('button', { name: 'Закрыть уведомление о достижении', exact: true }).click();
  await expect(ceremony).toHaveAttribute('data-achievement-id', 'paper_tiger');
  await expect(page.locator('.calculator-speech')).toHaveCount(0);
  await ceremony.getByRole('button', { name: 'Закрыть уведомление о достижении', exact: true }).click();
  await expect(ceremony).toHaveCount(0);
  await page.clock.runFor(6_000);
  await expectNoSpeech(page);
  await expectSafeSpeech(page);
  await calculate(page, '9*9');
  await expect(page.locator('.calculator-speech')).toBeVisible();
  await expect(page.locator('.result-value')).toHaveText('= 81');
});

test('an older accepted reply cannot revive speech after the newest visible reaction expires', async ({ page }) => {
  await deterministicSpeech(page);
  await openCalculator(page);
  const delayed = await holdReply(page, '**/api/calculations', (route) => route.request().postDataJSON().expression === '8*9');
  try {
    await page.locator('#expression').fill('8*9');
    await page.locator('#expression').press('Enter');
    await delayed.received;
    await calculate(page, '3*9');
    await expect(page.locator('.calculator-speech')).toBeVisible();
    await page.clock.runFor(6_000);
    await expect(page.locator('.calculator-speech')).toHaveCount(0);
    await watchUnwantedSpeech(page);
    await delayed.deliver();
    await page.clock.runFor(6_000);
    await expectNoSpeech(page);
    await expect(page.locator('.result-source code')).toHaveText('3*9');
    await expect(page.locator('.result-value')).toHaveText('= 27');
    await calculate(page, '5*9');
    await expect(page.locator('.calculator-speech')).toBeVisible();
  } finally {
    delayed.release();
  }
});

test('a transport retry commits real math without becoming a new deliberate comment', async ({ page }) => {
  await deterministicSpeech(page);
  await openCalculator(page);
  await calculate(page, '8*9');
  await expect(page.locator('.calculator-speech')).toBeVisible();
  await page.clock.runFor(6_000);
  // React commits the exit phase after the clock jump; its cleanup timer then
  // starts. Exclude only the subsequent failure/retry, not this legitimate exit.
  await expect(page.locator('.calculator-speech')).toHaveCount(0);
  await watchUnwantedSpeech(page);
  let requestId = '';
  await page.route('**/api/calculations', async (route) => {
    requestId = route.request().postDataJSON().requestId;
    await route.abort('failed');
  });
  await page.locator('#expression').fill('3*9');
  await page.locator('#expression').press('Enter');
  const retry = page.locator('.calculation-result .secondary-button');
  await expect(retry).toBeEnabled();
  await page.unroute('**/api/calculations');
  const reply = page.waitForResponse((response) => new URL(response.url()).pathname === '/api/calculations');
  await retry.click();
  const response = await reply;
  expect(response.status()).toBe(200);
  const accepted: CalculationResponse = await response.json();
  expect(accepted.calculation.requestId).toBe(requestId);
  expect(accepted.calculation.outcome).toEqual({ kind: 'success', value: '27' });
  await expect(page.locator('.result-value')).toHaveText('= 27');
  await page.clock.runFor(6_000);
  await expectNoSpeech(page);
  await calculate(page, '5*9');
  await expect(page.locator('.calculator-speech')).toBeVisible();
});

test('identity adoption clears speech and ignores a previous owner reply, but the new owner can still react', async ({ page, browser, context, baseURL }) => {
  await deterministicSpeech(page);
  await openCalculator(page);
  await calculate(page, '11*11');
  await expect(page.locator('.calculator-speech')).toBeVisible();
  const replacement = await browser.newContext({ baseURL });
  const delayed = await holdReply(page, '**/api/calculations', (route) => route.request().postDataJSON().expression === '13*13');
  try {
    const replacementPage = await replacement.newPage();
    await openCalculator(replacementPage);
    await calculate(replacementPage, '22*22');
    await page.locator('#expression').fill('13*13');
    await page.locator('#expression').press('Enter');
    await delayed.received;
    await context.clearCookies();
    await context.addCookies(await replacement.cookies());
    const history = page.waitForResponse((response) => new URL(response.url()).pathname === '/api/history');
    await page.locator('#header-achievements').click();
    expect((await history).status()).toBe(200);
    await page.locator('#achievement-collection').getByRole('button', { name: 'Закрыть достижения' }).click();
    await expect(page.locator('.calculator-speech')).toHaveCount(0);
    await watchUnwantedSpeech(page);
    await delayed.deliver();
    await page.clock.runFor(6_000);
    await expectNoSpeech(page);
    await expect(page.locator('.calculation-result')).toHaveCount(0);
    await calculate(page, '14*14');
    await expect(page.locator('.calculator-speech')).toBeVisible();
    await expect(page.locator('.result-value')).toHaveText('= 196');
  } finally {
    delayed.release();
    await replacement.close();
  }
});

test('turning jokes off removes current speech and does not replay disabled outcomes when re-enabled', async ({ page }) => {
  await deterministicSpeech(page);
  await openCalculator(page);
  await calculate(page, '8*9');
  await expect(page.locator('.calculator-speech')).toBeVisible();
  await page.locator('#tool-settings').click();
  const jokes = page.getByRole('checkbox', { name: /^Шутки/ });
  await jokes.uncheck();
  await expect(page.locator('.calculator-speech')).toHaveCount(0);
  await expect(page.locator('.calculator-personality')).toHaveCount(0);
  await watchUnwantedSpeech(page);
  await page.locator('#tool-bay-heading').press('Escape');
  await calculate(page, '3*9');
  await page.clock.runFor(6_000);
  await expect(page.locator('.calculator-speech')).toHaveCount(0);
  await page.locator('#tool-settings').click();
  await jokes.check();
  await page.locator('#tool-bay-heading').press('Escape');
  await page.clock.runFor(6_000);
  await expectNoSpeech(page);
  await calculate(page, '5*9');
  await expect(page.locator('.calculator-speech')).toBeVisible();
  await expect(page.locator('#expression')).toBeFocused();
});

for (const mode of ['reduced-motion', 'effects-off'] as const) {
  test(`${mode} preserves readable, non-moving speech and dismisses it without affecting the result`, async ({ page }) => {
    await deterministicSpeech(page);
    if (mode === 'reduced-motion') await page.emulateMedia({ reducedMotion: 'reduce' });
    await openCalculator(page);
    await page.evaluate(() => document.fonts.ready.then(() => undefined));
    if (mode === 'effects-off') {
      await page.locator('#tool-settings').click();
      await page.getByRole('checkbox', { name: /«Спецэффекты»/ }).uncheck();
      await page.locator('#tool-bay-heading').press('Escape');
    }
    await calculate(page, '8*9');
    const speech = page.locator('.calculator-speech');
    await expect(speech).toBeVisible();
    await page.clock.runFor(200);
    const settled = await speech.boundingBox();
    await page.clock.runFor(300);
    expect(await speech.boundingBox()).toEqual(settled);
    await expect(page.locator('#expression')).toBeFocused();
    await page.clock.runFor(5_000);
    await expect(speech).toHaveCount(0);
    await expect(page.locator('.result-value')).toHaveText('= 72');
  });
}
