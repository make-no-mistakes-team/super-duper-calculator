import { useMemo } from 'react';
import type { CalculationRecord } from '../../contracts';
import type { Language, Messages } from '../../i18n';

export type HistoryProps = {
  items: CalculationRecord[];
  nextCursor: string | null;
  loading: boolean;
  hasReadError: boolean;
  language: Language;
  messages: Messages;
  onLoadMore: () => void;
  onSelect: (record: CalculationRecord) => void;
  /** Записи, использующие недоступное расширение (например, факториал). */
  isUnsupported?: (record: CalculationRecord) => boolean;
};

export function History({
  items,
  nextCursor,
  loading,
  hasReadError,
  language,
  messages,
  onLoadMore,
  onSelect,
  isUnsupported = () => false,
}: HistoryProps) {
  const t = messages.history;
  const dateFormatter = useMemo(
    () =>
      new Intl.DateTimeFormat(language === 'ru' ? 'ru-RU' : 'en-US', {
        dateStyle: 'short',
        timeStyle: 'short',
      }),
    [language],
  );

  const isEmpty = items.length === 0 && !loading && !hasReadError;

  return (
    <section className="history" aria-labelledby="history-heading">
      <div className="section-heading">
        <h2 id="history-heading">{t.title}</h2>
      </div>

      {isEmpty && <p className="history-empty">{t.empty}</p>}

      {items.length > 0 && (
        <ol className="history-list">
          {items.map((item) => {
            const unsupported = isUnsupported(item);
            const outcomeText =
              item.outcome.kind === 'success'
                ? item.outcome.value
                : (t.mathErrors[item.outcome.error.code] ?? t.mathErrorUnknown);
            const outcomeLabel =
              item.outcome.kind === 'success'
                ? t.outcomeLabel.success
                : t.outcomeLabel.error;
            const timeText = dateFormatter.format(new Date(item.createdAt));
            const angleText = t.angleUnitLabel[item.context.angleUnit];
            const accessibleLabel = `${item.expression}. ${outcomeLabel}: ${outcomeText}. ${angleText}. ${timeText}`;

            return (
              <li key={item.id} className="history-item">
                <button
                  type="button"
                  className="history-item-button"
                  onClick={() => onSelect(item)}
                  aria-label={accessibleLabel}
                >
                  <code className="history-expression">{item.expression}</code>
                  <span className="history-meta">
                    <span
                      className={
                        item.outcome.kind === 'success'
                          ? 'history-outcome history-outcome--success'
                          : 'history-outcome history-outcome--error'
                      }
                    >
                      {item.outcome.kind === 'success' ? '= ' : ''}
                      {outcomeText}
                    </span>
                    <span className="history-angle" aria-hidden="true">
                      {item.context.angleUnit}
                    </span>
                    <time dateTime={item.createdAt} className="history-time">
                      {timeText}
                    </time>
                  </span>
                  {unsupported && (
                    <span className="history-unsupported">{t.unsupported}</span>
                  )}
                </button>
              </li>
            );
          })}
        </ol>
      )}

      {loading && (
        <p className="history-loading" role="status" aria-live="polite">
          {t.loading}
        </p>
      )}

      {hasReadError && (
        <p className="history-read-error" role="alert">
          {t.readError}
        </p>
      )}

      {nextCursor !== null && !loading && (
        <button
          type="button"
          className="history-load-more"
          onClick={onLoadMore}
        >
          {t.loadMore}
        </button>
      )}
    </section>
  );
}