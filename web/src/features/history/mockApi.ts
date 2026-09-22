import type { HistoryPage } from '../../contracts';
import { mockPage1, mockPage2 } from './mockData';

/** Контракт: по умолчанию 50 записей, не более 100. */
export const PAGE_SIZE_DEFAULT = 50;
export const PAGE_SIZE_MAX = 100;

export function clampPageSize(limit: number | undefined): number {
  if (limit === undefined || !Number.isFinite(limit) || limit <= 0) {
    return PAGE_SIZE_DEFAULT;
  }
  return Math.min(Math.floor(limit), PAGE_SIZE_MAX);
}

export type HistoryQuery = {
  cursor: string | null;
  limit: number;
};

export type FetchPage = (query: HistoryQuery) => Promise<HistoryPage>;

const delay = (ms: number) => new Promise<void>((resolve) => setTimeout(resolve, ms));

export const fetchNormal: FetchPage = async ({ cursor }) => {
  await delay(250);
  if (cursor === null) return mockPage1;
  if (cursor === 'mock-cursor-2') return mockPage2;
  throw new Error(`Unknown cursor: ${String(cursor)}`);
};

export const fetchEmpty: FetchPage = async () => {
  await delay(150);
  return { items: [], nextCursor: null };
};

export const fetchError: FetchPage = async () => {
  await delay(200);
  throw new Error('Simulated read failure');
};