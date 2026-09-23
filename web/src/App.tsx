import { useCallback, useEffect, useRef, useState } from 'react';
import { CalculatorInput } from './CalculatorInput';
import type { AngleUnit, CalculationRecord, CalculationRequest, CalculationResponse, HistoryPage } from './contracts';
import { History } from './features/history/History';
import { getMessages } from './i18n';

type ResultState =
  | { kind: 'idle' }
  | { kind: 'loading'; expression: string }
  | { kind: 'record'; record: CalculationRecord }
  | { kind: 'failed'; message: string; request: CalculationRequest };

const messages = getMessages('ru');

function displayValue(value: string) {
  const number = Number(value);
  const text = String(Number(number.toPrecision(12)));
  return { text, approximate: Number(text) !== number };
}

function requestError(status: number): string {
  if (status === 413) return 'Выражение слишком длинное.';
  if (status === 400) return 'Некорректный запрос. Проверьте выражение и настройки.';
  if (status === 409) return 'Действие уже использовано для другого выражения.';
  return 'Не удалось сохранить вычисление. Проверьте соединение и повторите попытку.';
}

export default function App() {
  const [expression, setExpression] = useState('');
  const [angleUnit, setAngleUnit] = useState<AngleUnit>('deg');
  const [result, setResult] = useState<ResultState>({ kind: 'idle' });
  const [items, setItems] = useState<CalculationRecord[]>([]);
  const [nextCursor, setNextCursor] = useState<string | null>(null);
  const [historyLoading, setHistoryLoading] = useState(true);
  const [historyError, setHistoryError] = useState(false);
  const submissionSequence = useRef(0);
  const historySequence = useRef(0);
  const sessionPromise = useRef<Promise<void> | null>(null);

  const ensureSession = useCallback(() => {
    if (sessionPromise.current === null) {
      sessionPromise.current = fetch('/api/session', { cache: 'no-store' })
        .then((response) => {
          if (!response.ok) throw new Error('Session request failed');
        })
        .catch((error: unknown) => {
          sessionPromise.current = null;
          throw error;
        });
    }
    return sessionPromise.current;
  }, []);

  const loadHistory = useCallback(async (cursor: string | null) => {
    const sequence = ++historySequence.current;
    setHistoryLoading(true);
    setHistoryError(false);
    try {
      await ensureSession();
      const query = cursor === null ? '' : `?cursor=${encodeURIComponent(cursor)}`;
      const response = await fetch(`/api/history${query}`, { cache: 'no-store' });
      if (!response.ok) throw new Error('History request failed');
      const page = (await response.json()) as HistoryPage;
      if (sequence !== historySequence.current) return;
      setItems((previous) => cursor === null ? page.items : [
        ...previous,
        ...page.items.filter((item) => !previous.some((old) => old.id === item.id)),
      ]);
      setNextCursor(page.nextCursor);
    } catch {
      if (sequence === historySequence.current) setHistoryError(true);
    } finally {
      if (sequence === historySequence.current) setHistoryLoading(false);
    }
  }, [ensureSession]);

  useEffect(() => {
    void loadHistory(null);
  }, [loadHistory]);

  const submit = useCallback(async (request: CalculationRequest) => {
    const sequence = ++submissionSequence.current;
    setResult({ kind: 'loading', expression: request.expression });
    try {
      await ensureSession();
      const response = await fetch('/api/calculations', {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify(request),
      });
      if (!response.ok) throw new Error(requestError(response.status));
      const data = (await response.json()) as CalculationResponse;
      if (sequence === submissionSequence.current) {
        setResult({ kind: 'record', record: data.calculation });
      }
      void loadHistory(null);
    } catch (error) {
      if (sequence === submissionSequence.current) {
        setResult({
          kind: 'failed',
          message: error instanceof Error ? error.message : requestError(0),
          request,
        });
      }
    }
  }, [ensureSession, loadHistory]);

  const selectHistory = useCallback((record: CalculationRecord) => {
    submissionSequence.current++;
    setExpression(record.expression);
    setAngleUnit(record.context.angleUnit);
    setResult({ kind: 'record', record });
    document.getElementById('expression')?.focus();
  }, []);

  const outcome = result.kind === 'record' ? result.record.outcome : null;
  const source = result.kind === 'record' ? result.record.expression : '';
  const span = outcome?.kind === 'error' ? outcome.error.span : null;
  const display = outcome?.kind === 'success' ? displayValue(outcome.value) : null;

  return (
    <main className="workspace">
      <header>
        <p className="eyebrow">Научный калькулятор</p>
        <h1>Супер-дупер калькулятор</h1>
        <p className="intro">Введите выражение и нажмите Enter или «Вычислить».</p>
      </header>

      <CalculatorInput
        expression={expression}
        angleUnit={angleUnit}
        onExpressionChange={setExpression}
        onAngleUnitChange={setAngleUnit}
        onSubmit={() => void submit({ requestId: crypto.randomUUID(), expression, angleUnit })}
      >

      <section className="calculation-result" aria-labelledby="result-heading">
        <h2 id="result-heading">Результат</h2>
        {result.kind === 'idle' && <p>Здесь появится результат вычисления.</p>}
        {result.kind === 'loading' && <p role="status">Вычисляем: <code>{result.expression}</code></p>}
        {result.kind === 'failed' && (
          <div role="alert">
            <p>{result.message}</p>
            <button type="button" onClick={() => void submit(result.request)}>Повторить</button>
          </div>
        )}
        {result.kind === 'record' && (
          <div role={outcome?.kind === 'error' ? 'alert' : 'status'}>
            <p className="result-source"><code>{source}</code> · {result.record.context.angleUnit}</p>
            {outcome?.kind === 'success' ? (
              <>
                <p className="result-value">{display?.approximate ? '≈' : '='} {display?.text}</p>
                {display?.approximate && (
                  <p className="full-value">Полное значение: <code>{outcome.value}</code></p>
                )}
              </>
            ) : outcome?.kind === 'error' ? (
              <>
                <p className="result-error">
                  {messages.history.mathErrors[outcome.error.code] ?? messages.history.mathErrorUnknown}
                </p>
                {span && (
                  <code className="result-highlight">
                    {source.slice(0, span.start)}
                    <mark>{source.slice(span.start, span.end) || '│'}</mark>
                    {source.slice(span.end)}
                  </code>
                )}
              </>
            ) : null}
          </div>
        )}
      </section>
      </CalculatorInput>

      <History
        items={items}
        nextCursor={nextCursor}
        loading={historyLoading}
        hasReadError={historyError}
        language="ru"
        messages={messages}
        onLoadMore={() => void loadHistory(nextCursor)}
        onSelect={selectHistory}
      />
      {historyError && !historyLoading && (
        <button type="button" onClick={() => void loadHistory(null)}>Повторить загрузку истории</button>
      )}
    </main>
  );
}
