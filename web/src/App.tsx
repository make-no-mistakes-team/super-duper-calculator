import { useCallback, useEffect, useRef, useState } from 'react';
import { CalculatorInput } from './CalculatorInput';
import type { AngleUnit, CalculationRecord, CalculationRequest, CalculationResponse, HistoryPage } from './contracts';
import { History } from './features/history/History';
import { getMessages } from './i18n';

type ResultState =
  | { kind: 'idle' }
  | { kind: 'loading'; request: CalculationRequest }
  | { kind: 'record'; record: CalculationRecord }
  | { kind: 'failed'; message: string; retryable: boolean; request: CalculationRequest };

const messages = getMessages('ru');
const networkError = 'Не удалось сохранить вычисление. Проверьте соединение и повторите попытку.';

class RequestStatusError extends Error {
  constructor(readonly status: number) {
    super('HTTP request failed');
  }
}

function displayValue(value: string) {
  const number = Number(value);
  const text = String(Number(number.toPrecision(12)));
  return { text, approximate: Number(text) !== number };
}

function requestError(status: number): { message: string; retryable: boolean } {
  if (status === 413) {
    return {
      message: 'Превышены ограничения запроса: длина выражения, число токенов или глубина вложенности.',
      retryable: false,
    };
  }
  if (status === 400) {
    return { message: 'Некорректный запрос. Проверьте выражение и настройки.', retryable: false };
  }
  if (status === 409) {
    return { message: 'Действие уже использовано для другого выражения.', retryable: false };
  }
  if (status === 408 || status === 429) {
    return { message: 'Запрос временно отклонён. Повторите попытку позже.', retryable: true };
  }
  if (status >= 400 && status < 500) {
    return { message: 'Сервер отклонил запрос. Проверьте его и отправьте заново.', retryable: false };
  }
  return { message: networkError, retryable: true };
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
  const mounted = useRef(false);

  const ensureSession = useCallback(() => {
    if (sessionPromise.current === null) {
      sessionPromise.current = fetch('/api/session', { cache: 'no-store' })
        .then((response) => {
          if (!response.ok) throw new RequestStatusError(response.status);
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
      if (!mounted.current || sequence !== historySequence.current) return;
      const query = cursor === null ? '' : `?cursor=${encodeURIComponent(cursor)}`;
      const response = await fetch(`/api/history${query}`, { cache: 'no-store' });
      if (!mounted.current || sequence !== historySequence.current) return;
      if (!response.ok) throw new Error('History request failed');
      const page = (await response.json()) as HistoryPage;
      if (!mounted.current || sequence !== historySequence.current) return;
      setItems((previous) => cursor === null ? page.items : [
        ...previous,
        ...page.items.filter((item) => !previous.some((old) => old.id === item.id)),
      ]);
      setNextCursor(page.nextCursor);
    } catch {
      if (mounted.current && sequence === historySequence.current) setHistoryError(true);
    } finally {
      if (mounted.current && sequence === historySequence.current) setHistoryLoading(false);
    }
  }, [ensureSession]);

  useEffect(() => {
    mounted.current = true;
    void loadHistory(null);
    return () => {
      mounted.current = false;
      historySequence.current++;
      submissionSequence.current++;
    };
  }, [loadHistory]);

  const submit = useCallback(async (request: CalculationRequest) => {
    const sequence = ++submissionSequence.current;
    setResult({ kind: 'loading', request });
    try {
      await ensureSession();
      if (!mounted.current) return;
      const response = await fetch('/api/calculations', {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify(request),
      });
      if (!mounted.current) return;
      if (!response.ok) throw new RequestStatusError(response.status);
      const data = (await response.json()) as CalculationResponse;
      if (mounted.current && sequence === submissionSequence.current) {
        setResult({ kind: 'record', record: data.calculation });
      }
      if (mounted.current) void loadHistory(null);
    } catch (error) {
      if (mounted.current && sequence === submissionSequence.current) {
        const failure = error instanceof RequestStatusError
          ? requestError(error.status)
          : error instanceof SyntaxError
            ? { message: 'Не удалось прочитать ответ сервера. Повторите попытку.', retryable: true }
            : { message: networkError, retryable: true };
        setResult({
          kind: 'failed',
          ...failure,
          request,
        });
      }
    }
  }, [ensureSession, loadHistory]);

  const selectHistory = useCallback((record: CalculationRecord) => {
    setExpression(record.expression);
    setAngleUnit(record.context.angleUnit);
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
        {result.kind === 'loading' && (
          <p className="result-source" role="status">
            Вычисляем: <code>{result.request.expression}</code> · {messages.history.angleUnitLabel[result.request.angleUnit]}
          </p>
        )}
        {result.kind === 'failed' && (
          <div role="alert">
            <p className="result-source">
              <code>{result.request.expression}</code> · {messages.history.angleUnitLabel[result.request.angleUnit]}
            </p>
            <p>{result.message}</p>
            {result.retryable && (
              <button type="button" onClick={() => void submit(result.request)}>Повторить</button>
            )}
          </div>
        )}
        {result.kind === 'record' && (
          <div role={outcome?.kind === 'error' ? 'alert' : 'status'}>
            <p className="result-source">
              <code>{source}</code> · {messages.history.angleUnitLabel[result.record.context.angleUnit]}
            </p>
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
