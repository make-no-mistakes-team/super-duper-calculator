import { useCallback, useEffect, useRef, useState } from 'react';
import type { AngleUnit, CalculationRecord } from '../../contracts';
import { clampPageSize, PAGE_SIZE_DEFAULT, type FetchPage } from './mockApi';

export type HistoryHandlers = {
  items: CalculationRecord[];
  nextCursor: string | null;
  loading: boolean;
  hasReadError: boolean;
  handleLoadMore: () => void;
  handleSelect: (record: CalculationRecord) => void;
};

/** Убираем дубликаты по id, порядок сохраняем: новые — в конец. */
function mergeUnique(
  existing: CalculationRecord[],
  added: CalculationRecord[],
): CalculationRecord[] {
  const seen = new Set(existing.map((item) => item.id));
  const merged = [...existing];
  for (const item of added) {
    if (!seen.has(item.id)) {
      seen.add(item.id);
      merged.push(item);
    }
  }
  return merged;
}

/** Лимит, который клиент запрашивает у сервера. */
const REQUEST_LIMIT = clampPageSize(PAGE_SIZE_DEFAULT);

export function useHistoryHandlers(
  fetchPage: FetchPage,
  onRestore: (expression: string, angleUnit: AngleUnit) => void,
): HistoryHandlers {
  const [items, setItems] = useState<CalculationRecord[]>([]);
  const [nextCursor, setNextCursor] = useState<string | null>(null);
  const [loading, setLoading] = useState(true);
  const [hasReadError, setHasReadError] = useState(false);
  const requestIdRef = useRef(0);

  const load = useCallback(
    async (cursor: string | null) => {
      const requestId = ++requestIdRef.current;
      setLoading(true);
      setHasReadError(false);
      try {
        const page = await fetchPage({ cursor, limit: REQUEST_LIMIT });
        if (requestId !== requestIdRef.current) return;
        setItems((prev) =>
          cursor === null ? page.items : mergeUnique(prev, page.items),
        );
        setNextCursor(page.nextCursor);
      } catch {
        if (requestId !== requestIdRef.current) return;
        setHasReadError(true);
      } finally {
        if (requestId === requestIdRef.current) setLoading(false);
      }
    },
    [fetchPage],
  );

  useEffect(() => {
    setItems([]);
    setNextCursor(null);
    void load(null);
    return () => {
      requestIdRef.current++;
    };
  }, [load]);

  const handleLoadMore = useCallback(() => {
    if (loading || nextCursor === null) return;
    void load(nextCursor);
  }, [loading, nextCursor, load]);

  const handleSelect = useCallback(
    (record: CalculationRecord) => {
      onRestore(record.expression, record.context.angleUnit);
    },
    [onRestore],
  );

  return { items, nextCursor, loading, hasReadError, handleLoadMore, handleSelect };
}