import { useEffect, useLayoutEffect, useRef } from 'react';
import type { Achievement, DiscoveryDefinition } from '../../contracts';
import { achievementAccent, achievementArtwork } from './achievementPresentation';
import { AchievementDescription } from './AchievementDescription';
import './achievement-collection.css';

export type AchievementCollectionProps = {
  catalog: DiscoveryDefinition[];
  achievements: Achievement[];
  available: boolean;
  loading: boolean;
  onRetry: () => void;
  open: boolean;
  onToggle: (open: boolean) => void;
  selectedId?: string | null;
  freshAwardIds?: readonly string[];
};

const earnedDate = new Intl.DateTimeFormat('ru-RU', {
  day: 'numeric', month: 'long', year: 'numeric', hour: '2-digit', minute: '2-digit',
});

function dateLabel(value: string) {
  const date = new Date(value);
  return Number.isFinite(date.getTime()) ? earnedDate.format(date) : null;
}

export function AchievementCollection({
  catalog, achievements, available, loading, onRetry, open, onToggle, selectedId, freshAwardIds,
}: AchievementCollectionProps) {
  const dialogRef = useRef<HTMLDialogElement>(null);
  const closeRef = useRef<HTMLButtonElement>(null);
  const bodyRef = useRef<HTMLDivElement>(null);
  const openerRef = useRef<HTMLElement | null>(null);
  const cardsRef = useRef(new Map<string, HTMLLIElement>());
  const earnedById = new Map(achievements.map((award) => [award.id, award.earnedAt]));
  const earnedCount = catalog.filter((definition) => earnedById.has(definition.id)).length;
  const orderedCatalog = [
    ...catalog.filter((item) => earnedById.has(item.id)),
    ...catalog.filter((item) => !earnedById.has(item.id) && !item.secret),
    ...catalog.filter((item) => !earnedById.has(item.id) && item.secret),
  ];

  const restoreOpener = () => {
    const opener = openerRef.current;
    const target = opener?.isConnected && !opener.matches(':disabled')
      ? opener : document.getElementById('expression');
    target?.focus({ preventScroll: true });
  };

  // Synchronize with the native top layer, without closing it during StrictMode's
  // effect replay. Only a real controlled close changes focus.
  useLayoutEffect(() => {
    const dialog = dialogRef.current;
    if (!dialog) return;
    if (open && !dialog.open) {
      const active = document.activeElement;
      if (active instanceof HTMLElement && !dialog.contains(active) && active !== document.body) {
        openerRef.current = active;
      } else {
        openerRef.current = document.getElementById('expression');
      }
      dialog.showModal();
      closeRef.current?.focus({ preventScroll: true });
    } else if (!open && dialog.open) {
      dialog.close();
      restoreOpener();
    }
  }, [open]);

  useLayoutEffect(() => {
    if (!open || !selectedId) return;
    cardsRef.current.get(selectedId)?.scrollIntoView({ block: 'center', behavior: 'instant' });
  }, [open, selectedId, catalog, achievements, available]);

  useEffect(() => {
    const dialog = dialogRef.current;
    return () => {
      // A StrictMode cleanup still has a connected dialog. A real removal does
      // not; defer until React has committed the removal before restoring focus.
      queueMicrotask(() => {
        if (dialog && !dialog.isConnected && dialog.open) restoreOpener();
      });
    };
  }, []);

  const close = () => {
    dialogRef.current?.close();
    restoreOpener();
    onToggle(false);
  };

  return (
    <dialog
      ref={dialogRef}
      id="achievement-collection"
      className="achievement-collection"
      aria-labelledby="achievement-collection-title"
      onCancel={(event) => { event.preventDefault(); event.stopPropagation(); close(); }}
      onClose={(event) => {
        if (!event.currentTarget.open && open) { restoreOpener(); onToggle(false); }
      }}
      onKeyDown={(event) => {
        if (event.key === 'Escape') event.stopPropagation();
      }}
      onClick={(event) => {
        if (event.target !== event.currentTarget) return;
        const rect = event.currentTarget.getBoundingClientRect();
        if (event.clientX < rect.left || event.clientX > rect.right || event.clientY < rect.top || event.clientY > rect.bottom) close();
      }}
    >
      <header className="achievement-collection__header">
        <div>
          <h2 id="achievement-collection-title">Достижения</h2>
          <p className="achievement-collection__count">
            {catalog.length > 0 ? `Получено ${earnedCount} из ${catalog.length}` : 'Личная коллекция'}
          </p>
        </div>
        <button ref={closeRef} className="achievement-collection__close" type="button" onClick={close} aria-label="Закрыть достижения">
          <svg width="20" height="20" viewBox="0 0 20 20" fill="none" aria-hidden="true"><path d="m4 4 12 12M16 4 4 16" stroke="currentColor" strokeWidth="1.5" /></svg>
        </button>
      </header>
      <div ref={bodyRef} className="achievement-collection__body" aria-busy={loading}>
        {(loading || !available || catalog.length === 0) && (
          <div className="achievement-collection__availability" role="status">
            <p>{loading ? 'Загружаем коллекцию…' : !available ? 'Коллекция сейчас недоступна. Полученные достижения не потеряны.' : 'Каталог достижений не загрузился.'}</p>
            {!loading && <button type="button" onClick={onRetry}>Повторить загрузку</button>}
          </div>
        )}
        {catalog.length > 0 && (
            <ul className="achievement-collection__list">
              {orderedCatalog.map((definition) => {
                const earnedAt = earnedById.get(definition.id);
                const earned = earnedAt !== undefined;
                const date = earned ? dateLabel(earnedAt) : null;
                return (
                  <li
                    key={definition.id}
                    ref={(element) => { if (element) cardsRef.current.set(definition.id, element); else cardsRef.current.delete(definition.id); }}
                    className={`achievement-collection__item${earned ? ' achievement-collection__item--earned' : ''}${selectedId === definition.id ? ' achievement-collection__item--selected' : ''}`}
                    style={achievementAccent(definition.id)}
                    data-achievement-id={definition.id}
                  >
                    <img className="achievement-collection__artwork" src={achievementArtwork(definition.id)} width="128" height="128" alt="" loading="lazy" />
                    <div className="achievement-collection__copy">
                      <h4>{definition.ru.name}</h4>
                      <span className={`achievement-collection__state${earned ? ' achievement-collection__state--earned' : ''}`}>
                        {earned ? 'Получено' : 'Не получено'}
                      </span>
                      <AchievementDescription
                        text={earned || !definition.secret ? definition.ru.description : null}
                        locked={!earned}
                        concealed={!earned && definition.secret}
                        active={open}
                        rootRef={bodyRef}
                        freshAwardIds={freshAwardIds}
                        awardId={definition.id}
                        className={!earned && definition.secret ? 'achievement-collection__concealed' : 'achievement-collection__description'}
                      />
                      {earned && (date ? <time dateTime={earnedAt}>{date}</time> : <span className="achievement-collection__date">Дата получения недоступна</span>)}
                    </div>
                  </li>
                );
              })}
            </ul>
        )}
      </div>
    </dialog>
  );
}
