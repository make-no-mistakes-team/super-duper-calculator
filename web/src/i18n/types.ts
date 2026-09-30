export type Language = 'ru' | 'en';

export type HistoryMessages = {
  title: string;
  loading: string;
  empty: string;
  loadMore: string;
  readError: string;
};

export type CalculationMessages = {
  unsupported: string;
  outcomeLabel: { success: string; error: string };
  mathErrors: Record<string, string>;
  mathErrorUnknown: string;
  errorAdvice: Record<string, string>;
  defaultErrorAdvice: string;
  syntaxExpectations: Record<string, string>;
  unknownIdentifier: (name: string) => string;
  wrongArity: (name: string, arities: number[] | undefined) => string;
  unexpectedSymbol: (symbol: string) => string;
  requestErrors: {
    network: string;
    limit: string;
    invalid: string;
    conflict: string;
    sessionExpired: string;
    throttled: string;
    retryAfter: (seconds: number) => string;
    timeout: string;
    rejected: string;
    unreadable: string;
  };
  loading: string;
  loadingSource: string;
  announcement: (value: string, approximate: boolean) => string;
  errorAnnouncement: { parse: string; evaluate: string };
  errorStage: { parse: string; evaluate: string };
  retry: string;
  published: string;
  publicationUnavailable: string;
  copy: string;
  copyExact: string;
  copied: string;
  exactValue: string;
  copyFailed: string;
  copyAnnouncement: string;
  copyFailedAnnouncement: string;
  goToError: string;
  restoreExpression: string;
  extensions: { factorial: string; percentage: string; remainder: string };
};

export type Messages = {
  history: HistoryMessages;
  calculation: CalculationMessages;
};