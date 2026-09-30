type Cue = {
  output: GainNode;
  voices: Set<OscillatorNode>;
  cleanup: number;
};

/** Shared gesture audio: a quiet dispatch tick and the authored award signal. */
export class AchievementAudio {
  private context: AudioContext | null = null;
  private cue: Cue | null = null;
  private submitCue: Cue | null = null;

  // Called synchronously by the host's trusted pointer/key handler, never by
  // bootstrap, a catalog refresh or an effect that attempts to defeat autoplay.
  unlock() {
    if (typeof window.AudioContext !== 'function') return;
    try {
      this.context ??= new AudioContext();
      if (this.context.state === 'suspended') void this.context.resume().catch(() => undefined);
    } catch {
      // An unavailable audio device must not prevent earning or reading awards.
    }
  }

  play() {
    const context = this.context;
    if (!context || context.state !== 'running' || document.hidden) return;
    this.stop();
    const start = context.currentTime + .012;
    const output = context.createGain();
    output.gain.value = .075;
    output.connect(context.destination);
    const cue: Cue = { output, voices: new Set(), cleanup: 0 };
    this.cue = cue;
    const steps = [
      { offset: 0, semitone: 0, length: .13 },
      { offset: .085, semitone: 5, length: .15 },
      { offset: .18, semitone: 12, length: .17 },
      { offset: .29, semitone: 17, length: .22 },
      { offset: .44, semitone: 24, length: .36 },
    ];
    for (const step of steps) {
      const voice = context.createOscillator();
      const envelope = context.createGain();
      voice.type = step.semitone === 24 ? 'triangle' : 'square';
      voice.frequency.value = 220 * 2 ** (step.semitone / 12);
      const at = start + step.offset;
      envelope.gain.setValueAtTime(0, at);
      envelope.gain.linearRampToValueAtTime(.48, at + .007);
      envelope.gain.exponentialRampToValueAtTime(.045, at + step.length - .025);
      envelope.gain.linearRampToValueAtTime(0, at + step.length);
      voice.connect(envelope);
      envelope.connect(output);
      cue.voices.add(voice);
      voice.onended = () => {
        cue.voices.delete(voice);
        voice.disconnect();
        envelope.disconnect();
      };
      voice.start(at);
      voice.stop(at + step.length + .01);
    }
    cue.cleanup = window.setTimeout(() => {
      if (this.cue === cue) this.stop();
    }, 950);
  }

  playSubmit() {
    const context = this.context;
    if (!context || context.state !== 'running' || document.hidden || this.cue) return;
    this.stopCue(this.submitCue);
    const start = context.currentTime + .003;
    const output = context.createGain();
    output.gain.value = .025;
    output.connect(context.destination);
    const voice = context.createOscillator();
    const envelope = context.createGain();
    voice.type = 'triangle';
    voice.frequency.setValueAtTime(440, start);
    voice.frequency.exponentialRampToValueAtTime(330, start + .065);
    envelope.gain.setValueAtTime(0, start);
    envelope.gain.linearRampToValueAtTime(.42, start + .004);
    envelope.gain.exponentialRampToValueAtTime(.001, start + .06);
    envelope.gain.linearRampToValueAtTime(0, start + .065);
    voice.connect(envelope);
    envelope.connect(output);
    const cue: Cue = { output, voices: new Set([voice]), cleanup: 0 };
    this.submitCue = cue;
    voice.onended = () => {
      cue.voices.delete(voice);
      voice.disconnect();
      envelope.disconnect();
    };
    voice.start(start);
    voice.stop(start + .065);
    cue.cleanup = window.setTimeout(() => {
      if (this.submitCue !== cue) return;
      this.submitCue = null;
      this.stopCue(cue);
    }, 100);
  }

  stop() {
    const award = this.cue;
    const submission = this.submitCue;
    this.cue = null;
    this.submitCue = null;
    this.stopCue(award);
    this.stopCue(submission);
  }

  private stopCue(cue: Cue | null) {
    if (!cue) return;
    window.clearTimeout(cue.cleanup);
    cue.output.gain.cancelScheduledValues(0);
    cue.output.gain.value = 0;
    cue.output.disconnect();
    for (const voice of cue.voices) voice.stop();
  }

  dispose() {
    this.stop();
    const context = this.context;
    this.context = null;
    if (context && context.state !== 'closed') void context.close().catch(() => undefined);
  }
}
