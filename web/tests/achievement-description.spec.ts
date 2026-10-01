import type { Locator } from '@playwright/test';
import type { SessionResponse } from '../src/contracts';
import { expect, openCalculator, test } from './calculator-fixture';

declare global {
  interface Window {
    calculatorReplayedDecode: boolean;
  }
}

async function unchangedCipher(cipher: Locator) {
  const before = await cipher.textContent();
  // Observe across multiple authored cipher ticks, rather than racing one frame.
  await cipher.page().waitForTimeout(650);
  return await cipher.textContent() === before;
}

function watchReplayedDecode() {
  window.calculatorReplayedDecode = false;
  new MutationObserver(() => {
    if (document.querySelector('#achievement-collection [data-achievement-id="answer_found"] .achievement-description--decoding')) {
      window.calculatorReplayedDecode = true;
    }
  }).observe(document, { attributes: true, childList: true, subtree: true });
}

test('locked cipher changes only while visible and pauses offscreen, closed and under reduced motion', async ({ page }) => {
  await page.setViewportSize({ width: 390, height: 640 });
  await openCalculator(page);
  await page.locator('#header-achievements').click();
  const dialog = page.locator('#achievement-collection');
  const card = dialog.locator('[data-achievement-id="answer_found"]');
  const cipher = card.locator('.achievement-description__cipher');
  await card.scrollIntoViewIfNeeded();
  await expect(cipher).toBeVisible();
  const initial = await cipher.textContent();
  await expect.poll(() => cipher.textContent()).not.toBe(initial);
  await dialog.locator('.achievement-collection__body').evaluate((body) => { body.scrollTop = 0; });
  await expect(cipher).not.toBeInViewport();
  expect(await unchangedCipher(cipher)).toBe(true);

  await card.scrollIntoViewIfNeeded();
  const restored = await cipher.textContent();
  await expect.poll(() => cipher.textContent()).not.toBe(restored);
  await page.keyboard.press('Escape');
  await expect(dialog).not.toBeVisible();
  expect(await unchangedCipher(cipher)).toBe(true);

  await page.emulateMedia({ reducedMotion: 'reduce' });
  await page.locator('#header-achievements').click();
  await card.scrollIntoViewIfNeeded();
  await expect(cipher).toBeVisible();
  expect(await unchangedCipher(cipher)).toBe(true);
});

test('a visible locked award decodes only its authoritative grant, never collection browsing or bootstrap', async ({ page }) => {
  await openCalculator(page);
  const sessionReply = await page.request.get('/api/session');
  expect(sessionReply.status()).toBe(200);
  const session: SessionResponse = await sessionReply.json();
  const definition = session.discoveryCatalog?.find((item) => item.id === 'answer_found');
  if (!definition?.secret) throw new Error('Expected the service catalog to conceal the answer-found condition');
  // Unlike holdReply, this gate stops dispatch BEFORE the real service commits.
  // Otherwise collection refresh correctly discovers the earned award quietly.
  let release!: () => void;
  const gate = new Promise<void>((resolve) => { release = resolve; });
  let arrived!: () => void;
  const received = new Promise<void>((resolve) => { arrived = resolve; });
  await page.route('**/api/calculations', async (route) => {
    arrived();
    await gate;
    await route.continue();
  });
  try {
    await page.locator('#expression').fill('6*7');
    await page.locator('#expression').press('Enter');
    await received;
    const refresh = page.waitForResponse((response) => new URL(response.url()).pathname === '/api/session');
    await page.locator('#header-achievements').click();
    expect((await refresh).status()).toBe(200);
    const dialog = page.locator('#achievement-collection');
    const card = dialog.locator('[data-achievement-id="answer_found"]');
    await card.scrollIntoViewIfNeeded();
    const cipher = card.locator('.achievement-description__cipher');
    await expect(cipher).toBeVisible();
    const concealedFrame = await cipher.textContent();
    // Actual ticking proves the observer has registered this visible locked card.
    await expect.poll(() => cipher.textContent()).not.toBe(concealedFrame);
    expect(await dialog.textContent()).not.toContain(definition.ru.description);
    expect(await dialog.ariaSnapshot()).not.toContain(definition.ru.description);
    const calculation = page.waitForResponse((response) => new URL(response.url()).pathname === '/api/calculations');
    release();
    expect((await calculation).status()).toBe(200);
    await expect(card.locator('.achievement-description--decoding')).toBeVisible();
    await card.scrollIntoViewIfNeeded();
    await expect(card.locator('.achievement-description--decoding')).toHaveCount(0);
    await expect(card.locator('.achievement-description__cipher')).toHaveCount(0);
    await expect(card.locator('time')).toBeVisible();
    const earnedAt = await card.locator('time').getAttribute('datetime');
    await page.addInitScript(watchReplayedDecode);
    await page.evaluate(watchReplayedDecode);
    await page.keyboard.press('Escape');
    const ceremony = page.getByRole('complementary', { name: 'Новое достижение', exact: true });
    await ceremony.getByRole('button', { name: 'Закрыть уведомление о достижении', exact: true }).click();
    await page.locator('#header-achievements').click();
    await card.scrollIntoViewIfNeeded();
    await expect(card.locator('.achievement-description--decoding')).toHaveCount(0);
    await expect(card.locator('.achievement-description__cipher')).toHaveCount(0);
    await expect(card.locator('time')).toHaveAttribute('datetime', earnedAt!);
    expect(await page.evaluate(() => window.calculatorReplayedDecode)).toBe(false);
    await page.keyboard.press('Escape');
    await openCalculator(page);
    await expect(ceremony).toHaveCount(0);
    await page.locator('#header-achievements').click();
    await card.scrollIntoViewIfNeeded();
    await expect(card.locator('.achievement-description--decoding')).toHaveCount(0);
    await expect(card.locator('.achievement-description__cipher')).toHaveCount(0);
    await expect(card.locator('time')).toHaveAttribute('datetime', earnedAt!);
    expect(await page.evaluate(() => window.calculatorReplayedDecode)).toBe(false);
  } finally {
    release();
    await page.unroute('**/api/calculations');
  }
});
