export type Language = 'ru' | 'en';

export type HistoryMessages = {
  title: string;
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
};