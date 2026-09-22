import { useCallback, useMemo, useState } from 'react';
import type { AngleUnit, CalculationRecord } from '../../contracts';
import { getMessages, type Language } from '../../i18n';
import { History } from './History';
import { useHistoryHandlers } from './useHistoryHandlers';
import { fetchEmpty, fetchError, fetchNormal } from './mockApi';
import type { FetchPage } from './mockApi';

type Scenario = 'normal' | 'empty' | 'error';

const scenarioOptions: ReadonlyArray<{ value: Scenario; label: string }> = [
  { value: 'normal', label: 'Обычная' },
  { value: 'empty', label: 'Пустая' },
  { value: 'error', label: 'Ошибка чтения' },
];

// Демонстрационные «недоступные» записи: id известны заранее.
const UNSUPPORTED_IDS = new Set(['b1']);

export function HistoryDemo() {
  const [language, setLanguage] = useState<Language>('ru');
  const [scenario, setScenario] = useState<Scenario>('normal');
  const [restored, setRestored] = useState<{
    expression: string;
    angleUnit: AngleUnit;
  } | null>(null);

  const messages = getMessages(language);

  const fetchPage: FetchPage = useMemo(() => {
    if (scenario === 'empty') return fetchEmpty;
    if (scenario === 'error') return fetchError;
    return fetchNormal;
  }, [scenario]);

  const handleRestore = useCallback(
    (expression: string, angleUnit: AngleUnit) => {
      setRestored({ expression, angleUnit });
    },
    [],
  );

  const { items, nextCursor, loading, hasReadError, handleLoadMore, handleSelect } =
    useHistoryHandlers(fetchPage, handleRestore);

  const isUnsupported = useCallback(
    (record: CalculationRecord) => UNSUPPORTED_IDS.has(record.id),
    [],
  );

  const t = messages.history;

  return (
    <section className="history-demo" aria-labelledby="history-demo-heading">
      <div className="section-heading">
        <h2 id="history-demo-heading">Демонстрация истории</h2>
      </div>

      <div className="history-demo-controls">
        <label>
          Язык / Language:{' '}
          <select
            value={language}
            onChange={(e) => setLanguage(e.target.value as Language)}
          >
            <option value="ru">Русский</option>
            <option value="en">English</option>
          </select>
        </label>

        <label>
          Сценарий:{' '}
          <select
            value={scenario}
            onChange={(e) => setScenario(e.target.value as Scenario)}
          >
            {scenarioOptions.map((opt) => (
              <option key={opt.value} value={opt.value}>
                {opt.label}
              </option>
            ))}
          </select>
        </label>
      </div>

      <History
        items={items}
        nextCursor={nextCursor}
        loading={loading}
        hasReadError={hasReadError}
        language={language}
        messages={messages}
        onLoadMore={handleLoadMore}
        onSelect={handleSelect}
        isUnsupported={isUnsupported}
      />

      {restored !== null && (
        <p className="history-restored" role="status" aria-live="polite">
          {t.restoredLabel}: <code>{restored.expression}</code> ·{' '}
          {t.angleUnitLabel[restored.angleUnit]}
        </p>
      )}
    </section>
  );
}