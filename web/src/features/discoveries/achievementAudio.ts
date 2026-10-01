type Cue = {
  output: GainNode;
  voices: Map<OscillatorNode, GainNode>;
  cleanup: number;
};

export type ButtonCue = 'open' | 'close' | 'edit' | 'toggle';

type OrdinaryMotif = {
  waveform: OscillatorType;
  from: number;
  to: number;
  turn?: number;
  duration: number;
  volume: number;
};

// Fourfold amplitude lift (+12.04 dB); the loudest envelope peaks at .336.
const SUBMIT_MOTIF: OrdinaryMotif = { waveform: 'triangle', from: 440, to: 330, duration: .065, volume: .8 };
const BUTTON_MOTIFS: Record<ButtonCue, OrdinaryMotif> = {
  open: { waveform: 'triangle', from: 300, to: 600, duration: .075, volume: .448 },
  close: { waveform: 'triangle', from: 600, to: 300, duration: .06, volume: .448 },
  edit: { waveform: 'triangle', from: 720, to: 680, duration: .035, volume: .384 },
  toggle: { waveform: 'sine', from: 500, turn: 750, to: 500, duration: .08, volume: .448 },
};

/** One gesture-unlocked device, one replaceable ordinary cue, award priority. */
export class AchievementAudio {
  private context: AudioContext | null = null;
  private cue: Cue | null = null;
  private ordinaryCue: Cue | null = null;

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
    // The same fourfold lift preserves the square-led award's priority. At most
    // two .48 envelopes overlap, bounded by .864; ordinary cues cannot overlap
    // each other or an award, so output retains at least 1.27 dB of headroom.
    output.gain.value = .9;
    output.connect(context.destination);
    const cue: Cue = { output, voices: new Map(), cleanup: 0 };
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
      cue.voices.set(voice, envelope);
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
    this.playOrdinary(SUBMIT_MOTIF);
  }

  playButton(cue: ButtonCue): void {
    this.playOrdinary(BUTTON_MOTIFS[cue]);
  }

  private playOrdinary(motif: OrdinaryMotif) {
    const context = this.context;
    if (!context || context.state !== 'running' || document.hidden || this.cue) return;
    this.stopCue(this.ordinaryCue);
    this.ordinaryCue = null;
    const start = context.currentTime + .003;
    const output = context.createGain();
    output.gain.value = motif.volume;
    output.connect(context.destination);
    const voice = context.createOscillator();
    const envelope = context.createGain();
    voice.type = motif.waveform;
    voice.frequency.setValueAtTime(motif.from, start);
    if (motif.turn !== undefined) {
      voice.frequency.exponentialRampToValueAtTime(motif.turn, start + motif.duration / 2);
    }
    voice.frequency.exponentialRampToValueAtTime(motif.to, start + motif.duration);
    envelope.gain.setValueAtTime(0, start);
    envelope.gain.linearRampToValueAtTime(.42, start + .004);
    envelope.gain.exponentialRampToValueAtTime(.001, start + motif.duration - .005);
    envelope.gain.linearRampToValueAtTime(0, start + motif.duration);
    voice.connect(envelope);
    envelope.connect(output);
    const cue: Cue = { output, voices: new Map([[voice, envelope]]), cleanup: 0 };
    this.ordinaryCue = cue;
    voice.onended = () => {
      cue.voices.delete(voice);
      voice.disconnect();
      envelope.disconnect();
    };
    voice.start(start);
    voice.stop(start + motif.duration);
    cue.cleanup = window.setTimeout(() => {
      if (this.ordinaryCue !== cue) return;
      this.ordinaryCue = null;
      this.stopCue(cue);
    }, Math.ceil(motif.duration * 1_000) + 35);
  }

  stop() {
    const award = this.cue;
    const ordinary = this.ordinaryCue;
    this.cue = null;
    this.ordinaryCue = null;
    this.stopCue(award);
    this.stopCue(ordinary);
  }

  private stopCue(cue: Cue | null) {
    if (!cue) return;
    window.clearTimeout(cue.cleanup);
    cue.output.gain.cancelScheduledValues(0);
    cue.output.gain.value = 0;
    cue.output.disconnect();
    for (const [voice, envelope] of cue.voices) {
      voice.onended = null;
      voice.stop();
      voice.disconnect();
      envelope.disconnect();
    }
    cue.voices.clear();
  }

  dispose() {
    this.stop();
    const context = this.context;
    this.context = null;
    if (context && context.state !== 'closed') void context.close().catch(() => undefined);
  }
}
