import { useEffect, useRef, useState } from 'react';
import type { PersonalStatistics } from '../../contracts';
import './statistics.css';

type StatisticsPanelProps = {
  load: () => Promise<PersonalStatistics>;
  refreshKey: number;
};

type LoadState =
  | { kind: 'idle' }
  | { kind: 'loading'; key: number }
  | { kind: 'error'; key: number }
  | { kind: 'ready'; key: number; statistics: PersonalStatistics };

const numberFormatter = new Intl.NumberFormat('ru-RU');

function Usage({ counts }: { counts: Record<string, number> }) {
  const entries = Object.entries(counts).sort(
    ([leftName, leftCount], [rightName, rightCount]) =>
      rightCount - leftCount || leftName.localeCompare(rightName, 'ru'),
  );

  if (entries.length === 0) {
    return <p className="statistics-panel__missing">Нет данных об использовании.</p>;
  }

  return (
    <dl className="statistics-panel__usage-list">
      {entries.map(([name, count]) => (
        <div className="statistics-panel__row" key={name}>
          <dt><code>{name}</code></dt>
          <dd>{numberFormatter.format(count)}</dd>
        </div>
      ))}
    </dl>
  );
}

function StatisticsContent({ statistics }: { statistics: PersonalStatistics }) {
  const longest = statistics.longestExpression;

  return (
    <>
      <dl className="statistics-panel__totals">
        <div className="statistics-panel__row">
          <dt>Всего вычислений</dt>
          <dd>{numberFormatter.format(statistics.totalCalculations)}</dd>
        </div>
        <div className="statistics-panel__row">
          <dt>Успешные</dt>
          <dd>{numberFormatter.format(statistics.successes)}</dd>
        </div>
        <div className="statistics-panel__row">
          <dt>Математические ошибки</dt>
          <dd>{numberFormatter.format(statistics.mathematicalErrors)}</dd>
        </div>
      </dl>

      {statistics.totalCalculations === 0 ? (
        <p className="statistics-panel__empty">
          Вычислений пока нет. Сводка заполнится после первого результата.
        </p>
      ) : (
        <details className="statistics-panel__details">
          <summary>Подробности</summary>
          <dl>
            <div className="statistics-panel__row">
              <dt>Попытки деления на ноль</dt>
              <dd>{numberFormatter.format(statistics.divisionByZeroAttempts)}</dd>
            </div>
          </dl>
          <p className="statistics-panel__note">
            Попытки деления на ноль входят в число ошибок.
          </p>

          <div className="statistics-panel__usage">
            <h3>Операции</h3>
            <Usage counts={statistics.operators} />
            <h3>Функции</h3>
            <Usage counts={statistics.functions} />
          </div>

          <div className="statistics-panel__extremes">
            <h3>Самое длинное выражение</h3>
            {longest === null ? (
              <p className="statistics-panel__missing">Нет сохранённого выражения.</p>
            ) : (
              <>
                <code className="statistics-panel__source" dir="auto" tabIndex={0}>
                  {longest.expression}
                </code>
                <p className="statistics-panel__length">
                  Длина в кодовых единицах UTF‑16: {numberFormatter.format(longest.length)}
                </p>
              </>
            )}
            <dl className="statistics-panel__depth">
              <div className="statistics-panel__row">
                <dt>Максимальная глубина разбора</dt>
                <dd>
                  {statistics.maxParsedDepth === null
                    ? 'Нет данных'
                    : numberFormatter.format(statistics.maxParsedDepth)}
                </dd>
              </div>
            </dl>
          </div>
        </details>
      )}
    </>
  );
}

export function StatisticsPanel({ load, refreshKey }: StatisticsPanelProps) {
  const [state, setState] = useState<LoadState>({ kind: 'idle' });
  const [retry, setRetry] = useState(0);
  const latestRequest = useRef(0);
  const loadedKey = useRef<number | null>(null);

  useEffect(() => {
    if (loadedKey.current === refreshKey) return;

    const request = ++latestRequest.current;
    setState({ kind: 'loading', key: refreshKey });
    void (async () => {
      try {
        const statistics = await load();
        if (request !== latestRequest.current) return;
        loadedKey.current = refreshKey;
        setState({ kind: 'ready', key: refreshKey, statistics });
      } catch {
        if (request === latestRequest.current) setState({ kind: 'error', key: refreshKey });
      }
    })();

    return () => {
      if (request === latestRequest.current) latestRequest.current++;
    };
  }, [refreshKey, load, retry]);

  const visibleState = state.kind !== 'idle' && state.key === refreshKey ? state : null;

  return (
    <section className="statistics-panel" aria-label="Личная статистика">
      <div className="statistics-panel__body">
        {visibleState?.kind === 'ready' ? (
          <StatisticsContent statistics={visibleState.statistics} />
        ) : visibleState?.kind === 'error' ? (
          <div className="statistics-panel__failure">
            <p role="alert">Не удалось загрузить статистику.</p>
            <button type="button" onClick={() => setRetry((previous) => previous + 1)}>
              Повторить
            </button>
          </div>
        ) : (
          <p className="statistics-panel__loading" role="status">
            Загружаем статистику…
          </p>
        )}
      </div>
    </section>
  );
}
