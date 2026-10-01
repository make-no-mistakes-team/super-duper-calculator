import { useEffect, useLayoutEffect, useRef, useState } from 'react';
import type { CSSProperties, RefObject } from 'react';
import { createPortal, flushSync } from 'react-dom';
import type { CalculationRecord, DiscoveryDefinition, FunEvent } from '../../contracts';
import { PHRASE_CATALOG } from './phraseCatalog';
import type { CommentaryPhrase, PhraseContext } from './phraseCatalog';
import './calculator-personality.css';

export type CalculatorPersonalityProps = {
  identity: string | null;
  calculation: CalculationRecord | null;
  speechEligible: boolean;
  submittedAt: number | null;
  events: FunEvent[];
  catalog: DiscoveryDefinition[];
  enabled: boolean;
  effectsEnabled: boolean;
  suppressed: boolean;
  loading: boolean;
  anchorRef: RefObject<HTMLElement | null>;
};

type FaceMood = 'idle' | 'loading' | 'success' | 'error' | 'recovery';
type Edge = 'top' | 'bottom' | 'left' | 'right';
type Bounds = { left: number; top: number; right: number; bottom: number };
type Placement = { left: number; top: number; edge: Edge; slot: string; tail: Bounds };
type Speech = {
  id: string;
  text: string;
  phrase: CommentaryPhrase | null;
  context: PhraseContext | null;
  expiresAt: number;
  placement: Placement | null;
  phase: 'measuring' | 'entering' | 'leaving';
  startedAt: number | null;
};

const START_GAP_MS = 5_000;
const DISPLAY_MS = 4_500;
const RECENT_PHRASE_LIMIT = 32;
const CONSUMED_CALCULATION_LIMIT = 256;
const PRIORITY_SELECTOR = 'dialog[open], [aria-modal="true"], .comic-incident, .achievement-ceremony';
const PROTECTED_SELECTOR = `[data-speech-protected], ${PRIORITY_SELECTOR}`;
const touchGrassComments: Record<number, string> = {
  50: '50 вычислений. Клавиатуре тоже нужен перерыв.',
  100: '100 вычислений. Калькулятор впечатлён. Улица всё ещё существует.',
};

function intersects(a: Bounds, b: Bounds, gap = 0) {
  return a.left < b.right + gap && a.right > b.left - gap &&
    a.top < b.bottom + gap && a.bottom > b.top - gap;
}

function viewportBounds(): Bounds {
  const viewport = window.visualViewport;
  const left = viewport?.offsetLeft ?? 0;
  const top = viewport?.offsetTop ?? 0;
  return { left: left + 10, top: top + 10,
    right: left + (viewport?.width ?? document.documentElement.clientWidth) - 10,
    bottom: top + (viewport?.height ?? document.documentElement.clientHeight) - 10 };
}

function visibleBounds(element: Element): DOMRect | null {
  if (!element.getClientRects().length) return null;
  const style = getComputedStyle(element);
  if (style.visibility === 'hidden' || style.display === 'none' || style.opacity === '0') return null;
  const rect = element.getBoundingClientRect();
  return rect.width > 0 && rect.height > 0 ? rect : null;
}

function protectedBounds() {
  return [...document.querySelectorAll(PROTECTED_SELECTOR)].flatMap<Bounds>((element) => {
    const rect = visibleBounds(element);
    if (!rect) return [];
    if (!(element instanceof HTMLElement) ||
      !element.getAnimations().some((animation) => animation.playState === 'running')) return [rect];
    const style = getComputedStyle(element);
    if (style.transform === 'none') return [rect];
    // Protect the settled box as well as the current animated box. A newly
    // disclosed sheet must not sweep across speech between browser frames.
    const matrix = new DOMMatrixReadOnly(style.transform);
    const origin = style.transformOrigin.split(' ');
    const originX = Number.parseFloat(origin[0] ?? '0');
    const originY = Number.parseFloat(origin[1] ?? '0');
    const width = element.offsetWidth;
    const height = element.offsetHeight;
    const corners = [[0, 0], [width, 0], [0, height], [width, height]].map(([x = 0, y = 0]) => ({
      x: (x - originX) * matrix.a + (y - originY) * matrix.c + matrix.e + originX,
      y: (x - originX) * matrix.b + (y - originY) * matrix.d + matrix.f + originY,
    }));
    const left = rect.left - Math.min(...corners.map((point) => point.x));
    const top = rect.top - Math.min(...corners.map((point) => point.y));
    // Authored console disclosures overshoot their settled box briefly.
    return [{
      left: Math.min(rect.left, left) - 16, top: Math.min(rect.top, top) - 16,
      right: Math.max(rect.right, left + width) + 16,
      bottom: Math.max(rect.bottom, top + height) + 16,
    }];
  });
}

function safePlacements(anchor: DOMRect, width: number, height: number, obstacles: Bounds[]): Placement[] {
  const viewport = viewportBounds();
  const candidates: Placement[] = [];
  const gap = 16;
  const tailWidth = 12;
  const add = (edge: Edge, fraction: number) => {
    let left = anchor.left + anchor.width * fraction - width / 2;
    let top = anchor.top + anchor.height * fraction - height / 2;
    let tail: Bounds;
    if (edge === 'top' || edge === 'bottom') {
      left = Math.max(viewport.left + 12, Math.min(left, viewport.right - width - 12));
      top = edge === 'top' ? anchor.top - gap - height : anchor.bottom + gap;
      const x = Math.max(anchor.left + 16, Math.min(left + width / 2, anchor.right - 16));
      tail = { left: x - tailWidth / 2, right: x + tailWidth / 2,
        top: edge === 'top' ? top + height : anchor.bottom,
        bottom: edge === 'top' ? anchor.top : top };
    } else {
      top = Math.max(viewport.top + 12, Math.min(top, viewport.bottom - height - 12));
      left = edge === 'left' ? anchor.left - gap - width : anchor.right + gap;
      const y = Math.max(anchor.top + 16, Math.min(top + height / 2, anchor.bottom - 16));
      tail = { top: y - tailWidth / 2, bottom: y + tailWidth / 2,
        left: edge === 'left' ? left + width : anchor.right,
        right: edge === 'left' ? anchor.left : left };
    }
    // Reserve the whole squash/stretch envelope, not just the final text rectangle.
    const envelope = { left: left - 12, top: top - 12, right: left + width + 12, bottom: top + height + 12 };
    if (envelope.left < viewport.left || envelope.top < viewport.top ||
      envelope.right > viewport.right || envelope.bottom > viewport.bottom) return;
    // The tail attaches to the decorative console border. Inflating its obstacle
    // check would reject the adjacent result even though the border separates
    // them; require actual tail clearance, while padding the animated bubble.
    if (obstacles.some((rect) => intersects(envelope, rect, 3) || intersects(tail, rect))) return;
    candidates.push({ left, top, edge, slot: `${edge}-${fraction}`, tail });
  };
  for (const edge of ['top', 'right', 'bottom', 'left'] as const) {
    for (const fraction of [0.25, 0.5, 0.75]) add(edge, fraction);
  }
  return candidates;
}

function contextsFor(record: CalculationRecord, previousError: boolean, repeated: boolean): PhraseContext[] {
  if (record.outcome.kind === 'error') {
    switch (record.outcome.error.code) {
      case 'SYNTAX_ERROR': return ['syntax'];
      case 'DIVISION_BY_ZERO': return ['division_zero'];
      case 'DOMAIN_ERROR': return ['domain'];
      case 'UNKNOWN_IDENTIFIER':
      case 'UNSUPPORTED_FEATURE':
      case 'WRONG_ARITY':
      case 'NUMERIC_OVERFLOW': return ['math_error'];
      default: return [];
    }
  }
  const contexts: PhraseContext[] = ['ordinary'];
  const value = Number(record.outcome.value);
  const magnitude = Math.abs(value);
  if (record.outcome.value === '0') contexts.push('zero');
  if (value < 0) contexts.push('negative');
  if (magnitude > 1e12) contexts.push('huge');
  if (magnitude > 0 && magnitude < 1e-12) contexts.push('tiny');
  if (record.facts && record.facts.operationCount >= 8) contexts.push('complex');
  if (record.facts && Object.values(record.facts.functions).some((count) => count > 0)) contexts.push('functions');
  if (record.facts && record.facts.depth >= 3) contexts.push('nesting');
  if (repeated) contexts.push('repeat');
  if (previousError) contexts.push('recovery');
  if (Math.random() < 0.05) contexts.push('philosophy');
  if (Math.random() < 0.02) contexts.push('hype');
  return contexts;
}

function selectPhrase(contexts: PhraseContext[], used: Set<string>, recent: PhraseContext[]): { phrase: CommentaryPhrase; context: PhraseContext } | null {
  const groups = contexts.flatMap((context) => {
    const phrases = PHRASE_CATALOG.filter((phrase) => phrase.contexts.includes(context) && !used.has(phrase.id));
    if (!phrases.length) return [];
    const frequency = recent.filter((item) => item === context).length;
    const weight = (context === 'ordinary' ? 1 : context === 'philosophy' || context === 'hype' ? 0.5 : 4) / (1 + frequency * 3);
    return [{ phrases, weight, context }];
  });
  let draw = Math.random() * groups.reduce((sum, group) => sum + group.weight, 0);
  for (const group of groups) {
    draw -= group.weight;
    if (draw <= 0) {
      const phrase = group.phrases[Math.floor(Math.random() * group.phrases.length)];
      return phrase ? { phrase, context: group.context } : null;
    }
  }
  return null;
}

function PixelFace({ mood, reaction }: { mood: FaceMood; reaction: string }) {
  return (
    <svg key={reaction} className="calculator-personality__pixels" data-mood={mood} viewBox="0 0 32 24" width="48" height="36" fill="currentColor" shapeRendering="crispEdges" aria-hidden="true">
      <path className="calculator-personality__face-surface" d="M4 2h24v2h2v16h-2v2H4v-2H2V4h2z" />
      <path className="calculator-personality__face-frame" d="M4 2h24v2h2v16h-2v2H4v-2H2V4h2zm0 4v12h2v2h20v-2h2V6h-2V4H6v2z" />
      <g className="calculator-personality__eyes">
        <g className="calculator-personality__eye-lids">
          {mood === 'success' || mood === 'recovery' ? <path d="M8 9h2V7h4v2h2v2h-2V9h-4v2H8zm10 0h2V7h4v2h2v2h-2V9h-4v2h-2z" /> :
            mood === 'error' ? <path d="M8 7h2v2h4V7h2v2h-2v2h2v2h-2v-2h-4v2H8v-2h2V9H8zm10 0h2v2h4V7h2v2h-2v2h2v2h-2v-2h-4v2h-2v-2h2V9h-2z" /> :
              <path d="M8 7h6v6H8zm12 0h6v6h-6z" />}
        </g>
      </g>
      <path className="calculator-personality__mouth" d={mood === 'error' ? 'M12 17h2v-2h6v2h2v2h-2v-2h-6v2h-2z' : mood === 'success' || mood === 'recovery' ? 'M10 15h2v2h8v-2h2v2h-2v2h-8v-2h-2z' : mood === 'loading' ? 'M12 16h8v2h-8z' : 'M12 17h8v2h-8z'} />
      <path className="calculator-personality__energy" d="M0 0h2v2H0zm30 0h2v2h-2zM0 22h2v2H0zm30 0h2v2h-2z" />
    </svg>
  );
}

/** Identity keys every timer, reaction, consumed event and anti-repeat pool. */
export function CalculatorPersonality(props: CalculatorPersonalityProps) {
  return <PersonalitySession key={props.identity ?? 'no-identity'} {...props} />;
}

function PersonalitySession({ identity, calculation, speechEligible, submittedAt, events, catalog, enabled, effectsEnabled, suppressed, loading, anchorRef }: CalculatorPersonalityProps) {
  const [speech, setSpeech] = useState<Speech | null>(null);
  const [face, setFace] = useState<{ left: number; top: number } | null>(null);
  const [foreground, setForeground] = useState(!document.hidden);
  const [reducedMotion, setReducedMotion] = useState(() => matchMedia('(prefers-reduced-motion: reduce)').matches);
  const [reaction, setReaction] = useState<{ mood: FaceMood; id: string } | null>(null);
  const bubbleRef = useRef<HTMLDivElement>(null);
  const speechRef = useRef<Speech | null>(null);
  const measureBeforePaint = useRef<(() => void) | null>(null);
  const handledCalculations = useRef(new Set<string>());
  const handledEvents = useRef(new Map<string, number>());
  const successfulExpressions = useRef(new Set<string>());
  const usedPhrases = useRef(new Set<string>());
  const recentContexts = useRef<PhraseContext[]>([]);
  const previousError = useRef(false);
  const lastStart = useRef(Number.NEGATIVE_INFINITY);
  const lastSlot = useRef<string | null>(null);
  const visibleSince = useRef(document.hidden ? Number.POSITIVE_INFINITY : Number.NEGATIVE_INFINITY);
  const minimal = !effectsEnabled || reducedMotion;
  const allowed = Boolean(identity && enabled && !suppressed && foreground);
  speechRef.current = speech;

  useEffect(() => {
    const media = matchMedia('(prefers-reduced-motion: reduce)');
    const change = () => setReducedMotion(media.matches);
    const visibility = () => {
      visibleSince.current = document.hidden ? Number.POSITIVE_INFINITY : Date.now();
      setForeground(!document.hidden);
      setSpeech(null);
      setReaction(null);
    };
    media.addEventListener('change', change);
    document.addEventListener('visibilitychange', visibility);
    return () => {
      media.removeEventListener('change', change);
      document.removeEventListener('visibilitychange', visibility);
    };
  }, []);

  useEffect(() => {
    if (!allowed) {
      setSpeech(null);
      setReaction(null);
    }
  }, [allowed]);

  // Observe every newest accepted outcome, including quiet retries, once.
  // Permission to react is separate: blocked/late outcomes never form a queue.
  useEffect(() => {
    const now = Date.now();
    // Use the browser's request timestamp, not the server's wall clock: a
    // delayed pre-resume response is stale even if delivered in the foreground.
    const fresh = submittedAt !== null && Number.isFinite(submittedAt) &&
      submittedAt > visibleSince.current && submittedAt <= now && now - submittedAt < 15_000;
    let choice: ReturnType<typeof selectPhrase> = null;
    let newCalculation = false;
    // The face follows pending work; speech owns its original display lifetime.
    if (loading) {
      setReaction(null);
    }
    if (calculation && !handledCalculations.current.has(calculation.id)) {
      handledCalculations.current.add(calculation.id);
      if (handledCalculations.current.size > CONSUMED_CALCULATION_LIMIT) {
        const oldest = handledCalculations.current.values().next().value;
        if (oldest !== undefined) handledCalculations.current.delete(oldest);
      }
      newCalculation = true;
      const wasError = previousError.current;
      const expression = calculation.facts?.normalizedExpression ?? calculation.expression.trim();
      const repeated = successfulExpressions.current.has(expression);
      const contexts = contextsFor(calculation, wasError, repeated);
      previousError.current = calculation.outcome.kind === 'error';
      if (calculation.outcome.kind === 'success') {
        successfulExpressions.current.add(expression);
        if (successfulExpressions.current.size > 64) {
          const oldest = successfulExpressions.current.values().next().value;
          if (oldest !== undefined) successfulExpressions.current.delete(oldest);
        }
      }
      if (allowed && speechEligible && fresh && !loading) {
        setReaction({ mood: calculation.outcome.kind === 'error' ? 'error' : wasError ? 'recovery' : 'success', id: calculation.id });
        if (!speechRef.current) choice = selectPhrase(contexts, usedPhrases.current, recentContexts.current);
      }
    }
    for (const [id, until] of handledEvents.current) {
      if (until <= now) handledEvents.current.delete(id);
    }
    let eventSpeech: { id: string; text: string; createdAt: number; expiresAt: number } | null = null;
    for (const event of events) {
      if (handledEvents.current.has(event.id)) continue;
      const createdAt = Date.parse(event.createdAt);
      const expiresAt = Date.parse(event.expiresAt);
      handledEvents.current.set(event.id, Math.max(now + 60_000, Number.isFinite(expiresAt) ? expiresAt : now));
      const definition = catalog.find((item) => item.id === event.ruleId);
      if (!allowed || speechRef.current || !speechEligible || !fresh || loading || event.kind !== 'comment' || event.scope !== 'personal' || !definition?.ru.comment ||
        !Number.isFinite(createdAt) || !Number.isFinite(expiresAt) || expiresAt <= now || createdAt > now ||
        createdAt <= visibleSince.current || now - createdAt >= 15_000) continue;
      if (!eventSpeech || createdAt >= eventSpeech.createdAt) {
        eventSpeech = { id: event.id, createdAt, expiresAt,
          text: event.ruleId === 'touch_grass' && typeof event.params.count === 'number'
            ? touchGrassComments[event.params.count] ?? definition.ru.comment : definition.ru.comment };
      }
    }
    if (!allowed || loading || document.hidden || speechRef.current || now - lastStart.current < START_GAP_MS ||
      (!eventSpeech && (!newCalculation || !choice)) || Math.random() >= 0.7) return;
    setSpeech({ id: eventSpeech?.id ?? choice!.phrase.id,
      text: eventSpeech?.text ?? choice!.phrase.ru,
      phrase: eventSpeech ? null : choice!.phrase, context: eventSpeech ? null : choice!.context,
      expiresAt: eventSpeech?.expiresAt ?? Number.POSITIVE_INFINITY,
      placement: null, phase: 'measuring', startedAt: null });
  }, [calculation, speechEligible, submittedAt, events, catalog, allowed, loading]);

  useEffect(() => {
    if (!reaction) return;
    const timeout = window.setTimeout(() => setReaction(null), 1_150);
    return () => window.clearTimeout(timeout);
  }, [reaction]);

  useLayoutEffect(() => {
    if (!allowed) return;
    let frame = 0;
    let disposed = false;
    const resizeObserver = new ResizeObserver(() => remeasure());
    const observeRegions = () => {
      resizeObserver.disconnect();
      if (anchorRef.current) resizeObserver.observe(anchorRef.current);
      if (bubbleRef.current) resizeObserver.observe(bubbleRef.current);
      document.querySelectorAll(PROTECTED_SELECTOR).forEach((element) => resizeObserver.observe(element));
    };
    const measure = () => {
      if (disposed || document.hidden) return;
      const anchor = anchorRef.current;
      const rect = anchor ? visibleBounds(anchor) : null;
      const viewport = viewportBounds();
      const prioritySurface = [...document.querySelectorAll(PRIORITY_SELECTOR)]
        .some((element) => !element.closest('[data-exit-snapshot]') && visibleBounds(element) !== null);
      if (prioritySurface || !rect || !intersects(rect, viewport)) {
        setFace(null);
        setSpeech(null);
        return;
      }
      const obstacles = protectedBounds();
      const facePosition = { left: rect.left + 10, top: rect.top - 20 };
      const faceBounds = { left: facePosition.left - 8, top: facePosition.top - 8, right: facePosition.left + 56, bottom: facePosition.top + 44 };
      const faceSafe = faceBounds.left >= viewport.left && faceBounds.top >= viewport.top &&
        faceBounds.right <= viewport.right && faceBounds.bottom <= viewport.bottom &&
        !obstacles.some((item) => intersects(faceBounds, item, 2));
      setFace((current) => faceSafe ? current?.left === facePosition.left && current.top === facePosition.top ? current : facePosition : null);
      const current = speechRef.current;
      const bubble = bubbleRef.current;
      if (!current || !bubble) return;
      const size = { width: bubble.offsetWidth, height: bubble.offsetHeight };
      if (faceSafe) obstacles.push(new DOMRect(faceBounds.left, faceBounds.top, 64, 52));
      const candidates = safePlacements(rect, size.width, size.height, obstacles);
      if (!candidates.length || current.expiresAt <= Date.now()) {
        setSpeech(null);
        return;
      }
      const kept = current.placement && candidates.find((candidate) => candidate.slot === current.placement?.slot);
      const alternatives = candidates.filter((candidate) => candidate.slot !== lastSlot.current);
      const choices = alternatives.length ? alternatives : candidates;
      const placement = kept || choices[Math.floor(Math.random() * choices.length)];
      if (!placement) return;
      if (current.startedAt === null) {
        const startedAt = Date.now();
        if (startedAt - lastStart.current < START_GAP_MS) { setSpeech(null); return; }
        lastStart.current = startedAt;
        lastSlot.current = placement.slot;
        if (current.phrase) {
          usedPhrases.current.add(current.phrase.id);
          if (usedPhrases.current.size > RECENT_PHRASE_LIMIT) {
            const oldest = usedPhrases.current.values().next().value;
            if (oldest !== undefined) usedPhrases.current.delete(oldest);
          }
          if (current.context) recentContexts.current = [...recentContexts.current.slice(-3), current.context];
        }
        setSpeech({ ...current, placement, phase: 'entering', startedAt });
      } else if (!current.placement || current.placement.left !== placement.left || current.placement.top !== placement.top || current.placement.slot !== placement.slot) {
        setSpeech({ ...current, placement });
      }
      // A disclosed panel can move without changing its layout box. Sample only
      // while real protected-surface motion is running, then stop the frame loop.
      if ([...document.querySelectorAll(PROTECTED_SELECTOR)].some((element) =>
        element.getAnimations().some((animation) => animation.playState === 'running'))) schedule();
    };
    const schedule = () => {
      if (!frame && !disposed) frame = requestAnimationFrame(() => {
        frame = 0;
        if (!disposed) flushSync(measure);
      });
    };
    const remeasure = () => {
      if (disposed) return;
      cancelAnimationFrame(frame);
      frame = 0;
      flushSync(measure);
    };
    const mutationObserver = new MutationObserver((records) => {
      if (records.every((record) => record.target instanceof Element && record.target.closest('[data-calculator-personality]'))) return;
      observeRegions();
      remeasure();
    });
    observeRegions();
    mutationObserver.observe(document.body, { subtree: true, childList: true, attributes: true,
      attributeFilter: ['class', 'style', 'open', 'hidden', 'aria-expanded', 'data-speech-protected', 'data-tool-open'] });
    window.addEventListener('resize', remeasure);
    window.addEventListener('scroll', remeasure, true);
    document.addEventListener('animationstart', remeasure, true);
    document.addEventListener('transitionrun', remeasure, true);
    window.visualViewport?.addEventListener('resize', remeasure);
    window.visualViewport?.addEventListener('scroll', remeasure);
    measureBeforePaint.current = measure;
    return () => {
      disposed = true;
      cancelAnimationFrame(frame);
      resizeObserver.disconnect();
      mutationObserver.disconnect();
      measureBeforePaint.current = null;
      window.removeEventListener('resize', remeasure);
      window.removeEventListener('scroll', remeasure, true);
      document.removeEventListener('animationstart', remeasure, true);
      document.removeEventListener('transitionrun', remeasure, true);
      window.visualViewport?.removeEventListener('resize', remeasure);
      window.visualViewport?.removeEventListener('scroll', remeasure);
    };
  }, [allowed, anchorRef, speech?.id]);

  // Parent tool/layout changes commit their DOM before layout effects run.
  // Re-evaluate on every such commit, without introducing a second layout prop.
  useLayoutEffect(() => {
    measureBeforePaint.current?.();
  });

  useEffect(() => {
    if (!speech || speech.startedAt === null) return;
    const duration = speech.phase === 'leaving' ? minimal ? 120 : 240 : Math.max(0, Math.min(speech.startedAt + DISPLAY_MS, speech.expiresAt) - Date.now());
    const timeout = window.setTimeout(() => {
      setSpeech((current) => {
        if (!current || current.id !== speech.id) return current;
        return speech.phase === 'leaving' ? null : { ...current, phase: 'leaving' };
      });
    }, duration);
    return () => window.clearTimeout(timeout);
  }, [speech?.id, speech?.startedAt, speech?.expiresAt, speech?.phase, minimal]);

  if (!allowed) return null;
  const mood = loading ? 'loading' : reaction?.id === calculation?.id ? reaction?.mood ?? 'idle' : 'idle';
  const placement = speech?.placement;
  const bubbleStyle: CSSProperties = placement ? { left: placement.left, top: placement.top } : { left: -10_000, top: 0 };
  const tailStyle: CSSProperties | undefined = placement ? {
    left: placement.tail.left, top: placement.tail.top,
    width: placement.tail.right - placement.tail.left, height: placement.tail.bottom - placement.tail.top,
  } : undefined;
  return createPortal(
    <div className="calculator-personality" data-calculator-personality="" data-minimal={minimal}>
      {face && <div className="calculator-personality__face" style={{ left: face.left, top: face.top }} aria-hidden="true">
        <PixelFace mood={mood} reaction={loading ? 'loading' : reaction?.id ?? 'idle'} />
      </div>}
      {speech && <>
        {placement && <span className="calculator-personality__tail" data-edge={placement.edge} data-phase={speech.phase} style={tailStyle} aria-hidden="true" />}
        <div ref={bubbleRef} className="calculator-speech calculator-personality__bubble" data-edge={placement?.edge ?? 'top'} data-phase={speech.phase} style={bubbleStyle}
          aria-label="Реакция калькулятора" aria-hidden={speech.phase === 'measuring' ? true : undefined}>
          <p>{speech.text}</p>
        </div>
      </>}
    </div>, document.body,
  );
}
