import { useCallback, useEffect, useLayoutEffect, useRef, useState, type CSSProperties, type ReactNode } from 'react';
import type { Achievement, DiscoveryDefinition } from '../../contracts';
import { AchievementAudio } from './achievementAudio';
import { achievementAccent, achievementArtwork } from './achievementPresentation';
import { AchievementDescription } from './AchievementDescription';
import './achievement-celebration.css';

export type AchievementCelebrationOptions = {
  identity: string | null;
  catalog: DiscoveryDefinition[];
  soundEnabled: boolean;
  effectsEnabled: boolean;
  onOpenCollection: (id: string) => void;
};

type AwardMoment = {
  award: Achievement;
  definition: DiscoveryDefinition;
  sequence: number;
  sounded: boolean;
};

type AwardQueue = {
  identity: string | null;
  seen: Set<string>;
  waiting: Achievement[];
  current: AwardMoment | null;
  sequence: number;
};

type QueueSnapshot = {
  identity: string | null;
  current: AwardMoment | null;
  waitingCount: number;
  freshAwardIds: readonly string[];
};

const EMPTY_AWARD_IDS: readonly string[] = [];

function freshQueue(identity: string | null): AwardQueue {
  return { identity, seen: new Set(), waiting: [], current: null, sequence: 0 };
}

export function useAchievementCelebration(options: AchievementCelebrationOptions): {
  enqueue: (awards: Achievement[]) => void;
  view: ReactNode;
  unlockAudio: () => void;
  playSubmitSound: () => void;
  freshAwardIds: readonly string[];
  active: boolean;
} {
  const queue = useRef<AwardQueue>(freshQueue(options.identity));
  const committedOptions = useRef(options);
  const audio = useRef<AchievementAudio | null>(null);
  const mounted = useRef(false);
  const [snapshot, setSnapshot] = useState<QueueSnapshot>({ identity: options.identity, current: null, waitingCount: 0, freshAwardIds: EMPTY_AWARD_IDS });

  const publish = useCallback(() => {
    const state = queue.current;
    setSnapshot((previous) => (
      previous.identity === state.identity &&
      previous.current === state.current &&
      previous.waitingCount === state.waiting.length &&
      previous.freshAwardIds.length === state.seen.size
        ? previous
        : {
          identity: state.identity, current: state.current, waitingCount: state.waiting.length,
          freshAwardIds: previous.identity === state.identity && previous.freshAwardIds.length === state.seen.size
            ? previous.freshAwardIds : Array.from(state.seen),
        }
    ));
  }, []);

  const advance = useCallback(() => {
    const state = queue.current;
    const award = state.waiting[0];
    if (state.current || !award) return;
    const definition = committedOptions.current.catalog.find((item) => item.id === award.id);
    // Keep the authoritative award until its metadata arrives; catalog outages
    // must never turn a genuine grant into a lost or fabricated ceremony.
    if (!definition) return;
    state.waiting.shift();
    state.current = { award, definition, sequence: ++state.sequence, sounded: false };
  }, []);

  useLayoutEffect(() => {
    committedOptions.current = options;
    if (queue.current.identity !== options.identity) {
      audio.current?.stop();
      queue.current = freshQueue(options.identity);
    }
    if (!options.soundEnabled) audio.current?.stop();
    advance();
    publish();
  }, [options.identity, options.catalog, options.soundEnabled, options.effectsEnabled, options.onOpenCollection, advance, publish]);

  const enqueue = useCallback((awards: Achievement[]) => {
    const state = queue.current;
    for (const award of awards) {
      if (!award.id || state.seen.has(award.id)) continue;
      state.seen.add(award.id);
      state.waiting.push(award);
    }
    advance();
    publish();
  }, [advance, publish]);

  const dismiss = useCallback((sequence: number) => {
    if (queue.current.current?.sequence !== sequence) return;
    audio.current?.stop();
    queue.current.current = null;
    advance();
    publish();
  }, [advance, publish]);

  const unlockAudio = useCallback(() => {
    if (!committedOptions.current.soundEnabled) return;
    audio.current ??= new AchievementAudio();
    audio.current.unlock();
  }, []);

  const playSubmitSound = useCallback(() => {
    if (!committedOptions.current.soundEnabled) return;
    audio.current ??= new AchievementAudio();
    audio.current.unlock();
    audio.current.playSubmit();
  }, []);

  useEffect(() => {
    mounted.current = true;
    const onVisibilityChange = () => { if (document.hidden) audio.current?.stop(); };
    document.addEventListener('visibilitychange', onVisibilityChange);
    return () => {
      mounted.current = false;
      document.removeEventListener('visibilitychange', onVisibilityChange);
      queueMicrotask(() => { if (!mounted.current) audio.current?.dispose(); });
    };
  }, []);

  useEffect(() => {
    const current = snapshot.current;
    if (!current || snapshot.identity !== options.identity || current.sounded) return;
    // Record consumption even when muted/locked: later preference changes or
    // gesture unlocks do not replay a sound for an already-presented award.
    current.sounded = true;
    if (options.soundEnabled) audio.current?.play();
  }, [snapshot.current, snapshot.identity, options.identity, options.soundEnabled]);

  const sameOwner = snapshot.identity === options.identity;
  const current = sameOwner ? snapshot.current : null;
  const active = sameOwner && (current !== null || snapshot.waitingCount > 0);
  const view = current ? (
    <AwardCeremony
      key={`${snapshot.identity}:${current.sequence}:${current.award.id}`}
      moment={current}
      effectsEnabled={options.effectsEnabled}
      waitingCount={snapshot.waitingCount}
      onDismiss={() => dismiss(current.sequence)}
      onOpenCollection={() => {
        dismiss(current.sequence);
        committedOptions.current.onOpenCollection(current.award.id);
      }}
    />
  ) : null;
  return { enqueue, view, unlockAudio, playSubmitSound, freshAwardIds: sameOwner ? snapshot.freshAwardIds : EMPTY_AWARD_IDS, active };
}

// A finite, deterministic pixel burst, authored once per award. Coordinates and
// timings are geometry, not an icon or substitute for the eight real sprites.
const sparks = Array.from({ length: 36 }, (_, index) => {
  const angle = index * Math.PI * 2 / 36;
  const reach = 90 + (index % 4) * 34;
  return {
    '--spark-origin-x': `${12 + (index % 6) * 15}%`,
    '--spark-origin-y': `${16 + Math.floor(index / 6) * 13}%`,
    '--spark-x': `${Math.round(Math.cos(angle) * reach)}px`,
    '--spark-y': `${Math.round(Math.sin(angle) * reach * .66)}px`,
    '--spark-delay': `${(index % 6) * 35}ms`,
    '--spark-size': `${index % 3 === 0 ? 12 : 6}px`,
  } as CSSProperties;
});

function AwardCeremony({ moment, effectsEnabled, waitingCount, onDismiss, onOpenCollection }: {
  moment: AwardMoment;
  effectsEnabled: boolean;
  waitingCount: number;
  onDismiss: () => void;
  onOpenCollection: () => void;
}) {
  const card = useRef<HTMLElement | null>(null);
  const previousFocus = useRef<HTMLElement | null>(null);
  const remaining = useRef(7_500);
  const started = useRef(0);
  const burstDeadline = useRef(performance.now() + 1_250);
  const initialEffects = useRef(effectsEnabled);
  const [burst, setBurst] = useState(true);
  const [hovered, setHovered] = useState(false);
  const [focused, setFocused] = useState(false);
  const [hidden, setHidden] = useState(document.hidden);
  const dismissed = useRef(false);
  const dismissHandler = useRef(onDismiss);
  dismissHandler.current = onDismiss;

  const restoreFocus = useCallback(() => {
    if (!card.current?.contains(document.activeElement)) return;
    const previous = previousFocus.current;
    const target = previous?.isConnected && !previous.matches(':disabled')
      ? previous : document.getElementById('expression');
    target?.focus({ preventScroll: true });
  }, []);

  const setCard = useCallback((node: HTMLElement | null) => {
    if (!node) restoreFocus();
    card.current = node;
  }, [restoreFocus]);

  const finish = useCallback(() => {
    if (dismissed.current) return;
    dismissed.current = true;
    restoreFocus();
    dismissHandler.current();
  }, [restoreFocus]);

  useEffect(() => {
    const delay = Math.max(0, burstDeadline.current - performance.now());
    const timer = window.setTimeout(() => setBurst(false), delay);
    return () => window.clearTimeout(timer);
  }, []);

  useEffect(() => {
    const onVisibilityChange = () => setHidden(document.hidden);
    document.addEventListener('visibilitychange', onVisibilityChange);
    return () => document.removeEventListener('visibilitychange', onVisibilityChange);
  }, []);

  useEffect(() => {
    // The display itself is click-through. Measure hover without making the
    // editor behind it inert or turning the spectacle into an invisible hitbox.
    const onPointerMove = (event: PointerEvent) => {
      if (event.pointerType === 'touch') { setHovered(false); return; }
      const rect = card.current?.getBoundingClientRect();
      setHovered(Boolean(rect && event.clientX >= rect.left && event.clientX <= rect.right &&
        event.clientY >= rect.top && event.clientY <= rect.bottom));
    };
    const onPointerLeave = () => setHovered(false);
    document.addEventListener('pointermove', onPointerMove, { passive: true });
    document.addEventListener('pointerleave', onPointerLeave);
    return () => {
      document.removeEventListener('pointermove', onPointerMove);
      document.removeEventListener('pointerleave', onPointerLeave);
    };
  }, []);

  // The brief's readable interval starts after the one-shot ritual. Timers
  // preserve their remaining duration over hover, focus, background and replay.
  useEffect(() => {
    if (burst || hovered || focused || hidden) return;
    started.current = performance.now();
    const timer = window.setTimeout(finish, remaining.current);
    return () => {
      window.clearTimeout(timer);
      remaining.current = Math.max(0, remaining.current - (performance.now() - started.current));
    };
  }, [burst, hovered, focused, hidden, finish]);

  const openCollection = () => {
    if (dismissed.current) return;
    dismissed.current = true;
    restoreFocus();
    onOpenCollection();
  };

  return (
    <div className={`achievement-ceremony-stage${effectsEnabled && initialEffects.current ? ' achievement-ceremony-stage--effects' : ''}${burst ? ' achievement-ceremony-stage--burst' : ''}`} style={achievementAccent(moment.award.id)}>
      {effectsEnabled && initialEffects.current && <div className="achievement-ceremony__spectacle" aria-hidden="true">
        <div className="achievement-ceremony__wave" />
        <div className="achievement-ceremony__sparks">{sparks.map((style, index) => <i key={index} style={style} />)}</div>
      </div>}
      <aside
        ref={setCard}
        className="achievement-ceremony"
        aria-label="Новое достижение"
        data-achievement-id={moment.award.id}
        onFocus={(event) => {
          if (!event.currentTarget.contains(event.relatedTarget)) {
            previousFocus.current = event.relatedTarget instanceof HTMLElement ? event.relatedTarget : document.getElementById('expression');
          }
          setFocused(true);
        }}
        onBlur={(event) => {
          if (!event.currentTarget.contains(event.relatedTarget)) setFocused(false);
        }}
        onKeyDown={(event) => {
          if (event.key === 'Escape' && !event.nativeEvent.isComposing) {
            event.preventDefault(); event.stopPropagation(); finish();
          }
        }}
      >
        <img className="achievement-ceremony__artwork" src={achievementArtwork(moment.award.id)} width="128" height="128" alt="" />
        <div className="achievement-ceremony__copy">
          <h2>Достижение получено!</h2>
          <h3>{moment.definition.ru.name}</h3>
          <AchievementDescription
            text={moment.definition.ru.description}
            locked={false}
            concealed={false}
            active
            freshCeremony={moment.definition.secret}
            awardId={moment.award.id}
            className="achievement-ceremony__description"
          />
          <div className="achievement-ceremony__actions">
            <button type="button" onClick={openCollection}>В коллекцию</button>
            {waitingCount > 0 && <span className="achievement-ceremony__queued">Следом: {waitingCount}</span>}
          </div>
        </div>
        <button className="achievement-ceremony__close" type="button" onClick={finish} aria-label="Закрыть уведомление о достижении">
          <svg width="18" height="18" viewBox="0 0 18 18" fill="none" aria-hidden="true"><path d="m4 4 10 10M14 4 4 14" stroke="currentColor" strokeWidth="1.5" /></svg>
        </button>
      </aside>
      <span className="visually-hidden" role="status" aria-live="polite" aria-atomic="true">Достижение получено! {moment.definition.ru.name}. {moment.definition.ru.description}</span>
    </div>
  );
}
