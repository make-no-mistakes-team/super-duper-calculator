import { useCallback, useMemo, useState } from 'react';
import type { AngleUnit, CalculationRecord } from '../../contracts';
import { getMessages, type Language } from '../../i18n';
import { History } from './History';
import { useHistoryHandlers } from './useHistoryHandlers';
import { fetchEmpty, fetchError, fetchNormal } from './mockApi';
import type { FetchPage } from './mockApi';

type Scenario = 'normal' | 'empty' | 'error';

const scenarioOptions: ReadonlyArray<Scenario> = ['normal', 'empty', 'error'];

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
        <h2 id="history-demo-heading">{t.demo.title}</h2>
      </div>

      <div className="history-demo-controls">
        <label>
          {t.demo.languageLabel}:{' '}
          <select
            value={language}
            onChange={(e) => setLanguage(e.target.value as Language)}
          >
            <option value="ru">Русский</option>
            <option value="en">English</option>
          </select>
        </label>

        <label>
          {t.demo.scenarioLabel}:{' '}
          <select
            value={scenario}
            onChange={(e) => setScenario(e.target.value as Scenario)}
          >
            {scenarioOptions.map((option) => (
              <option key={option} value={option}>
                {t.demo.scenarios[option]}
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