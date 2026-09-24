import { useCallback, useEffect, useRef, useState } from 'react';
import { ApiStatusError, getCapabilities, getHistory, postCalculation, startSession } from './api';
import { CalculatorInput } from './CalculatorInput';
import type { AngleUnit, CalculationRecord, CalculationRequest, Capabilities, CalculationResponse } from './contracts';
import { History } from './features/history/History';
import { getMessages } from './i18n';

type ResultState =
  | { kind: 'idle' }
  | { kind: 'loading'; request: CalculationRequest }
  | { kind: 'record'; record: CalculationRecord; publication: CalculationResponse['publication']['status'] }
  | { kind: 'failed'; message: string; retryable: boolean; request: CalculationRequest };

const messages = getMessages('ru');
const networkError = 'Не удалось сохранить вычисление. Проверьте соединение и повторите попытку.';
const errorAdvice: Record<string, string> = {
  SYNTAX_ERROR: 'Проверьте скобки и порядок операций.',
  UNKNOWN_IDENTIFIER: 'Проверьте имя функции. Доступные функции — на панели «Функции».',
  UNSUPPORTED_FEATURE: 'Используйте операции, доступные на панели инструментов.',
  WRONG_ARITY: 'Проверьте число аргументов функции. Разделяйте аргументы запятыми.',
  DIVISION_BY_ZERO: 'Измените знаменатель: делить на ноль нельзя.',
  DOMAIN_ERROR: 'Проверьте аргументы функции: для этих значений результат не определён.',
  NUMERIC_OVERFLOW: 'Уменьшите числа или степень в выражении.',
};

function unavailableExtension(expression: string, capabilities: Capabilities | null): string | null {
  if (capabilities === null) return null;
  if (!capabilities.features.factorial && expression.includes('!')) return 'факториал';
  if (!capabilities.features.percentage && expression.includes('%')) return 'проценты';
  if (!capabilities.features.remainder && /\bmod\s*\(/.test(expression)) return 'остаток от деления';
  return null;
}

function displayValue(value: string) {
  const number = Number(value);
  const text = Object.is(number, -0) ? '-0' : String(Number(number.toPrecision(12)));
  return { text, approximate: !Object.is(Number(text), number) };
}

function requestError(error: ApiStatusError): { message: string; retryable: boolean } {
  const { status } = error;
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
  if (status === 401) {
    return { message: 'Сессия истекла. Повторите попытку, чтобы создать новую.', retryable: true };
  }
  if (status === 429) {
    return { message: error.retryAfterSeconds === null
      ? 'Слишком много запросов. Повторите попытку позже.'
      : `Слишком много запросов. Повторите через ${error.retryAfterSeconds} с.`, retryable: true };
  }
  if (status === 408) {
    return { message: 'Время ожидания истекло. Повторите попытку.', retryable: true };
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
  const [capabilities, setCapabilities] = useState<Capabilities | null>(null);
  const [capabilitiesError, setCapabilitiesError] = useState(false);
  const [copyStatus, setCopyStatus] = useState<'idle' | 'copied' | 'failed'>('idle');
  const [restoredRecord, setRestoredRecord] = useState<CalculationRecord | null>(null);
  const [activeTool, setActiveTool] = useState<'functions' | 'keypad' | 'history' | null>(null);
  const submissionSequence = useRef(0);
  const historySequence = useRef(0);
  const copySequence = useRef(0);
  const sessionPromise = useRef<Promise<void> | null>(null);
  const mounted = useRef(false);

  const ensureSession = useCallback(() => {
    if (sessionPromise.current === null) {
      sessionPromise.current = startSession()
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
      const page = await getHistory(cursor);
      if (!mounted.current || sequence !== historySequence.current) return;
      setItems((previous) => cursor === null ? page.items : [
        ...previous,
        ...page.items.filter((item) => !previous.some((old) => old.id === item.id)),
      ]);
      setNextCursor(page.nextCursor);
    } catch (error) {
      if (error instanceof ApiStatusError && error.status === 401) sessionPromise.current = null;
      if (mounted.current && sequence === historySequence.current) setHistoryError(true);
    } finally {
      if (mounted.current && sequence === historySequence.current) setHistoryLoading(false);
    }
  }, [ensureSession]);

  useEffect(() => {
    mounted.current = true;
    void loadHistory(null);
    void getCapabilities().then((data) => {
      if (mounted.current) { setCapabilities(data); setCapabilitiesError(false); }
    }).catch(() => { if (mounted.current) setCapabilitiesError(true); });
    return () => {
      mounted.current = false;
      historySequence.current++;
      submissionSequence.current++;
    };
  }, [loadHistory]);

  const submit = useCallback(async (request: CalculationRequest) => {
    const sequence = ++submissionSequence.current;
    copySequence.current++;
    setCopyStatus('idle');
    setRestoredRecord(null);
    setResult({ kind: 'loading', request });
    try {
      await ensureSession();
      if (!mounted.current) return;
      const data = await postCalculation(request);
      if (mounted.current && sequence === submissionSequence.current) {
        setResult({ kind: 'record', record: data.calculation, publication: data.publication.status });
      }
      if (mounted.current) void loadHistory(null);
    } catch (error) {
      if (mounted.current && sequence === submissionSequence.current) {
        if (error instanceof ApiStatusError && error.status === 401) sessionPromise.current = null;
        const failure = error instanceof ApiStatusError
          ? requestError(error)
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
    setRestoredRecord(record);
    setActiveTool(null);
    document.getElementById('expression')?.focus();
  }, []);

  const copyResult = useCallback(async (value: string) => {
    const sequence = ++copySequence.current;
    try {
      await navigator.clipboard.writeText(value);
      if (sequence === copySequence.current) setCopyStatus('copied');
    } catch {
      if (sequence === copySequence.current) setCopyStatus('failed');
    }
  }, []);

  const outcome = result.kind === 'record' ? result.record.outcome : null;
  const source = result.kind === 'record' ? result.record.expression : '';
  const span = outcome?.kind === 'error' ? outcome.error.span : null;
  const display = outcome?.kind === 'success' ? displayValue(outcome.value) : null;
  const restoredUnavailable = restoredRecord === null ? null : unavailableExtension(expression, capabilities);

  return (
    <main className="workspace">
      <header className="workspace-header">
        <h1 className="workspace-brand">
          <svg className="brand-mark" width="32" height="32" viewBox="0 0 32 32" fill="currentColor" aria-hidden="true">
            <path d="M0 0h12v4H4v8H0zM20 0h12v12h-4V4h-8zM0 20h4v8h8v4H0zM28 20h4v12H20v-4h8zM10 10h4v4h-4zm4 4h4v4h-4zm-4 4h4v4h-4zm10 0h4v4h-4z" />
          </svg>
          <span>Unnecessarily<br /><strong>Advanced Calculator</strong></span>
        </h1>
        <div className="workspace-state" aria-label="Личный режим: вычисления не публикуются">
          <svg width="14" height="14" viewBox="0 0 24 24" fill="none" aria-hidden="true">
            <path d="M7 10V7a5 5 0 0 1 10 0v3M5 10h14v11H5z" stroke="currentColor" strokeWidth="2" />
          </svg>
          Личный
        </div>
      </header>

      <CalculatorInput
        expression={expression}
        angleUnit={angleUnit}
        capabilities={capabilities}
        activeTool={activeTool}
        onToolChange={setActiveTool}
        onExpressionChange={(value) => { setExpression(value); setRestoredRecord(null); }}
        onAngleUnitChange={setAngleUnit}
        onSubmit={() => void submit({ requestId: crypto.randomUUID(), expression, angleUnit })}
        history={<>
          <History
            items={items}
            nextCursor={nextCursor}
            loading={historyLoading}
            hasReadError={historyError}
            language="ru"
            messages={messages}
            onLoadMore={() => void loadHistory(nextCursor)}
            onSelect={selectHistory}
            isUnsupported={(record) => unavailableExtension(record.expression, capabilities) !== null}
          />
          {historyError && !historyLoading && (
            <button className="secondary-button" type="button" onClick={() => void loadHistory(null)}>Повторить загрузку</button>
          )}
        </>}
      >
      {restoredRecord && !restoredUnavailable && (
        <p className="restoration-note" role="status">Выражение из истории. <kbd>Enter</kbd> — вычислить снова.</p>
      )}
      {restoredUnavailable && (
        <p className="restored-unsupported" role="status">
          Операция «{restoredUnavailable}» сейчас недоступна. Сохранённый ответ остаётся в истории; исправьте выражение перед новым вычислением.
        </p>
      )}
      {result.kind !== 'idle' && <section className="calculation-result" aria-labelledby="result-heading"
        key={result.kind === 'record' ? result.record.id : result.kind}>
        <h2 id="result-heading" className="visually-hidden">Результат</h2>
        {result.kind === 'loading' && (
          <p className="result-source result-pending" role="status">
            Вычисляем: <code>{result.request.expression}</code> · {messages.history.angleUnitLabel[result.request.angleUnit]}
          </p>
        )}
        {result.kind === 'failed' && (
          <div role="alert">
            <p className="result-source">
              <code>{result.request.expression}</code> · {messages.history.angleUnitLabel[result.request.angleUnit]}
            </p>
            <p className="result-error">{result.message}</p>
            {result.retryable && (
              <button className="secondary-button" type="button" onClick={() => void submit(result.request)}>Повторить</button>
            )}
          </div>
        )}
        {result.kind === 'record' && (
          <div role={outcome?.kind === 'error' ? 'alert' : undefined}>
            <p className="result-source">
              <code>{source}</code><span className="result-angle" title={messages.history.angleUnitLabel[result.record.context.angleUnit]}>
                {result.record.context.angleUnit.toUpperCase()}
              </span>
            </p>
            {result.publication === 'published' && <p className="publication-status">Опубликовано в комнате</p>}
            {result.publication === 'unavailable' && <p className="publication-status publication-status--warning">Вычисление сохранено лично; публикация недоступна.</p>}
            {outcome?.kind === 'success' ? (
              <>
                <div className="result-answer">
                  <p className="result-value" role="status"><span>{display?.approximate ? '≈' : '='}</span> {display?.text}</p>
                  <button type="button" className="copy-result" title="Скопировать точное значение"
                    aria-label={copyStatus === 'copied' ? 'Скопировано' : 'Скопировать точное значение'}
                    onClick={() => void copyResult(outcome.value)}>
                    <svg width="18" height="18" viewBox="0 0 24 24" fill="none" aria-hidden="true">
                      {copyStatus === 'copied'
                        ? <path d="m4 12 5 5L20 6" stroke="currentColor" strokeWidth="2" />
                        : <path d="M9 9h11v11H9zM5 15H3V3h12v2" stroke="currentColor" strokeWidth="2" />}
                    </svg>
                    <span>{copyStatus === 'copied' ? 'Скопировано' : 'Копировать'}</span>
                  </button>
                </div>
                <span className="visually-hidden" role="status">{copyStatus === 'copied' ? 'Точное значение скопировано' : ''}</span>
                {display?.approximate && (
                  <details className="full-value"><summary>Точное значение</summary><code>{outcome.value}</code></details>
                )}
                {copyStatus === 'failed' && <p className="copy-error" role="alert">Не удалось скопировать. Выделите значение вручную: <code>{outcome.value}</code></p>}
              </>
            ) : outcome?.kind === 'error' ? (
              <>
                <p className="result-error">
                  {messages.history.mathErrors[outcome.error.code] ?? messages.history.mathErrorUnknown}
                </p>
                <p className="error-advice">{errorAdvice[outcome.error.code] ?? 'Проверьте выражение и попробуйте снова.'}</p>
                {span && (
                  <code className="result-highlight">
                    {source.slice(0, span.start)}
                    <mark>{source.slice(span.start, span.end) || '│'}</mark>
                    {source.slice(span.end)}
                  </code>
                )}
                <button type="button" className="secondary-button" onClick={() => {
                  setExpression(source);
                  setAngleUnit(result.record.context.angleUnit);
                  setRestoredRecord(null);
                  setActiveTool(null);
                  requestAnimationFrame(() => {
                    const input = document.getElementById('expression');
                    if (input instanceof HTMLTextAreaElement) {
                      input.focus();
                      input.setSelectionRange(span?.start ?? source.length, span?.end ?? source.length);
                    }
                  });
                }}>{expression === source ? 'Исправить' : 'Вернуть выражение'}</button>
              </>
            ) : null}
          </div>
        )}
      </section>}
      {capabilitiesError && <p className="capabilities-note" role="status">Не удалось проверить возможности сервера. Доступны базовые операции.</p>}
      </CalculatorInput>
    </main>
  );
}
