import { useCallback, useEffect, useLayoutEffect, useRef, useState } from 'react';
import type { FunEvent } from '../../contracts';
import './comic-incident.css';

type ComicIncidentProps = {
  events: FunEvent[];
  enabled: boolean;
  onActiveChange: (active: boolean) => void;
};

type Scene = {
  id: string;
  deadline: number;
};

const FRESH_MS = 3_000;
const SCENE_MS = 3_000;
const SEEN_MS = 10_000;

// Keep consumed server IDs across a component remount. No event is queued for later display.
const seenUntil = new Map<string, number>();

export function ComicIncident({ events, enabled, onActiveChange }: ComicIncidentProps) {
  const [scene, setScene] = useState<Scene | null>(null);
  const currentScene = useRef<Scene | null>(null);
  const closeButton = useRef<HTMLButtonElement | null>(null);
  const previousControl = useRef<HTMLElement | null>(null);
  const lastResumedAt = useRef(document.hidden ? Number.POSITIVE_INFINITY : Number.NEGATIVE_INFINITY);

  const restoreFocusIfNeeded = useCallback(() => {
    if (closeButton.current === null || document.activeElement !== closeButton.current) return;
    const previous = previousControl.current;
    const fallback = document.getElementById('expression');
    const target = previous?.isConnected && !previous.matches(':disabled')
      ? previous
      : fallback instanceof HTMLElement ? fallback : null;
    target?.focus({ preventScroll: true });
  }, []);

  const dismiss = useCallback(() => {
    if (currentScene.current === null) return;
    restoreFocusIfNeeded();
    currentScene.current = null;
    setScene(null);
  }, [restoreFocusIfNeeded]);

  // A focused close button may also be removed by a parent unmount or preference change.
  const setCloseButton = useCallback((button: HTMLButtonElement | null) => {
    if (button === null) restoreFocusIfNeeded();
    closeButton.current = button;
  }, [restoreFocusIfNeeded]);

  useEffect(() => {
    const onVisibilityChange = () => {
      if (document.hidden) {
        dismiss();
      } else {
        // Events received in the background cannot play when the tab returns.
        lastResumedAt.current = Date.now();
      }
    };
    document.addEventListener('visibilitychange', onVisibilityChange);
    return () => document.removeEventListener('visibilitychange', onVisibilityChange);
  }, [dismiss]);

  useEffect(() => {
    const now = Date.now();
    for (const [id, until] of seenUntil) {
      if (until <= now) seenUntil.delete(id);
    }

    let candidate: { id: string; createdAt: number; expiresAt: number } | null = null;
    for (const event of events) {
      if (
        event.ruleId !== 'comic_incident' ||
        event.kind !== 'scene' ||
        event.scope !== 'personal' ||
        !event.id ||
        seenUntil.has(event.id)
      ) continue;

      const createdAt = Date.parse(event.createdAt);
      const expiresAt = Date.parse(event.expiresAt);
      seenUntil.set(
        event.id,
        Number.isFinite(expiresAt) && expiresAt > now
          ? Math.max(now + FRESH_MS, Math.min(expiresAt, now + SEEN_MS))
          : now + FRESH_MS,
      );

      if (
        !enabled ||
        document.hidden ||
        !Number.isFinite(createdAt) ||
        !Number.isFinite(expiresAt) ||
        createdAt > now ||
        now - createdAt >= FRESH_MS ||
        createdAt <= lastResumedAt.current ||
        expiresAt <= now
      ) continue;

      // Consume the entire burst, showing only its latest eligible event.
      if (candidate === null || createdAt > candidate.createdAt) {
        candidate = { id: event.id, createdAt, expiresAt };
      }
    }

    if (candidate !== null && currentScene.current === null) {
      const activeElement = document.activeElement;
      const fallback = document.getElementById('expression');
      previousControl.current = activeElement instanceof HTMLElement &&
        activeElement !== document.body && activeElement.tabIndex >= 0
        ? activeElement
        : fallback instanceof HTMLElement ? fallback : null;

      const next = {
        id: candidate.id,
        deadline: performance.now() + Math.min(SCENE_MS, candidate.expiresAt - Date.now()),
      };
      if (next.deadline > performance.now()) {
        currentScene.current = next;
        setScene(next);
      }
    }
  }, [events, enabled]);

  useEffect(() => {
    if (scene === null) return;
    const remaining = scene.deadline - performance.now();
    if (!enabled || document.hidden || remaining <= 0) {
      dismiss();
      return;
    }

    const onKeyDown = (event: KeyboardEvent) => {
      if (event.key !== 'Escape' || event.isComposing) return;
      event.preventDefault();
      event.stopPropagation();
      dismiss();
    };
    const timeout = window.setTimeout(dismiss, remaining);
    document.addEventListener('keydown', onKeyDown, true);
    return () => {
      window.clearTimeout(timeout);
      document.removeEventListener('keydown', onKeyDown, true);
    };
  }, [scene, enabled, dismiss]);

  const active = enabled && scene !== null && !document.hidden && scene.deadline > performance.now();
  useLayoutEffect(() => {
    onActiveChange(active);
    return () => onActiveChange(false);
  }, [active, onActiveChange]);

  if (!active) return null;

  return (
    <aside className="comic-incident" aria-label="Шуточная сцена калькулятора">
      <div className="comic-incident__copy">
        <div className="comic-incident__heading">
          <span className="comic-incident__count" aria-hidden="true">÷0 × 3</span>
          <h2>Мнимый сбой</h2>
        </div>
        <p className="comic-incident__setup">Три деления на ноль. Калькулятор берёт театральную паузу.</p>
        <p className="comic-incident__reveal"><strong>Шутка.</strong> Он работает; настоящая ошибка показана в результате.</p>
      </div>
      <button
        ref={setCloseButton}
        className="comic-incident__close"
        type="button"
        aria-label="Закрыть шуточную сцену"
        onClick={dismiss}
      >
        <svg width="18" height="18" viewBox="0 0 18 18" fill="none" aria-hidden="true">
          <path d="m4 4 10 10M14 4 4 14" stroke="currentColor" strokeWidth="1.5" />
        </svg>
      </button>
      <span className="visually-hidden" role="status" aria-live="polite" aria-atomic="true">
        Шуточный мнимый сбой. Калькулятор работает. Настоящая ошибка показана в результате.
      </span>
    </aside>
  );
}
