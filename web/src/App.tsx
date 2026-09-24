import { useCallback, useEffect, useRef, useState } from 'react';
import { ApiStatusError, getCapabilities, getHistory, getStatistics, postCalculation, startSession } from './api';
import { CalculatorInput } from './CalculatorInput';
import type { Achievement, AngleUnit, CalculationRecord, CalculationRequest, Capabilities, CalculationResponse, DiscoveryDefinition, FunEvent, MathError, SessionResponse } from './contracts';
import { History } from './features/history/History';
import { AchievementCollection } from './features/discoveries/AchievementCollection';
import { DiscoveryNotice } from './features/discoveries/DiscoveryNotice';
import { ComicIncident } from './features/discoveries/ComicIncident';
import { PreferencesPanel, usePreferences } from './features/preferences/Preferences';
import { StatisticsPanel } from './features/statistics/StatisticsPanel';
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
const syntaxExpectations: Record<string, string> = {
  operand: 'Не хватает числа или выражения.',
  operator: 'Между частями выражения нужен оператор.',
  digit: 'В числе не хватает цифры.',
  exponent: 'После e укажите степень, например 1e3.',
  ')': 'Не хватает закрывающей скобки.',
  '(': 'Не хватает открывающей скобки.',
};

function mathErrorText(error: MathError, capabilities: Capabilities | null): string {
  const text = (value: unknown) => typeof value === 'string' ? value.slice(0, 80) : null;
  const name = text(error.params?.name);
  if (error.code === 'UNKNOWN_IDENTIFIER' && name) return `Неизвестное имя «${name}».`;
  if (error.code === 'WRONG_ARITY' && name) {
    const arities = capabilities && Object.hasOwn(capabilities.functions, name) ? capabilities.functions[name] : undefined;
    return `Неверное число аргументов у ${name}.${arities ? ` Ожидается: ${arities.join(' или ')}.` : ''}`;
  }
  if (error.code === 'SYNTAX_ERROR') {
    const token = text(error.params?.expected);
    const expectation = token && Object.hasOwn(syntaxExpectations, token) ? syntaxExpectations[token] : undefined;
    if (expectation) return expectation;
    const unexpected = text(error.params?.unexpected);
    if (unexpected) return `Неожиданный символ «${unexpected}».`;
  }
  return messages.history.mathErrors[error.code] ?? messages.history.mathErrorUnknown;
}

function mergeAchievements(previous: Achievement[], incoming: Achievement[]): Achievement[] {
  if (incoming.length === 0) return previous;
  const awards = new Map(previous.map((award) => [award.id, award]));
  for (const award of incoming) {
    const existing = awards.get(award.id);
    if (!existing || award.earnedAt < existing.earnedAt) awards.set(award.id, award);
  }
  return [...awards.values()];
}

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
  const [activeTool, setActiveTool] = useState<'functions' | 'keypad' | 'history' | 'settings' | null>(null);
  const [achievements, setAchievements] = useState<Achievement[]>([]);
  const [discoveryCatalog, setDiscoveryCatalog] = useState<DiscoveryDefinition[]>([]);
  const [discoveriesAvailable, setDiscoveriesAvailable] = useState(false);
  const [collectionLoading, setCollectionLoading] = useState(false);
  const [funEvents, setFunEvents] = useState<FunEvent[]>([]);
  const [incidentActive, setIncidentActive] = useState(false);
  const [collectionOpen, setCollectionOpen] = useState(false);
  const [statisticsRevision, setStatisticsRevision] = useState(0);
  const [announcement, setAnnouncement] = useState('');
  const { preferences, updatePreferences } = usePreferences();
  const submissionSequence = useRef(0);
  const historySequence = useRef(0);
  const copySequence = useRef(0);
  const sessionInFlight = useRef<Promise<void> | null>(null);
  const sessionReady = useRef(false);
  const sessionIdentity = useRef<string | null>(null);
  const sessionGeneration = useRef(0);
  const mounted = useRef(false);

  const invalidateSession = useCallback(() => {
    sessionGeneration.current++;
    sessionReady.current = false;
    sessionInFlight.current = null;
    setCollectionLoading(false);
    setHistoryLoading(false);
    setResult((previous) => previous.kind === 'loading' ? { kind: 'idle' } : previous);
    setAchievements([]);
    setDiscoveryCatalog([]);
    setDiscoveriesAvailable(false);
    setFunEvents([]);
  }, []);

  // Fetch an already established owner's page without re-entering session bootstrap.
  const readHistoryPage = useCallback(async (cursor: string | null, sequence = ++historySequence.current) => {
    const generation = sessionGeneration.current;
    setHistoryLoading(true);
    setHistoryError(false);
    try {
      const page = await getHistory(cursor);
      if (!mounted.current || generation !== sessionGeneration.current || sequence !== historySequence.current) return;
      setItems((previous) => cursor === null ? page.items : [
        ...previous,
        ...page.items.filter((item) => !previous.some((old) => old.id === item.id)),
      ]);
      setNextCursor(page.nextCursor);
    } catch (error) {
      if (mounted.current && generation === sessionGeneration.current && sequence === historySequence.current) {
        if (error instanceof ApiStatusError && error.status === 401) invalidateSession();
        setHistoryError(true);
      }
    } finally {
      if (mounted.current && generation === sessionGeneration.current && sequence === historySequence.current) setHistoryLoading(false);
    }
  }, [invalidateSession]);

  const applySession = useCallback((data: SessionResponse) => {
    if (!mounted.current) return;
    const changed = sessionIdentity.current !== null && sessionIdentity.current !== data.identity;
    sessionIdentity.current = data.identity;
    if (changed) {
      sessionGeneration.current++;
      submissionSequence.current++;
      historySequence.current++;
      copySequence.current++;
      setCopyStatus('idle');
      setAchievements(data.achievements ?? []);
      setDiscoveryCatalog([]);
      setItems([]);
      setNextCursor(null);
      setHistoryLoading(false);
      setHistoryError(false);
      setResult({ kind: 'idle' });
      setRestoredRecord(null);
      setFunEvents([]);
      setStatisticsRevision((revision) => revision + 1);
      setAnnouncement('Создана новая личная сессия. Предыдущая история относится к прежней сессии.');
      void readHistoryPage(null);
    }
    setDiscoveriesAvailable(data.discoveriesAvailable === true);
    if (data.discoveriesAvailable) {
      setAchievements((previous) => mergeAchievements(previous, data.achievements ?? []));
      setDiscoveryCatalog(data.discoveryCatalog ?? []);
    }
  }, [readHistoryPage]);

  const requestSession = useCallback(() => {
    if (sessionInFlight.current !== null) return sessionInFlight.current;
    const generation = sessionGeneration.current;
    setCollectionLoading(true);
    const pending = startSession()
      .then((data) => {
        if (!mounted.current || generation !== sessionGeneration.current) throw new Error('Session changed');
        applySession(data);
        sessionReady.current = true;
      })
      .catch((error: unknown) => {
        if (mounted.current && generation === sessionGeneration.current && error instanceof ApiStatusError && error.status === 401) {
          invalidateSession();
        }
        throw error;
      })
      .finally(() => {
        if (sessionInFlight.current === pending) {
          sessionInFlight.current = null;
          if (mounted.current) setCollectionLoading(false);
        }
      });
    sessionInFlight.current = pending;
    return pending;
  }, [applySession, invalidateSession]);

  const ensureSession = useCallback(async () => {
    if (!sessionReady.current) await requestSession();
  }, [requestSession]);

  const refreshCollection = useCallback(async () => {
    const generation = sessionGeneration.current;
    try {
      await requestSession();
    } catch {
      if (mounted.current && generation === sessionGeneration.current) setDiscoveriesAvailable(false);
    }
  }, [requestSession]);

  const loadStatistics = useCallback(async () => {
    await ensureSession();
    const generation = sessionGeneration.current;
    try {
      const statistics = await getStatistics();
      if (generation !== sessionGeneration.current) throw new Error('Session changed');
      return statistics;
    } catch (error) {
      if (generation === sessionGeneration.current && error instanceof ApiStatusError && error.status === 401) invalidateSession();
      throw error;
    }
  }, [ensureSession, invalidateSession]);

  const loadHistory = useCallback(async (cursor: string | null) => {
    const sequence = ++historySequence.current;
    setHistoryLoading(true);
    setHistoryError(false);
    try {
      await ensureSession();
      // Identity adoption already starts its first page and supersedes old cursors.
      if (!mounted.current || sequence !== historySequence.current) return;
      await readHistoryPage(cursor, sequence);
    } catch (error) {
      if (mounted.current && sequence === historySequence.current) {
        if (error instanceof ApiStatusError && error.status === 401) invalidateSession();
        setHistoryError(true);
      }
    } finally {
      if (mounted.current && sequence === historySequence.current) setHistoryLoading(false);
    }
  }, [ensureSession, invalidateSession, readHistoryPage]);

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
    let sequence = ++submissionSequence.current;
    let identity = sessionIdentity.current;
    let generation: number | null = null;
    copySequence.current++;
    setCopyStatus('idle');
    setRestoredRecord(null);
    setAnnouncement('Вычисляем…');
    setFunEvents([]);
    setResult({ kind: 'loading', request });
    try {
      await ensureSession();
      if (!mounted.current) return;
      // A renewed identity can clear an old outcome during bootstrap.
      // This explicit submission belongs to the newly established session.
      sequence = ++submissionSequence.current;
      identity = sessionIdentity.current;
      generation = sessionGeneration.current;
      const data = await postCalculation(request);
      if (generation !== sessionGeneration.current) return;
      if (mounted.current && sequence === submissionSequence.current) {
        setResult({ kind: 'record', record: data.calculation, publication: data.publication.status });
        setFunEvents(data.funEvents ?? []);
        const outcome = data.calculation.outcome;
        if (outcome.kind === 'success') {
          const value = displayValue(outcome.value);
          setAnnouncement(`Результат: ${value.approximate ? 'приблизительно ' : ''}${value.text}.`);
        } else {
          setAnnouncement(`${outcome.error.stage === 'parse' ? 'Ошибка в записи выражения' : 'Ошибка вычисления'}: ${mathErrorText(outcome.error, capabilities)}`);
        }
      }
      if (mounted.current) {
        setAchievements((previous) => mergeAchievements(previous, data.achievements ?? []));
        setStatisticsRevision((revision) => revision + 1);
        void loadHistory(null);
      }
    } catch (error) {
      if (mounted.current && identity === sessionIdentity.current && (generation === null || generation === sessionGeneration.current) && sequence === submissionSequence.current) {
        if (error instanceof ApiStatusError && error.status === 401) invalidateSession();
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
        setAnnouncement(failure.message);
      }
    }
  }, [ensureSession, loadHistory, capabilities, invalidateSession]);

  const selectHistory = useCallback((record: CalculationRecord) => {
    setExpression(record.expression);
    setAngleUnit(record.context.angleUnit);
    setRestoredRecord(record);
    setActiveTool(null);
    setAnnouncement(unavailableExtension(record.expression, capabilities)
      ? 'Выражение восстановлено, но содержит недоступную операцию. Исправьте его перед вычислением.'
      : 'Выражение и угловой режим восстановлены из истории. Enter — вычислить снова.');
    document.getElementById('expression')?.focus();
  }, [capabilities]);

  const copyResult = useCallback(async (value: string) => {
    const sequence = ++copySequence.current;
    try {
      await navigator.clipboard.writeText(value);
      if (sequence === copySequence.current) {
        setCopyStatus('copied');
        setAnnouncement('Точное значение скопировано.');
      }
    } catch {
      if (sequence === copySequence.current) {
        setCopyStatus('failed');
        setAnnouncement('Не удалось скопировать. Выделите точное значение вручную.');
      }
    }
  }, []);

  const outcome = result.kind === 'record' ? result.record.outcome : null;
  const source = result.kind === 'record' ? result.record.expression : '';
  const span = outcome?.kind === 'error' ? outcome.error.span : null;
  const display = outcome?.kind === 'success' ? displayValue(outcome.value) : null;
  const restoredUnavailable = restoredRecord === null ? null : unavailableExtension(expression, capabilities);

  return (
    <main className="workspace">
      <div className="visually-hidden" role="status" aria-live="polite" aria-atomic="true">{announcement}</div>
      <header className="workspace-header">
        <h1 className="workspace-brand">
          <svg className="brand-mark" width="32" height="32" viewBox="0 0 32 32" fill="currentColor" aria-hidden="true">
            <path d="M0 0h12v4H4v8H0zM20 0h12v12h-4V4h-8zM0 20h4v8h8v4H0zM28 20h4v12H20v-4h8zM10 10h4v4h-4zm4 4h4v4h-4zm-4 4h4v4h-4zm10 0h4v4h-4z" />
          </svg>
          <span>Unnecessarily<br /><strong>Advanced Calculator</strong></span>
        </h1>
        <div className="workspace-header-actions">
        <div className="workspace-state" aria-label="Личный режим: вычисления не публикуются">
          <svg width="14" height="14" viewBox="0 0 24 24" fill="none" aria-hidden="true">
            <path d="M7 10V7a5 5 0 0 1 10 0v3M5 10h14v11H5z" stroke="currentColor" strokeWidth="2" />
          </svg>
          Личный
        </div>
        {(capabilities?.features.themes || capabilities?.features.achievements) && (
          <button id="tool-settings" className="header-settings" type="button" aria-label="Настройки"
            aria-expanded={activeTool === 'settings'} aria-controls={activeTool === 'settings' ? 'tool-bay' : undefined}
            onKeyDown={(event) => {
              if (event.key === 'Escape' && !event.nativeEvent.isComposing && event.nativeEvent.keyCode !== 229 && activeTool !== null) {
                event.preventDefault();
                const trigger = document.getElementById(`tool-${activeTool}`);
                setActiveTool(null);
                trigger?.focus({ preventScroll: true });
              }
            }}
            onClick={() => {
              setActiveTool(activeTool === 'settings' ? null : 'settings');
              if (activeTool !== 'settings') requestAnimationFrame(() => document.getElementById('tool-bay-heading')?.focus({ preventScroll: true }));
            }}>
            <svg width="18" height="18" viewBox="0 0 24 24" fill="none" aria-hidden="true">
              <path d="M4 7h16M4 17h16M8 4v6m8 4v6" stroke="currentColor" strokeWidth="2" />
            </svg><span>Настройки</span>
          </button>
        )}
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
        settings={<PreferencesPanel preferences={preferences} onChange={updatePreferences}
          achievementsAvailable={capabilities?.features.achievements === true} themesAvailable={capabilities?.features.themes === true} />}
        history={<>
          {capabilities?.features.statistics && <StatisticsPanel key={sessionIdentity.current} load={loadStatistics} refreshKey={statisticsRevision} />}
          {capabilities?.features.achievements && <AchievementCollection
            catalog={discoveryCatalog} achievements={achievements} available={discoveriesAvailable}
            loading={collectionLoading} open={collectionOpen} onToggle={(open) => {
              setCollectionOpen(open);
              if (open) void refreshCollection();
            }} onRetry={() => void refreshCollection()} />}
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
        <p className="restoration-note">Выражение из истории. <kbd>Enter</kbd> — вычислить снова.</p>
      )}
      {restoredUnavailable && (
        <p className="restored-unsupported">
          Операция «{restoredUnavailable}» сейчас недоступна. Сохранённый ответ остаётся в истории; исправьте выражение перед новым вычислением.
        </p>
      )}
      <ComicIncident events={funEvents} enabled={preferences.humor && preferences.largeEffects}
        onActiveChange={setIncidentActive} />
      {result.kind !== 'idle' && <section className="calculation-result" aria-labelledby="result-heading"
        key={result.kind === 'record' ? result.record.id : result.kind}>
        <h2 id="result-heading" className="visually-hidden">Результат</h2>
        {result.kind === 'loading' && (
          <p className="result-source result-pending">
            Вычисляем: <code>{result.request.expression}</code> · {messages.history.angleUnitLabel[result.request.angleUnit]}
          </p>
        )}
        {result.kind === 'failed' && (
          <div>
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
          <div>
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
                  <p className="result-value"><span>{display?.approximate ? '≈' : '='}</span> {display?.text}</p>
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
                {display?.approximate && (
                  <details className="full-value"><summary>Точное значение</summary><code>{outcome.value}</code></details>
                )}
                {copyStatus === 'failed' && <p className="copy-error">Не удалось скопировать. Выделите значение вручную: <code>{outcome.value}</code></p>}
              </>
            ) : outcome?.kind === 'error' ? (
              <>
                <p className="result-error">
                  {mathErrorText(outcome.error, capabilities)}
                </p>
                <p className="error-stage">{outcome.error.stage === 'parse' ? 'При разборе выражения' : 'При вычислении'}</p>
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
      <DiscoveryNotice events={funEvents} catalog={discoveryCatalog} humorEnabled={preferences.humor && !incidentActive}
        onOpenCollection={() => {
          setActiveTool('history');
          setCollectionOpen(true);
          void refreshCollection();
          requestAnimationFrame(() => document.getElementById('tool-bay-heading')?.focus({ preventScroll: true }));
        }} />
      {capabilitiesError && <p className="capabilities-note" role="status">Не удалось проверить возможности сервера. Доступны базовые операции.</p>}
      </CalculatorInput>
    </main>
  );
}
