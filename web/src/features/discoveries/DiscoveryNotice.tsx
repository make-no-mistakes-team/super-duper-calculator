import { useCallback, useEffect, useRef, useState } from 'react';
import type { DiscoveryDefinition, FunEvent } from '../../contracts';
import './discoveries.css';

export type DiscoveryNoticeProps = {
  events: FunEvent[];
  catalog: DiscoveryDefinition[];
  humorEnabled: boolean;
  onOpenCollection: () => void;
};

type Notice = {
  ruleId: string;
  comment: string;
  createdAt: number;
  expiresAt: number;
};

const COOLDOWN_MS = 15_000;
const NOTICE_MS = 5_000;
const touchGrassComments: Record<number, string> = {
  50: '50 вычислений. Клавиатуре тоже нужен перерыв.',
  100: '100 вычислений. Калькулятор впечатлён. Улица всё ещё существует.',
};

// This page-view state survives a component remount (for example, closing the history sheet).
// IDs are retained only while their events could still be fresh.
const presentation = {
  consumedUntil: new Map<string, number>(),
  lastShownAt: Number.NEGATIVE_INFINITY,
};

export function DiscoveryNotice({
  events,
  catalog,
  humorEnabled,
  onOpenCollection,
}: DiscoveryNoticeProps) {
  const [notice, setNotice] = useState<Notice | null>(null);
  const noticeRef = useRef<HTMLElement>(null);
  const lastResumedAt = useRef(document.hidden ? Number.POSITIVE_INFINITY : Number.NEGATIVE_INFINITY);
  const dismiss = useCallback((restoreFocus = true) => {
    if (restoreFocus && noticeRef.current?.contains(document.activeElement)) {
      document.getElementById('expression')?.focus({ preventScroll: true });
    }
    setNotice(null);
  }, []);
  const setNoticeElement = useCallback((element: HTMLElement | null) => {
    if (element === null && noticeRef.current?.contains(document.activeElement)) {
      document.getElementById('expression')?.focus({ preventScroll: true });
    }
    noticeRef.current = element;
  }, []);

  useEffect(() => {
    const onVisibilityChange = () => {
      if (document.hidden) {
        dismiss();
      } else {
        // Anything delivered before the tab returned is old, even if its expiry is later.
        lastResumedAt.current = Date.now();
      }
    };
    document.addEventListener('visibilitychange', onVisibilityChange);
    return () => document.removeEventListener('visibilitychange', onVisibilityChange);
  }, [dismiss]);

  useEffect(() => {
    const now = Date.now();
    for (const [id, until] of presentation.consumedUntil) {
      if (until <= now) presentation.consumedUntil.delete(id);
    }

    const definitions = new Map(catalog.map((definition) => [definition.id, definition]));
    let candidate: Notice | null = null;

    for (const event of events) {
      if (presentation.consumedUntil.has(event.id)) continue;
      presentation.consumedUntil.set(event.id, now + COOLDOWN_MS);

      const createdAt = Date.parse(event.createdAt);
      const expiresAt = Date.parse(event.expiresAt);
      const definition = definitions.get(event.ruleId);
      if (
        !humorEnabled ||
        document.hidden ||
        event.kind !== 'comment' ||
        event.scope !== 'personal' ||
        !definition ||
        !definition.ru.comment ||
        !Number.isFinite(createdAt) ||
        !Number.isFinite(expiresAt) ||
        createdAt > now ||
        createdAt <= lastResumedAt.current ||
        now - createdAt >= COOLDOWN_MS ||
        expiresAt <= now
      ) continue;

      // Keep only the newest eligible reaction in a burst. Earlier ones are consumed,
      // not scheduled to interrupt the next calculation.
      if (candidate === null || createdAt >= candidate.createdAt) {
        candidate = {
          ruleId: event.ruleId,
          comment: event.ruleId === 'touch_grass' && typeof event.params.count === 'number'
            ? touchGrassComments[event.params.count] ?? definition.ru.comment
            : definition.ru.comment,
          createdAt,
          expiresAt,
        };
      }
    }

    if (candidate && notice === null && now - presentation.lastShownAt >= COOLDOWN_MS) {
      presentation.lastShownAt = now;
      setNotice(candidate);
    }
    if (!humorEnabled && notice !== null) dismiss();
  }, [events, catalog, humorEnabled, notice, dismiss]);

  useEffect(() => {
    if (notice === null || !humorEnabled) return;
    const remaining = Math.min(NOTICE_MS, notice.expiresAt - Date.now());
    if (remaining <= 0) {
      dismiss();
      return;
    }
    const timeout = window.setTimeout(() => dismiss(), remaining);
    return () => window.clearTimeout(timeout);
  }, [notice, humorEnabled, dismiss]);

  return (
    <>
      <span className="visually-hidden discovery-notice__announcement" role="status" aria-live="polite" aria-atomic="true">
        {humorEnabled ? notice?.comment : ''}
      </span>
      {humorEnabled && notice !== null && (
        <aside
          ref={setNoticeElement}
          className={`discovery-notice${notice.ruleId === 'six_seven' ? ' discovery-notice--six-seven' : ''}`}
          aria-label="Реакция калькулятора"
        >
          {notice.ruleId === 'six_seven' && <span className="discovery-notice__pixel" aria-hidden="true" />}
          <p className="discovery-notice__comment">{notice.comment}</p>
          <button
            className="discovery-notice__collection"
            type="button"
            onClick={() => {
              dismiss(false);
              onOpenCollection();
            }}
          >
            Коллекция
          </button>
          <button
            className="discovery-notice__close"
            type="button"
            aria-label="Закрыть реакцию"
            onClick={() => dismiss()}
          >
            <svg width="16" height="16" viewBox="0 0 16 16" fill="none" aria-hidden="true">
              <path d="m3 3 10 10M13 3 3 13" stroke="currentColor" strokeWidth="1.5" />
            </svg>
          </button>
        </aside>
      )}
    </>
  );
}
