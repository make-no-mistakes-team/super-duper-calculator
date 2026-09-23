import { useEffect, useRef, useState } from 'react';
import { getReduction } from './api';
import type { CalculationRecord, ReductionResponse } from './contracts';
import type { Messages } from './i18n';

type PlaybackState =
  | { kind: 'idle' }
  | { kind: 'loading' }
  | { kind: 'failed' }
  | { kind: 'ready'; sequence: ReductionResponse; index: number; phase: 'highlight' | 'after' | 'complete' };

type Props = {
  record: CalculationRecord;
  messages: Messages['reduction'];
};

function isReductionResponse(value: unknown, record: CalculationRecord): value is ReductionResponse {
  if (record.outcome.kind !== 'success' || typeof value !== 'object' || value === null) return false;
  const data = value as Partial<ReductionResponse>;
  if (data.calculationId !== record.id || data.initialExpression !== record.expression ||
      data.finalExpression !== record.outcome.value || !Array.isArray(data.steps) || data.steps.length > 256) return false;

  let current = data.initialExpression;
  for (const step of data.steps) {
    if (typeof step?.before !== 'string' || typeof step.after !== 'string' ||
        typeof step.replacement !== 'string' || step.replacement === '' ||
        !step.span || !Number.isInteger(step.span.start) || !Number.isInteger(step.span.end) ||
        step.before !== current || step.span.start < 0 || step.span.end <= step.span.start ||
        step.span.end > step.before.length ||
        step.before.slice(0, step.span.start) + step.replacement + step.before.slice(step.span.end) !== step.after) return false;
    current = step.after;
  }
  return data.steps.length === 0 || current === data.finalExpression;
}

export function ReductionPlayback({ record, messages: t }: Props) {
  const [state, setState] = useState<PlaybackState>({ kind: 'idle' });
  const [visible, setVisible] = useState(() => document.visibilityState === 'visible');
  const [reducedMotion, setReducedMotion] = useState(() => window.matchMedia('(prefers-reduced-motion: reduce)').matches);
  const requestRef = useRef<AbortController | null>(null);
  const startRef = useRef<HTMLButtonElement>(null);

  useEffect(() => {
    const motion = window.matchMedia('(prefers-reduced-motion: reduce)');
    const onMotion = () => setReducedMotion(motion.matches);
    const onVisibility = () => setVisible(document.visibilityState === 'visible');
    motion.addEventListener('change', onMotion);
    document.addEventListener('visibilitychange', onVisibility);
    return () => {
      motion.removeEventListener('change', onMotion);
      document.removeEventListener('visibilitychange', onVisibility);
      requestRef.current?.abort();
    };
  }, []);

  useEffect(() => {
    if (state.kind !== 'ready' || state.phase === 'complete' || reducedMotion || !visible) return;
    const timer = window.setTimeout(() => {
      setState((current) => {
        if (current.kind !== 'ready' || current.sequence !== state.sequence || current.index !== state.index || current.phase !== state.phase) return current;
        if (current.phase === 'highlight') return { ...current, phase: 'after' };
        return current.index + 1 < current.sequence.steps.length
          ? { ...current, index: current.index + 1, phase: 'highlight' }
          : { ...current, phase: 'complete' };
      });
    }, state.phase === 'highlight' ? 1050 : 700);
    return () => window.clearTimeout(timer);
  }, [state, reducedMotion, visible]);

  async function start() {
    requestRef.current?.abort();
    const controller = new AbortController();
    requestRef.current = controller;
    setState({ kind: 'loading' });
    try {
      const response = await getReduction(record.id, controller.signal);
      if (controller.signal.aborted) return;
      if (!isReductionResponse(response, record)) {
        setState({ kind: 'failed' });
        return;
      }
      setState({ kind: 'ready', sequence: response, index: 0,
        phase: response.steps.length === 0 ? 'complete' : 'highlight' });
    } catch {
      if (!controller.signal.aborted) setState({ kind: 'failed' });
    } finally {
      if (requestRef.current === controller) requestRef.current = null;
    }
  }

  function close() {
    requestRef.current?.abort();
    requestRef.current = null;
    setState({ kind: 'idle' });
    requestAnimationFrame(() => startRef.current?.focus());
  }

  const ready = state.kind === 'ready' ? state : null;
  const step = ready?.phase === 'complete' ? null : ready?.sequence.steps[ready.index];

  return (
    <section className="playback" aria-labelledby="playback-heading">
      <h2 id="playback-heading">{t.heading}</h2>
      <p className="playback-source">{t.record}: <code>{record.expression}</code></p>

      {(state.kind === 'idle' || state.kind === 'failed') && (
        <button ref={startRef} type="button" className="playback-action" onClick={() => void start()}>{t.action}</button>
      )}
      {state.kind === 'loading' && <p role="status" className="playback-note">{t.loading}</p>}
      {state.kind === 'failed' && <p role="alert" className="playback-note playback-note--error">{t.unavailable}</p>}

      {ready && (
        <div className="playback-stage">
          {step ? (
            <>
              <p className="playback-count">{t.step} {ready.index + 1} / {ready.sequence.steps.length}</p>
              <code className={`playback-expression${ready.phase === 'after' && !reducedMotion ? ' playback-expression--after' : ''}`}>
                {ready.phase === 'after' && !reducedMotion ? step.after : <>
                  {step.before.slice(0, step.span.start)}
                  <mark>{step.before.slice(step.span.start, step.span.end)}</mark>
                  {step.before.slice(step.span.end)}
                </>}
              </code>
              {(ready.phase === 'highlight' || reducedMotion) && (
                <p className="playback-replacement">→ <code>{step.replacement}</code></p>
              )}
              {reducedMotion && <code className="playback-expression playback-expression--static">{step.after}</code>}
            </>
          ) : (
            <>
              {ready.sequence.steps.length === 0 && <p className="playback-note">{t.noSteps}</p>}
              <p className="playback-count">{t.final}</p>
              <code className="playback-expression playback-expression--final">{ready.sequence.finalExpression}</code>
            </>
          )}
        </div>
      )}

      {state.kind !== 'idle' && (
        <div className="playback-controls">
          {ready && ready.phase !== 'complete' && (
            <>
              {reducedMotion && <button type="button" onClick={() => setState((current) => current.kind === 'ready'
                ? { ...current, index: Math.min(current.index + 1, current.sequence.steps.length - 1),
                  phase: current.index + 1 < current.sequence.steps.length ? 'highlight' : 'complete' }
                : current)}>{t.next}</button>}
              <button type="button" onClick={() => setState({ ...ready, phase: 'complete' })}>{t.skip}</button>
            </>
          )}
          {ready?.phase === 'complete' && ready.sequence.steps.length > 0 && <button type="button" onClick={() => setState({ ...ready, index: 0,
            phase: ready.sequence.steps.length === 0 ? 'complete' : 'highlight' })}>{t.replay}</button>}
          <button type="button" onClick={close}>{t.close}</button>
        </div>
      )}
    </section>
  );
}
