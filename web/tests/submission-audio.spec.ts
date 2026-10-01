import type { Locator, Page } from '@playwright/test';
import type { CalculationRequest, CalculationResponse } from '../src/contracts';
import { calculate, expect, holdReply, openCalculator, test } from './calculator-fixture';

type VoiceSchedule = {
  start: number | null;
  stops: number[];
  ended: boolean;
  waveform: OscillatorType;
  initialFrequency: number;
  frequencies: { value: number; at: number }[];
};
type SoundSchedule = {
  disconnectedAt: number | null;
  voices: VoiceSchedule[];
  signal?: { peak: number; rms10ms: number };
};

declare global {
  interface Window {
    calculatorAudioProbe: { contexts: AudioContext[]; sounds: SoundSchedule[] };
  }
}

// Observe native scheduling and connections; all nodes still drive the real device.
async function observeAudio(page: Page, measureWaveforms = false) {
  await page.addInitScript((measureWaveforms: boolean) => {
    const probe = { contexts: [] as AudioContext[], sounds: [] as SoundSchedule[] };
    window.calculatorAudioProbe = probe;
    const destinations = new WeakMap<AudioNode, AudioNode | AudioParam>();
    const outputs = new WeakMap<GainNode, SoundSchedule>();
    const createGain = AudioContext.prototype.createGain;
    AudioContext.prototype.createGain = function () {
      const gain = createGain.call(this);
      const connect = gain.connect.bind(gain);
      gain.connect = ((destination: AudioNode | AudioParam, output?: number, input?: number) => {
        const result = destination instanceof AudioNode
          ? connect(destination, output, input) : connect(destination, output);
        destinations.set(gain, destination);
        if (destination === this.destination) {
          if (!probe.contexts.includes(this)) probe.contexts.push(this);
          const sound: SoundSchedule = { disconnectedAt: null, voices: [] };
          outputs.set(gain, sound);
          probe.sounds.push(sound);
          if (measureWaveforms) {
            // Tap the real mixed output without replacing the device, nodes or
            // automation. The long sample history covers even a short edit cue
            // between timer callbacks; 10 ms RMS compares actual signal energy.
            const analyser = this.createAnalyser();
            analyser.fftSize = 32768;
            connect(analyser);
            const samples = new Float32Array(analyser.fftSize);
            const windowSize = Math.round(this.sampleRate * .01);
            const signal = { peak: 0, rms10ms: 0 };
            sound.signal = signal;
            const measure = () => {
              analyser.getFloatTimeDomainData(samples);
              let energy = 0;
              for (let index = 0; index < samples.length; index += 1) {
                const sample = samples[index]!;
                signal.peak = Math.max(signal.peak, Math.abs(sample));
                energy += sample * sample;
                if (index >= windowSize) energy -= samples[index - windowSize]! ** 2;
                if (index >= windowSize - 1) {
                  signal.rms10ms = Math.max(signal.rms10ms, Math.sqrt(Math.max(0, energy) / windowSize));
                }
              }
            };
            const timer = window.setInterval(measure, 20);
            const disconnect = gain.disconnect.bind(gain);
            gain.disconnect = (() => {
              measure();
              window.clearInterval(timer);
              disconnect();
              analyser.disconnect();
            }) as typeof gain.disconnect;
          }
        }
        return result;
      }) as typeof gain.connect;
      const disconnect = gain.disconnect.bind(gain);
      gain.disconnect = (() => {
        disconnect();
        const sound = outputs.get(gain);
        if (sound) sound.disconnectedAt = this.currentTime;
      }) as typeof gain.disconnect;
      return gain;
    };
    const createOscillator = AudioContext.prototype.createOscillator;
    AudioContext.prototype.createOscillator = function () {
      const voice = createOscillator.call(this);
      const schedule: VoiceSchedule = {
        start: null, stops: [], ended: false, waveform: voice.type,
        initialFrequency: voice.frequency.value, frequencies: [],
      };
      const connect = voice.connect.bind(voice);
      voice.connect = ((destination: AudioNode | AudioParam, output?: number, input?: number) => {
        const result = destination instanceof AudioNode
          ? connect(destination, output, input) : connect(destination, output);
        destinations.set(voice, destination);
        return result;
      }) as typeof voice.connect;
      const setFrequency = voice.frequency.setValueAtTime.bind(voice.frequency);
      voice.frequency.setValueAtTime = (value, at) => {
        const result = setFrequency(value, at);
        schedule.frequencies.push({ value, at });
        return result;
      };
      const rampFrequency = voice.frequency.exponentialRampToValueAtTime.bind(voice.frequency);
      voice.frequency.exponentialRampToValueAtTime = (value, at) => {
        const result = rampFrequency(value, at);
        schedule.frequencies.push({ value, at });
        return result;
      };
      const start = voice.start.bind(voice);
      const stop = voice.stop.bind(voice);
      voice.start = (at = 0) => {
        start(at);
        schedule.start = at || this.currentTime;
        schedule.waveform = voice.type;
        schedule.initialFrequency = voice.frequency.value;
        const envelope = destinations.get(voice);
        const output = envelope instanceof AudioNode ? destinations.get(envelope) : undefined;
        if (output instanceof GainNode) outputs.get(output)?.voices.push(schedule);
      };
      voice.stop = (at = 0) => {
        stop(at);
        schedule.stops.push(at || this.currentTime);
      };
      voice.addEventListener('ended', () => { schedule.ended = true; });
      return voice;
    };
  }, measureWaveforms);
}

async function sounds(page: Page) {
  return page.evaluate(() => window.calculatorAudioProbe.sounds);
}

function profile(sound: SoundSchedule) {
  return sound.voices.map((voice) => ({
    waveform: voice.waveform,
    pitches: voice.frequencies.length ? voice.frequencies.map((point) => point.value) : [voice.initialFrequency],
    duration: Math.round((voice.stops[0]! - voice.start!) * 1_000_000),
  }));
}

async function activationSound(page: Page, action: () => Promise<unknown>) {
  const before = (await sounds(page)).length;
  await action();
  // Count deliberate signals, not oscillators: one activation must never double-dispatch.
  await expect.poll(async () => (await sounds(page)).length).toBe(before + 1);
  return (await sounds(page))[before]!;
}

async function settleAudio(page: Page) {
  await expect.poll(async () => (await sounds(page)).every((sound) =>
    sound.disconnectedAt !== null && sound.voices.every((voice) => voice.ended))).toBe(true);
}

async function trustedClickPair(page: Page, first: Locator, second: Locator) {
  const firstBox = await first.boundingBox();
  const secondBox = await second.boundingBox();
  if (!firstBox || !secondBox) throw new Error('Expected both activation controls to be visible');
  const session = await page.context().newCDPSession(page);
  try {
    for (const box of [firstBox, secondBox]) {
      const point = { x: box.x + box.width / 2, y: box.y + box.height / 2 };
      await session.send('Input.dispatchMouseEvent', { type: 'mousePressed', button: 'left', clickCount: 1, ...point });
      await session.send('Input.dispatchMouseEvent', { type: 'mouseReleased', button: 'left', clickCount: 1, ...point });
    }
  } finally {
    await session.detach();
  }
}

test('navigation, editing, toggles, deliberate submissions and awards have distinct native sounds on one device', async ({ page }) => {
  await observeAudio(page, true);
  await openCalculator(page);
  const editor = page.locator('#expression');
  await editor.click(); // A trusted, silent gesture unlocks before the first cue.
  const open = await activationSound(page, () => page.locator('#tool-functions').click());
  const close = await activationSound(page, () => page.getByRole('button', { name: 'Закрыть панель инструментов', exact: true }).click());
  await editor.fill('2+3');
  const edit = await activationSound(page, () => page.getByRole('button', { name: 'Очистить', exact: true }).click());
  await expect(editor).toHaveValue('');
  await page.locator('#tool-settings').click();
  const toggle = await activationSound(page, () => page.getByRole('checkbox', { name: /Спецэффекты/ }).click());
  await page.getByRole('button', { name: 'Закрыть панель инструментов', exact: true }).click();
  const submit = await activationSound(page, () => calculate(page, '2+3'));
  const profiles = [open, close, edit, toggle, submit].map((sound) => JSON.stringify(profile(sound)));
  expect(new Set(profiles).size).toBe(profiles.length);
  for (const ordinary of [open, close, edit, toggle, submit]) {
    expect(ordinary.voices.every((voice) => voice.start !== null && voice.stops[0]! - voice.start! < .15)).toBe(true);
  }

  const keyboardOpen = await activationSound(page, () => page.locator('#tool-functions').press('Enter'));
  expect(profile(keyboardOpen)).toEqual(profile(open));
  const keyboardClose = await activationSound(page, () => page.getByRole('button', { name: 'Закрыть панель инструментов', exact: true }).press('Space'));
  await expect(page.locator('#tool-functions')).toHaveAttribute('aria-expanded', 'false');
  expect(profile(keyboardClose)).toEqual(profile(close));
  await editor.fill('3+4');
  const pointerSubmit = await activationSound(page, () => page.locator('.editor-console .calculator-submit').click());
  await expect(page.locator('.result-value')).toHaveText('= 7');
  expect(profile(pointerSubmit)).toEqual(profile(submit));
  await page.locator('#tool-keypad').click();
  await editor.fill('');
  const keypadEdit = await activationSound(page, () => page.locator('.calculator-keys').getByRole('button', { name: 'Вставить 6', exact: true }).click());
  await expect(editor).toHaveValue('6');
  expect(profile(keypadEdit)).toEqual(profile(edit));
  const keypadSubmit = await activationSound(page, () => page.locator('#tool-bay .calculator-submit').click());
  await expect(page.locator('.result-value')).toHaveText('= 6');
  expect(profile(keypadSubmit)).toEqual(profile(submit));

  await calculate(page, '6*7');
  await expect(page.getByRole('complementary', { name: 'Новое достижение', exact: true })).toBeVisible();
  const award = (await sounds(page)).at(-1)!;
  const awardSpan = Math.max(...award.voices.map((voice) => voice.stops[0]!)) - Math.min(...award.voices.map((voice) => voice.start!));
  expect(awardSpan).toBeGreaterThan(.4);
  expect(award.voices.some((voice, index) => index > 0 && voice.initialFrequency > award.voices[index - 1]!.initialFrequency)).toBe(true);
  await settleAudio(page);
  const measured = await sounds(page);
  const ordinarySignals = [open, close, edit, toggle, submit].map((ordinary) =>
    measured.find((sound) => sound.voices[0]!.start === ordinary.voices[0]!.start)!.signal!);
  // Native waveform thresholds catch a return to barely audible buttons, not
  // an incidental output-gain ordering (different waveforms/envelopes differ).
  // Every peak floor exceeds the previous loudest ordinary bound (.084);
  // motif-specific RMS floors also require sustained energy, not just a spike.
  const ordinaryFloors = [
    { motif: 'open', peak: .12, rms10ms: .04 },
    { motif: 'close', peak: .12, rms10ms: .04 },
    { motif: 'edit', peak: .10, rms10ms: .035 },
    { motif: 'toggle', peak: .14, rms10ms: .055 },
    { motif: 'submit', peak: .24, rms10ms: .075 },
  ];
  for (const [index, floor] of ordinaryFloors.entries()) {
    const signal = ordinarySignals[index]!;
    expect(signal.peak, `${floor.motif} native PCM peak`).toBeGreaterThan(floor.peak);
    expect(signal.rms10ms, `${floor.motif} native 10 ms RMS`).toBeGreaterThan(floor.rms10ms);
  }
  const awardSignal = measured.at(-1)!.signal!;
  expect(awardSignal.peak, 'award native PCM peak').toBeGreaterThan(.3);
  expect(awardSignal.rms10ms, 'award native 10 ms RMS').toBeGreaterThan(.2);
  expect(awardSignal.rms10ms).toBeGreaterThan(Math.max(...ordinarySignals.map((signal) => signal.rms10ms)) * 1.25);
  // Includes all five motifs, their keyboard/keypad variants and every mixed
  // fanfare voice, measured downstream of the real envelopes and output gain.
  for (const sound of measured) expect(sound.signal!.peak, 'native PCM clipping boundary').toBeLessThan(1);
  await test.info().attach('native-audio-levels.json', {
    body: JSON.stringify(measured.map((sound) => ({ profile: profile(sound), ...sound.signal })), null, 2),
    contentType: 'application/json',
  });
  expect(profiles).not.toContain(JSON.stringify(profile(award)));
  expect(await page.evaluate(() => window.calculatorAudioProbe.contexts.length)).toBe(1);
});

test('hover, focus, disabled controls, typing, blank or composing Enter and programmatic activation stay silent', async ({ page }) => {
  await observeAudio(page);
  await openCalculator(page);
  const editor = page.locator('#expression');
  await editor.click();
  const before = (await sounds(page)).length;
  await page.locator('#tool-functions').hover();
  await page.locator('#tool-functions').focus();
  await editor.focus();
  await editor.press('Enter');
  await page.locator('.editor-console .calculator-submit').click({ force: true });
  await page.getByRole('button', { name: 'Очистить', exact: true }).click({ force: true });
  await editor.pressSequentially('2+3');
  await editor.press('Shift+Enter');
  await editor.dispatchEvent('keydown', { key: 'Enter', isComposing: true, bubbles: true });
  await editor.dispatchEvent('keydown', { key: 'Enter', keyCode: 229, bubbles: true });
  await page.locator('#tool-functions').evaluate((button) => (button as HTMLButtonElement).click());
  await expect(page.locator('#tool-bay')).toBeVisible();
  expect((await sounds(page)).length).toBe(before);
  await editor.dispatchEvent('keydown', { key: 'Enter', bubbles: true });
  await expect(page.locator('.result-value')).toHaveText('= 5');
  expect((await sounds(page)).length).toBe(before);
});

test('lost-delivery retry keeps the accepted identity and never adds a dispatch or button sound', async ({ page }) => {
  await observeAudio(page);
  await openCalculator(page);
  await page.locator('#expression').click();
  const attempts: CalculationRequest[] = [];
  let accepted: CalculationResponse | undefined;
  await page.route('**/api/calculations', async (route) => {
    attempts.push(route.request().postDataJSON() as CalculationRequest);
    if (attempts.length !== 1) return route.continue();
    const reply = await route.fetch();
    expect(reply.status()).toBe(200);
    accepted = await reply.json();
    await route.abort('failed');
  });
  await page.locator('#expression').fill('9+10');
  await activationSound(page, () => page.locator('#expression').press('Enter'));
  const retry = page.locator('.calculation-result .secondary-button');
  await expect(retry).toBeVisible();
  const beforeRetry = (await sounds(page)).length;
  const response = page.waitForResponse((reply) => new URL(reply.url()).pathname === '/api/calculations');
  await retry.click();
  const repeated: CalculationResponse = await (await response).json();
  expect(repeated.calculation.id).toBe(accepted?.calculation.id);
  expect(attempts).toHaveLength(2);
  expect(attempts[1]).toEqual(attempts[0]);
  await expect(page.locator('.result-value')).toHaveText('= 19');
  expect((await sounds(page)).length).toBe(beforeRetry);
});

test('rapid native activations replace button and submit voices in both directions, with no late queue', async ({ page }) => {
  await observeAudio(page);
  await openCalculator(page);
  await page.locator('#expression').click();
  await page.locator('#tool-keypad').click();
  await settleAudio(page);
  const editor = page.locator('#expression');
  const digit = page.locator('.calculator-keys').getByRole('button', { name: 'Вставить 1', exact: true });
  const submit = page.locator('.editor-console .calculator-submit');
  await editor.fill('5+6');
  const first = (await sounds(page)).length;
  await trustedClickPair(page, digit, submit);
  await expect(page.locator('.result-value')).toHaveText('= 66');
  const editThenSubmit = (await sounds(page)).slice(first);
  expect(editThenSubmit).toHaveLength(2); // Two deliberate activations, never double cues.
  const [edit, submission] = editThenSubmit;
  expect(edit!.disconnectedAt).not.toBeNull();
  expect(edit!.disconnectedAt!).toBeLessThanOrEqual(submission!.voices[0]!.start!);
  expect(edit!.voices.every((voice) => voice.stops.some((stop) => stop < voice.stops[0]!))).toBe(true);

  await settleAudio(page);
  await editor.fill('8+3');
  const second = (await sounds(page)).length;
  await trustedClickPair(page, submit, digit);
  await expect(page.locator('.result-value')).toHaveText('= 11');
  await expect(editor).toHaveValue('8+31');
  const submitThenEdit = (await sounds(page)).slice(second);
  expect(submitThenEdit).toHaveLength(2);
  const [earlierSubmit, laterEdit] = submitThenEdit;
  expect(earlierSubmit!.disconnectedAt).not.toBeNull();
  expect(earlierSubmit!.disconnectedAt!).toBeLessThanOrEqual(laterEdit!.voices[0]!.start!);
  expect(earlierSubmit!.voices.every((voice) => voice.stops.some((stop) => stop < voice.stops[0]!))).toBe(true);
  await settleAudio(page);
  expect((await sounds(page)).length).toBe(second + submitThenEdit.length);

  await editor.fill('9+4');
  const beforeMute = (await sounds(page)).length;
  await trustedClickPair(page, submit, page.locator('#sound-toggle'));
  await expect(page.locator('.result-value')).toHaveText('= 13');
  await expect(page.locator('#sound-toggle')).toHaveAttribute('aria-pressed', 'false');
  await settleAudio(page);
  const mutedSubmission = (await sounds(page))[beforeMute]!;
  expect(mutedSubmission.voices.every((voice) => voice.stops.some((stop) => stop < voice.stops[0]!))).toBe(true);
  const mutedCount = (await sounds(page)).length;
  await digit.click();
  await page.locator('#tool-functions').click();
  await calculate(page, '9+6');
  expect((await sounds(page)).length).toBe(mutedCount);
  await page.locator('#sound-toggle').click();
  expect((await sounds(page)).length).toBe(mutedCount);
  await activationSound(page, () => page.locator('#tool-functions').click());
  await settleAudio(page);
});

test('award preempts ordinary audio; mute cancels every voice and unmute never replays a visible award', async ({ page }) => {
  await observeAudio(page);
  await openCalculator(page);
  await page.locator('#expression').click();
  const held = await holdReply(page, '**/api/calculations');
  await page.locator('#expression').fill('6*7');
  await page.locator('#expression').press('Enter');
  await held.received;
  await settleAudio(page);
  const ordinary = await activationSound(page, () => page.locator('#tool-functions').press('Enter'));
  const ordinaryIndex = (await sounds(page)).length - 1;
  await held.deliver();
  await expect(page.getByRole('complementary', { name: 'Новое достижение', exact: true })).toBeVisible();
  const active = await sounds(page);
  const interrupted = active[ordinaryIndex]!;
  const award = active.at(-1)!;
  expect(profile(award)).not.toEqual(profile(ordinary));
  expect(interrupted.disconnectedAt).not.toBeNull();
  expect(interrupted.voices.every((voice) => voice.stops.some((stop) => stop < voice.stops[0]!))).toBe(true);
  const beforeMute = active.length;
  await page.locator('#tool-keypad').press('Enter');
  expect((await sounds(page)).length).toBe(beforeMute);
  await page.locator('#sound-toggle').press('Enter');
  await expect(page.locator('#sound-toggle')).toHaveAttribute('aria-pressed', 'false');
  await settleAudio(page);
  const mutedAward = (await sounds(page)).at(-1)!;
  expect(mutedAward.disconnectedAt).not.toBeNull();
  expect(mutedAward.voices.some((voice) => voice.stops.some((stop) => stop < voice.stops[0]!))).toBe(true);
  await page.locator('#tool-functions').click();
  await calculate(page, '2+8');
  expect((await sounds(page)).length).toBe(beforeMute);
  await page.locator('#sound-toggle').press('Space');
  await expect(page.locator('#sound-toggle')).toHaveAttribute('aria-pressed', 'true');
  await calculate(page, '4+5');
  await page.locator('#tool-keypad').click();
  expect((await sounds(page)).length).toBe(beforeMute);
  await page.getByRole('button', { name: 'Закрыть уведомление о достижении', exact: true }).click();
  await activationSound(page, () => page.locator('#tool-functions').click());
  await settleAudio(page);
});
