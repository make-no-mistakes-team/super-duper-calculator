import type { HistoryPage } from '../../contracts';

export const mockPage1: HistoryPage = {
  items: [
    {
      id: 'a1',
      requestId: 'r1',
      expression: '5*(',
      context: { angleUnit: 'deg', semanticsVersion: 'binary64-v1' },
      outcome: {
        kind: 'error',
        error: {
          code: 'SYNTAX_ERROR',
          stage: 'parse',
          params: { expected: 'operand' },
          span: { start: 3, end: 3 },
        },
      },
      createdAt: '2026-09-11T10:01:00Z',
    },
    {
      id: 'a2',
      requestId: 'r2',
      expression: 'sqrt(81)+2^3',
      context: { angleUnit: 'deg', semanticsVersion: 'binary64-v1' },
      outcome: { kind: 'success', value: '17' },
      facts: {
        operators: { '+': 1, '^': 1 },
        functions: { sqrt: 1 },
        operationCount: 3,
        depth: 1,
      },
      createdAt: '2026-09-11T10:00:00Z',
    },
    // Одинаковое время с a2 — проверяем стабильный порядок.
    {
      id: 'a3',
      requestId: 'r3',
      expression: 'sin(30)',
      context: { angleUnit: 'deg', semanticsVersion: 'binary64-v1' },
      outcome: { kind: 'success', value: '0.5' },
      createdAt: '2026-09-11T10:00:00Z',
    },
  ],
  nextCursor: 'mock-cursor-2',
};

export const mockPage2: HistoryPage = {
  items: [
    // Старая запись с расширением, которого больше нет (5! — факториал).
    {
      id: 'b1',
      requestId: 'r4',
      expression: '5!',
      context: { angleUnit: 'deg', semanticsVersion: 'binary64-v0' },
      outcome: { kind: 'success', value: '120' },
      createdAt: '2026-09-10T15:00:00Z',
    },
    {
      id: 'b2',
      requestId: 'r5',
      expression: 'sqrt(-1)',
      context: { angleUnit: 'rad', semanticsVersion: 'binary64-v1' },
      outcome: {
        kind: 'error',
        error: {
          code: 'DOMAIN_ERROR',
          stage: 'evaluate',
          params: {},
          span: { start: 0, end: 8 },
        },
      },
      createdAt: '2026-09-10T14:00:00Z',
    },
  ],
  nextCursor: null,
};

export const mockEmpty: HistoryPage = { items: [], nextCursor: null };