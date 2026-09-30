import { useCallback, useEffect, useRef, useState } from 'react';
import { ApiStatusError, getCapabilities, getHistory, getStatistics, postCalculation, startSession } from './api';
import { CalculatorInput } from './CalculatorInput';
import type { Achievement, CalculationRecord, CalculationRequest, Capabilities, DiscoveryDefinition, FunEvent, SessionResponse } from './contracts';
import { History } from './features/history/History';
import { AchievementCollection } from './features/discoveries/AchievementCollection';
import { useAchievementCelebration } from './features/discoveries/AchievementCelebration';
import { DiscoveryNotice } from './features/discoveries/DiscoveryNotice';
import { ComicIncident } from './features/discoveries/ComicIncident';
import { PreferencesPanel, usePreferences } from './features/preferences/Preferences';
import { StatisticsPanel } from './features/statistics/StatisticsPanel';
import { ResultView } from './features/calculation/ResultView';
import { outcomeAnnouncement, requestError, unavailableExtension, type CopyStatus, type ResultState } from './features/calculation/presentation';
import { useToolController, type CalculatorTool } from './features/tools/useToolController';
import { getMessages } from './i18n';

const messages = getMessages('ru');
const calculationMessages = messages.calculation;

function mergeAchievements(previous: Achievement[], incoming: Achievement[]): Achievement[] {
  if (incoming.length === 0) return previous;
  const awards = new Map(previous.map((award) => [award.id, award]));
  for (const award of incoming) {
    const existing = awards.get(award.id);
    if (!existing || award.earnedAt < existing.earnedAt) awards.set(award.id, award);
  }
  return [...awards.values()];
}

export default function App() {
  const [expression, setExpression] = useState('');
  const [result, setResult] = useState<ResultState>({ kind: 'idle' });
  const [items, setItems] = useState<CalculationRecord[]>([]);
  const [nextCursor, setNextCursor] = useState<string | null>(null);
  const [historyLoading, setHistoryLoading] = useState(true);
  const [historyError, setHistoryError] = useState(false);
  const [capabilities, setCapabilities] = useState<Capabilities | null>(null);
  const [capabilitiesError, setCapabilitiesError] = useState(false);
  const [copyStatus, setCopyStatus] = useState<CopyStatus>('idle');
  const [restoredRecord, setRestoredRecord] = useState<CalculationRecord | null>(null);
  const [activeTool, setActiveTool] = useState<CalculatorTool | null>(null);
  const [achievements, setAchievements] = useState<Achievement[]>([]);
  const [discoveryCatalog, setDiscoveryCatalog] = useState<DiscoveryDefinition[]>([]);
  const [discoveriesAvailable, setDiscoveriesAvailable] = useState(false);
  const [collectionLoading, setCollectionLoading] = useState(false);
  const [funEvents, setFunEvents] = useState<FunEvent[]>([]);
  const [incidentActive, setIncidentActive] = useState(false);
  const [collectionOpen, setCollectionOpen] = useState(false);
  const [selectedAward, setSelectedAward] = useState<string | null>(null);
  const [identity, setIdentity] = useState<string | null>(null);
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
  const toolController = useToolController(activeTool, setActiveTool, incidentActive || collectionOpen);

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
    setIdentity(null);
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
    setIdentity(data.identity);
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

  const openCollection = useCallback((id: string | null = null) => {
    setSelectedAward(id);
    setCollectionOpen(true);
    void refreshCollection();
  }, [refreshCollection]);

  const celebration = useAchievementCelebration({
    identity,
    catalog: discoveryCatalog,
    soundEnabled: preferences.soundEnabled,
    effectsEnabled: preferences.largeEffects,
    onOpenCollection: openCollection,
  });

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
    setAnnouncement(calculationMessages.loading);
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
        setAnnouncement(outcomeAnnouncement(data.calculation, capabilities, calculationMessages));
      }
      if (mounted.current) {
        setAchievements((previous) => mergeAchievements(previous, data.achievements ?? []));
        celebration.enqueue(data.achievements ?? []);
        setStatisticsRevision((revision) => revision + 1);
        void loadHistory(null);
      }
    } catch (error) {
      if (mounted.current && identity === sessionIdentity.current && (generation === null || generation === sessionGeneration.current) && sequence === submissionSequence.current) {
        if (error instanceof ApiStatusError && error.status === 401) invalidateSession();
        const failure = error instanceof ApiStatusError
          ? requestError(error, calculationMessages)
          : error instanceof SyntaxError
            ? { message: calculationMessages.requestErrors.unreadable, retryable: true }
            : { message: calculationMessages.requestErrors.network, retryable: true };
        setResult({
          kind: 'failed',
          ...failure,
          request,
        });
        setAnnouncement(failure.message);
      }
    }
  }, [ensureSession, loadHistory, capabilities, invalidateSession, celebration.enqueue]);

  const selectHistory = useCallback((record: CalculationRecord) => {
    setExpression(record.expression);
    setRestoredRecord(record);
    toolController.focusEditor();
    setAnnouncement(unavailableExtension(record.expression, capabilities, calculationMessages)
      ? 'Выражение восстановлено, но содержит недоступную операцию. Исправьте его перед вычислением.'
      : 'Выражение восстановлено из истории. Enter — вычислить снова.');
  }, [capabilities, toolController]);

  const copyResult = useCallback(async (value: string) => {
    const sequence = ++copySequence.current;
    try {
      await navigator.clipboard.writeText(value);
      if (sequence === copySequence.current) {
        setCopyStatus('copied');
        setAnnouncement(calculationMessages.copyAnnouncement);
      }
    } catch {
      if (sequence === copySequence.current) {
        setCopyStatus('failed');
        setAnnouncement(calculationMessages.copyFailedAnnouncement);
      }
    }
  }, []);

  const restoredUnavailable = restoredRecord === null ? null : unavailableExtension(expression, capabilities, calculationMessages);

  return (
    <main className="workspace" onKeyDown={toolController.onKeyDown}
      onPointerDownCapture={(event) => { if (event.isTrusted) celebration.unlockAudio(); }}
      onKeyDownCapture={(event) => { if (event.isTrusted) celebration.unlockAudio(); }}>
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
        {capabilities?.features.achievements && <button id="header-achievements" className="header-settings" type="button"
          aria-label="Достижения" aria-haspopup="dialog" aria-expanded={collectionOpen} onClick={() => openCollection()}>
          <svg width="18" height="18" viewBox="0 0 24 24" fill="none" aria-hidden="true">
            <path d="M8 3h8v8a4 4 0 0 1-8 0V3ZM8 5H4v4a4 4 0 0 0 4 4m8-8h4v4a4 4 0 0 1-4 4m-4 2v5m-5 1h10" stroke="currentColor" strokeWidth="2" />
          </svg><span>Достижения</span>
        </button>}
        <button id="sound-toggle" className="header-settings header-sound" type="button"
          aria-label={preferences.soundEnabled ? 'Выключить звук' : 'Включить звук'} aria-pressed={preferences.soundEnabled}
          title={preferences.soundEnabled ? 'Выключить звук' : 'Включить звук'}
          onClick={() => updatePreferences({ soundEnabled: !preferences.soundEnabled })}>
          <svg width="18" height="18" viewBox="0 0 24 24" fill="none" aria-hidden="true">
            <path d="M3 9h4l5-5v16l-5-5H3V9Z" stroke="currentColor" strokeWidth="2" />
            {preferences.soundEnabled
              ? <path d="M16 8a6 6 0 0 1 0 8m3-11a10 10 0 0 1 0 14" stroke="currentColor" strokeWidth="2" />
              : <path d="m3 3 18 18" stroke="currentColor" strokeWidth="2" />}
          </svg>
        </button>
        {(capabilities?.features.themes || capabilities?.features.achievements) && (
          <button id="tool-settings" className="header-settings" type="button" aria-label="Настройки"
            aria-expanded={activeTool === 'settings'} aria-controls={activeTool === 'settings' ? 'tool-bay' : undefined}
            onClick={() => toolController.toggle('settings')}>
            <svg width="18" height="18" viewBox="0 0 24 24" fill="none" aria-hidden="true">
              <path d="M4 7h16M4 17h16M8 4v6m8 4v6" stroke="currentColor" strokeWidth="2" />
            </svg><span>Настройки</span>
          </button>
        )}
        </div>
      </header>

      <CalculatorInput
        expression={expression}
        capabilities={capabilities}
        activeTool={activeTool}
        toolController={toolController}
        onExpressionChange={(value) => { setExpression(value); setRestoredRecord(null); }}
        onSubmit={() => {
          celebration.playSubmitSound();
          void submit({ requestId: crypto.randomUUID(), expression });
        }}
        settings={<PreferencesPanel preferences={preferences} onChange={updatePreferences}
          achievementsAvailable={capabilities?.features.achievements === true} themesAvailable={capabilities?.features.themes === true} />}
        statistics={capabilities?.features.statistics
          ? <StatisticsPanel key={identity} load={loadStatistics} refreshKey={statisticsRevision} />
          : <p className="capabilities-note">Статистика недоступна на этом сервере.</p>}
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
            isUnsupported={(record) => unavailableExtension(record.expression, capabilities, calculationMessages) !== null}
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
      <ComicIncident events={funEvents} enabled={preferences.humor && preferences.largeEffects && !celebration.active && !collectionOpen}
        onActiveChange={setIncidentActive} />
      <ResultView result={result} expression={expression} capabilities={capabilities}
        messages={calculationMessages} copyStatus={copyStatus}
        onCopy={(value) => void copyResult(value)} onRetry={(request) => void submit(request)}
        onCorrect={(record) => {
          setExpression(record.expression);
          setRestoredRecord(null);
          const span = record.outcome.kind === 'error' ? record.outcome.error.span : null;
          toolController.focusEditor({ selection: span ?? undefined });
        }} />
      <DiscoveryNotice events={funEvents} catalog={discoveryCatalog} humorEnabled={preferences.humor && !incidentActive && !celebration.active}
        onOpenCollection={() => openCollection()} />
      {capabilitiesError && <p className="capabilities-note" role="status">Не удалось проверить возможности сервера. Доступны базовые операции.</p>}
      </CalculatorInput>
      {celebration.view}
      <AchievementCollection key={identity} catalog={discoveryCatalog} achievements={achievements} available={discoveriesAvailable}
        loading={collectionLoading} open={collectionOpen} selectedId={selectedAward}
        freshAwardIds={celebration.freshAwardIds}
        onToggle={(open) => { if (open) openCollection(); else setCollectionOpen(false); }}
        onRetry={() => void refreshCollection()} />
    </main>
  );
}
