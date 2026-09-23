import type { Messages } from './types';

export const en: Messages = {
  reduction: {
    heading: 'Calculation steps',
    action: 'Show calculation',
    record: 'Saved expression',
    loading: 'Loading steps…',
    unavailable: 'Steps are unavailable for this record. The saved result remains in history.',
    step: 'Step',
    next: 'Next step',
    skip: 'Show result',
    close: 'Close',
    replay: 'Replay',
    final: 'Exact value',
    noSteps: 'This number is already the result; there are no calculation steps.',
  },
  history: {
    title: 'Calculation history',
    demo: {
      title: 'History demo',
      languageLabel: 'Language',
      scenarioLabel: 'Scenario',
      scenarios: { normal: 'Normal', empty: 'Empty', error: 'Read error' },
    },
    loading: 'Loading history…',
    empty: 'No saved calculations yet',
    loadMore: 'Load more',
    readError:
      'Could not load history. The main calculator remains available.',
    unsupported: 'This operation is no longer available for new calculations',
    restoredLabel: 'Restored to the editor',
    angleUnitLabel: { deg: 'degrees', rad: 'radians' },
    outcomeLabel: { success: 'Result', error: 'Error' },
    mathErrors: {
      SYNTAX_ERROR: 'Syntax error',
      UNKNOWN_IDENTIFIER: 'Unknown function or constant',
      UNSUPPORTED_FEATURE: 'Operation is not available',
      WRONG_ARITY: 'Wrong number of arguments',
      DIVISION_BY_ZERO: 'Division by zero',
      DOMAIN_ERROR: 'Value outside the function domain',
      NUMERIC_OVERFLOW: 'Numeric overflow',
    },
    mathErrorUnknown: 'Mathematical error',
  },
};
