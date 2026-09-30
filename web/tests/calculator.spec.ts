import type { CalculationResponse } from '../src/contracts';
import { calculate, expect, holdReply, openCalculator, test } from './calculator-fixture';

const scientific = 'sqrt(81)+2^3';

test('scientific calculation survives history restore, reuse, reload, and a private browser context', async ({ page, browser, baseURL }) => {
  await openCalculator(page);
  const result = await calculate(page, scientific);
  expect(result.calculation.outcome).toEqual({ kind: 'success', value: '17' });
  await expect(page.locator('.result-value')).toHaveText('= 17');

  await page.locator('#tool-history').click();
  const record = page.locator('.history-item-button').filter({ has: page.locator('.history-expression', { hasText: scientific }) }).first();
  await expect(record.locator('.history-outcome')).toHaveText('= 17');
  await page.locator('#expression').fill('999');
  await record.click();
  await expect(page.locator('#expression')).toHaveValue(scientific);
  await expect(page.locator('#expression')).toBeFocused();
  await expect(page.locator('#tool-bay')).toHaveCount(0);
  expect((await calculate(page, scientific)).calculation.outcome).toEqual({ kind: 'success', value: '17' });

  const historyReload = page.waitForResponse((response) => new URL(response.url()).pathname === '/api/history');
  await page.reload();
  expect((await historyReload).status()).toBe(200);
  await page.locator('#tool-history').click();
  await expect(page.locator('.history-item-button').filter({ hasText: scientific }).first().locator('.history-outcome')).toHaveText('= 17');

  const privateContext = await browser.newContext({ baseURL });
  try {
    const privatePage = await privateContext.newPage();
    await openCalculator(privatePage);
    await privatePage.locator('#tool-history').click();
    await expect(privatePage.locator('.history-item-button')).toHaveCount(0);
    expect((await calculate(privatePage, '7*8')).calculation.outcome).toEqual({ kind: 'success', value: '56' });
    await openCalculator(page);
    await page.locator('#tool-history').click();
    await expect(page.locator('.history-item-button').filter({ hasText: scientific }).first().locator('.history-outcome')).toHaveText('= 17');
    await expect(page.locator('.history-item-button').filter({ hasText: '7*8' })).toHaveCount(0);
  } finally {
    await privateContext.close();
  }
});

test('syntax errors remain saved and correction returns focus to the offending expression', async ({ page }) => {
  await openCalculator(page);
  const malformed = 'sqrt(81';
  const result = await calculate(page, malformed);
  expect(result.calculation.outcome.kind).toBe('error');
  if (result.calculation.outcome.kind !== 'error') throw new Error('Expected a mathematical error');
  expect(result.calculation.outcome.error.code).toBe('SYNTAX_ERROR');
  expect(result.calculation.outcome.error.stage).toBe('parse');
  await expect(page.locator('.result-highlight mark')).toBeVisible();
  await page.getByRole('button', { name: 'Исправить', exact: true }).click();
  await expect(page.locator('#expression')).toBeFocused();
  await expect(page.locator('#expression')).toHaveValue(malformed);
  expect(await page.locator('#expression').evaluate((element) => {
    if (!(element instanceof HTMLTextAreaElement)) throw new Error('Expected the expression editor');
    return { start: element.selectionStart, end: element.selectionEnd };
  })).toEqual(result.calculation.outcome.error.span);
  expect((await calculate(page, 'sqrt(81)')).calculation.outcome).toEqual({ kind: 'success', value: '9' });
  await page.locator('#tool-history').click();
  await page.locator('.history-item-button').filter({ has: page.locator('.history-expression', { hasText: /^sqrt\(81$/ }) }).click();
  await expect(page.locator('#expression')).toHaveValue(malformed);
  await expect(page.locator('.result-value')).toHaveText('= 9');
  const repeated = await calculate(page, malformed);
  if (repeated.calculation.outcome.kind !== 'error') throw new Error('Expected the saved expression to remain invalid');
  expect(repeated.calculation.outcome.error.code).toBe('SYNTAX_ERROR');
});

test('an unknown leading name is highlighted before a later square bracket', async ({ page }) => {
  await openCalculator(page);
  const malformed = 'asdasdfasdasfasfasfASFASFASFAFASFASFASFAFASFASFASFASFASFASFASFASFASFdf]asd]gla]hdgasd[gksdgdsgsd DeG';
  const result = await calculate(page, malformed);
  if (result.calculation.outcome.kind !== 'error') throw new Error('Expected a mathematical error');
  expect(result.calculation.outcome.error.code).toBe('UNKNOWN_IDENTIFIER');
  expect(result.calculation.outcome.error.span).toEqual({ start: 0, end: 1 });
  await expect(page.locator('.result-highlight mark')).toHaveText('a');
});

test('copy uses the canonical binary64 value rather than the rounded display', async ({ page, context }) => {
  await context.grantPermissions(['clipboard-read', 'clipboard-write']);
  await openCalculator(page);
  const result = await calculate(page, '1/3');
  expect(result.calculation.outcome).toEqual({ kind: 'success', value: '0.3333333333333333' });
  await expect(page.locator('.result-value')).toHaveText('≈ 0.333333333333');
  await page.getByRole('button', { name: 'Скопировать точное значение', exact: true }).click();
  await expect.poll(() => page.evaluate(() => navigator.clipboard.readText())).toBe('0.3333333333333333');
});

test('angle context belongs to the submitted record and an older reply cannot replace the latest result', async ({ page }) => {
  await openCalculator(page);
  await page.locator('#angle-unit').selectOption('deg');
  const delayed = await holdReply(page, '**/api/calculations', (route) => route.request().postDataJSON().expression === 'sin(90)');
  try {
    await page.locator('#expression').fill('sin(90)');
    await page.locator('#expression').press('Enter');
    await delayed.received;
    await page.locator('#angle-unit').selectOption('rad');
    const current = await calculate(page, 'cos(pi)');
    expect(current.calculation.context.angleUnit).toBe('rad');
    expect(current.calculation.outcome).toEqual({ kind: 'success', value: '-1' });
    await delayed.deliver();
    await expect(page.locator('.result-source code')).toHaveText('cos(pi)');
    await expect(page.locator('.result-value')).toHaveText('= -1');
    await expect(page.locator('.result-angle')).toHaveText('RAD');
    await page.locator('#tool-history').click();
    const previous = page.locator('.history-item-button').filter({ hasText: 'sin(90)' });
    await expect(previous.locator('.history-outcome')).toHaveText('= 1');
    await previous.click();
    await expect(page.locator('#angle-unit')).toHaveValue('deg');
    await expect(page.locator('#expression')).toHaveValue('sin(90)');
    const restored = await calculate(page, 'sin(90)');
    expect(restored.calculation.context.angleUnit).toBe('deg');
    expect(restored.calculation.outcome).toEqual({ kind: 'success', value: '1' });
  } finally {
    delayed.release();
  }
});

test('a held or failed collection refresh leaves calculation and history usable', async ({ page }) => {
  await openCalculator(page);
  await page.locator('#tool-history').click();
  const collection = page.locator('#achievement-collection');
  const delayed = await holdReply(page, '**/api/session');
  try {
    await collection.locator('summary').click();
    await delayed.received;
    expect((await calculate(page, scientific)).calculation.outcome).toEqual({ kind: 'success', value: '17' });
    await expect(page.locator('.history-item-button').filter({ hasText: scientific }).locator('.history-outcome')).toHaveText('= 17');
    await delayed.deliver();
  } finally {
    delayed.release();
  }
  await page.unroute('**/api/session');
  await collection.locator('summary').click();
  await expect(collection).not.toHaveAttribute('open', '');
  await page.route('**/api/session', (route) => route.abort('failed'));
  await collection.locator('summary').click();
  await expect(collection.getByRole('button', { name: 'Повторить загрузку', exact: true })).toBeEnabled();
  expect((await calculate(page, '8*9')).calculation.outcome).toEqual({ kind: 'success', value: '72' });
  await expect(page.locator('.history-item-button').filter({ hasText: '8*9' }).locator('.history-outcome')).toHaveText('= 72');
  await page.unroute('**/api/session');
  const recovery = page.waitForResponse((response) => new URL(response.url()).pathname === '/api/session');
  await collection.getByRole('button', { name: 'Повторить загрузку', exact: true }).click();
  expect((await recovery).status()).toBe(200);
  await expect(collection.getByRole('button', { name: 'Повторить загрузку', exact: true })).toHaveCount(0);
});

test('adopting a replacement identity automatically loads its history and rejects an old owner reply', async ({ page, browser, context, baseURL }) => {
  await openCalculator(page);
  await calculate(page, '11*11');
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
    await page.locator('#tool-history').click();
    const adoptedHistory = page.waitForResponse((response) => new URL(response.url()).pathname === '/api/history');
    await page.locator('#achievement-collection summary').click();
    expect((await adoptedHistory).status()).toBe(200);
    await expect(page.locator('.history-item-button').filter({ hasText: '22*22' }).locator('.history-outcome')).toHaveText('= 484');
    await expect(page.locator('.history-item-button').filter({ hasText: '11*11' })).toHaveCount(0);
    await delayed.deliver();
    await expect(page.locator('.calculation-result')).toHaveCount(0);
    await expect(page.locator('.history-item-button').filter({ hasText: '13*13' })).toHaveCount(0);
    expect((await calculate(page, '6*9')).calculation.outcome).toEqual({ kind: 'success', value: '54' });
    await expect(page.locator('.history-item-button').filter({ hasText: '22*22' }).locator('.history-outcome')).toHaveText('= 484');
  } finally {
    delayed.release();
    await replacement.close();
  }
});

test('IME keys do not calculate or close settings; ordinary Escape restores the trigger', async ({ page }) => {
  await openCalculator(page);
  await page.locator('#expression').fill('8+9');
  const requests: string[] = [];
  page.on('request', (request) => {
    if (new URL(request.url()).pathname === '/api/calculations') requests.push(request.url());
  });
  await page.locator('#expression').dispatchEvent('keydown', { key: 'Enter', code: 'Enter', isComposing: true, bubbles: true });
  await page.locator('#expression').dispatchEvent('keydown', { key: 'Enter', code: 'Enter', keyCode: 229, bubbles: true });
  await page.locator('#tool-settings').click();
  await expect(page.locator('#tool-bay-heading')).toBeFocused();
  expect(requests).toEqual([]);
  await page.locator('#tool-bay-heading').dispatchEvent('keydown', { key: 'Escape', isComposing: true, bubbles: true });
  await expect(page.locator('#tool-settings')).toHaveAttribute('aria-expanded', 'true');
  await page.locator('#tool-bay-heading').press('Escape');
  await expect(page.locator('#tool-bay')).toHaveCount(0);
  await expect(page.locator('#tool-settings')).toBeFocused();
  expect((await calculate(page, '8+9')).calculation.outcome).toEqual({ kind: 'success', value: '17' });
});

test('tool Escape restores each trigger and scientific insertion preserves the open bay and cursor', async ({ page }) => {
  await openCalculator(page);
  for (const tool of ['functions', 'keypad', 'history']) {
    const trigger = page.locator(`#tool-${tool}`);
    await trigger.click();
    await expect(page.locator('#tool-bay-heading')).toBeFocused();
    await page.locator('#tool-bay-heading').press('Escape');
    await expect(page.locator('#tool-bay')).toHaveCount(0);
    await expect(trigger).toBeFocused();
  }
  const editor = page.locator('#expression');
  await editor.fill('81');
  await editor.evaluate((element) => {
    if (!(element instanceof HTMLTextAreaElement)) throw new Error('Expected the expression editor');
    element.setSelectionRange(0, 2);
  });
  await page.locator('#tool-functions').click();
  await page.getByRole('button', { name: 'Вставить Корень: sqrt(81)', exact: true }).click();
  await expect(editor).toHaveValue('sqrt(81)');
  await expect(editor).toBeFocused();
  await expect(page.locator('#tool-functions')).toHaveAttribute('aria-expanded', 'true');
  expect(await editor.evaluate((element) => {
    if (!(element instanceof HTMLTextAreaElement)) throw new Error('Expected the expression editor');
    return { start: element.selectionStart, end: element.selectionEnd };
  })).toEqual({ start: 7, end: 7 });
  expect((await calculate(page, 'sqrt(81)')).calculation.outcome).toEqual({ kind: 'success', value: '9' });
});

test('Escape dismisses a real comic scene before the tool panel without blocking the mathematical error', async ({ page }) => {
  await openCalculator(page);
  await page.locator('#tool-settings').click();
  await calculate(page, '1/0');
  await calculate(page, '1/0');
  const third = await calculate(page, '1/0');
  expect(third.calculation.outcome.kind).toBe('error');
  expect(third.funEvents?.some((event) => event.ruleId === 'comic_incident' && event.kind === 'scene')).toBe(true);
  const scene = page.getByRole('complementary', { name: 'Шуточная сцена калькулятора', exact: true });
  await expect(scene).toBeVisible();
  await expect(page.locator('.result-error')).toBeVisible();
  await page.getByRole('button', { name: 'Закрыть шуточную сцену', exact: true }).focus();
  await page.keyboard.press('Escape');
  await expect(scene).toHaveCount(0);
  await expect(page.locator('#tool-settings')).toHaveAttribute('aria-expanded', 'true');
  await expect(page.locator('#expression')).toBeFocused();
  await page.keyboard.press('Escape');
  await expect(page.locator('#tool-settings')).toHaveAttribute('aria-expanded', 'false');
  expect((await calculate(page, '9+8')).calculation.outcome).toEqual({ kind: 'success', value: '17' });
});

test('both palettes preserve editor, angle, outcome and history on a phone-sized surface', async ({ page }) => {
  await page.setViewportSize({ width: 390, height: 844 });
  await openCalculator(page);
  await page.locator('#angle-unit').selectOption('rad');
  await calculate(page, scientific);
  await page.locator('#expression').fill('sin(pi/2)');
  for (const [label, theme] of [['Янтарная', 'amber'], ['Фиолетовая', 'violet']] as const) {
    await page.locator('#tool-settings').click();
    await page.getByRole('radio', { name: label, exact: true }).check();
    await expect(page.locator('html')).toHaveAttribute('data-theme', theme);
    await expect(page.locator('#expression')).toHaveValue('sin(pi/2)');
    await expect(page.locator('#angle-unit')).toHaveValue('rad');
    await expect(page.locator('.result-value')).toHaveText('= 17');
    await page.locator('#tool-bay-heading').press('Escape');
    await expect(page.locator('#tool-settings')).toBeFocused();
    await page.locator('#tool-history').click();
    await expect(page.locator('.history-item-button').filter({ hasText: scientific }).locator('.history-outcome')).toHaveText('= 17');
    await page.locator('#tool-bay-heading').press('Escape');
  }
  expect(await page.evaluate(() => document.documentElement.scrollWidth <= window.innerWidth)).toBe(true);
  const result: CalculationResponse = await calculate(page, 'sin(pi/2)');
  expect(result.calculation.context.angleUnit).toBe('rad');
  expect(result.calculation.outcome).toEqual({ kind: 'success', value: '1' });
  await expect(page.locator('.result-value')).toBeInViewport();
  await expect(page.locator('#tool-bay')).toHaveCount(0);
  await page.locator('#expression').fill('8*9');
  await page.locator('#tool-keypad').click();
  const submitted = page.waitForResponse((response) => new URL(response.url()).pathname === '/api/calculations');
  await page.locator('#tool-bay').getByRole('button', { name: 'Вычислить', exact: true }).click();
  const response = await submitted;
  expect(response.status()).toBe(200);
  const mobileResult: CalculationResponse = await response.json();
  expect(mobileResult.calculation.outcome).toEqual({ kind: 'success', value: '72' });
  await expect(page.locator('.result-value')).toHaveText('= 72');
  await expect(page.locator('#tool-bay')).toHaveCount(0);
  await expect(page.locator('.editor-console .calculator-submit')).toBeFocused();
});
