import type { CalculationRequest, CalculationResponse, Capabilities, HistoryPage } from './contracts';

export class ApiStatusError extends Error {
  constructor(readonly status: number, readonly retryAfterSeconds: number | null) {
    super(`HTTP ${status}`);
  }
}

async function readJson<T>(response: Response): Promise<T> {
  if (!response.ok) {
    const retryAfter = response.headers.get('Retry-After');
    const seconds = retryAfter === null ? NaN : Number(retryAfter);
    throw new ApiStatusError(response.status, Number.isInteger(seconds) && seconds >= 0 && seconds <= 3600 ? seconds : null);
  }
  return response.json() as Promise<T>;
}

export async function startSession(): Promise<void> {
  await readJson<unknown>(await fetch('/api/session', { cache: 'no-store', credentials: 'same-origin' }));
}

export async function getCapabilities(): Promise<Capabilities> {
  return readJson<Capabilities>(await fetch('/api/capabilities', { cache: 'no-store', credentials: 'same-origin' }));
}

export async function getHistory(cursor: string | null): Promise<HistoryPage> {
  const query = cursor === null ? '' : `?cursor=${encodeURIComponent(cursor)}`;
  return readJson<HistoryPage>(await fetch(`/api/history${query}`, { cache: 'no-store', credentials: 'same-origin' }));
}

export async function postCalculation(request: CalculationRequest): Promise<CalculationResponse> {
  return readJson<CalculationResponse>(await fetch('/api/calculations', {
    method: 'POST',
    credentials: 'same-origin',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify(request),
  }));
}

export async function getReduction(calculationId: string, signal: AbortSignal): Promise<unknown> {
  return readJson<unknown>(await fetch(`/api/calculations/${encodeURIComponent(calculationId)}/reduction`, {
    cache: 'no-store',
    credentials: 'same-origin',
    signal,
  }));
}
