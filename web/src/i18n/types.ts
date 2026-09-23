export type Language = 'ru' | 'en';

export type HistoryMessages = {
  title: string;
  demo: {
    title: string;
    languageLabel: string;
    scenarioLabel: string;
    scenarios: { normal: string; empty: string; error: string };
  };
  loading: string;
  empty: string;
  loadMore: string;
  readError: string;
  unsupported: string;
  restoredLabel: string;
  angleUnitLabel: { deg: string; rad: string };
  outcomeLabel: { success: string; error: string };
  mathErrors: Record<string, string>;
  mathErrorUnknown: string;
};

export type Messages = {
  history: HistoryMessages;
  reduction: {
    heading: string;
    action: string;
    record: string;
    loading: string;
    unavailable: string;
    step: string;
    next: string;
    skip: string;
    close: string;
    replay: string;
    final: string;
    noSteps: string;
  };
};
