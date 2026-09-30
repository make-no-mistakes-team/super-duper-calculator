import type { CalculationRecord, CalculationRequest, CalculationResponse, Capabilities, MathError } from '../../contracts';
import type { CalculationMessages } from '../../i18n';

export type ResultState =
  | { kind: 'idle' }
  | { kind: 'loading'; request: CalculationRequest }
  | { kind: 'record'; record: CalculationRecord; publication: CalculationResponse['publication']['status'] }
  | { kind: 'failed'; message: string; retryable: boolean; request: CalculationRequest };

export type CopyStatus = 'idle' | 'copied' | 'failed';

export function displayValue(value: string) {
  const number = Number(value);
  const text = Object.is(number, -0) ? '-0' : String(Number(number.toPrecision(12)));
  return { text, approximate: !Object.is(Number(text), number) };
}

export function mathErrorText(error: MathError, capabilities: Capabilities | null, messages: CalculationMessages): string {
  const text = (value: unknown) => typeof value === 'string' ? value.slice(0, 80) : null;
  const name = text(error.params?.name);
  if (error.code === 'UNKNOWN_IDENTIFIER' && name) return messages.unknownIdentifier(name);
  if (error.code === 'WRONG_ARITY' && name) {
    const arities = capabilities && Object.hasOwn(capabilities.functions, name) ? capabilities.functions[name] : undefined;
    return messages.wrongArity(name, arities);
  }
  if (error.code === 'SYNTAX_ERROR') {
    const token = text(error.params?.expected);
    const expectation = token && Object.hasOwn(messages.syntaxExpectations, token) ? messages.syntaxExpectations[token] : undefined;
    if (expectation) return expectation;
    const unexpected = text(error.params?.unexpected);
    if (unexpected) return messages.unexpectedSymbol(unexpected);
  }
  return messages.mathErrors[error.code] ?? messages.mathErrorUnknown;
}

export function unavailableExtension(expression: string, capabilities: Capabilities | null, messages: CalculationMessages): string | null {
  if (capabilities === null) return null;
  if (!capabilities.features.factorial && expression.includes('!')) return messages.extensions.factorial;
  if (!capabilities.features.percentage && expression.includes('%')) return messages.extensions.percentage;
  if (!capabilities.features.remainder && /\bmod\s*\(/.test(expression)) return messages.extensions.remainder;
  return null;
}

export function requestError(error: { status: number; retryAfterSeconds: number | null }, messages: CalculationMessages): { message: string; retryable: boolean } {
  const { status } = error;
  const copy = messages.requestErrors;
  if (status === 413) return { message: copy.limit, retryable: false };
  if (status === 400) return { message: copy.invalid, retryable: false };
  if (status === 409) return { message: copy.conflict, retryable: false };
  if (status === 401) return { message: copy.sessionExpired, retryable: true };
  if (status === 429) return {
    message: error.retryAfterSeconds === null ? copy.throttled : copy.retryAfter(error.retryAfterSeconds),
    retryable: true,
  };
  if (status === 408) return { message: copy.timeout, retryable: true };
  if (status >= 400 && status < 500) return { message: copy.rejected, retryable: false };
  return { message: copy.network, retryable: true };
}

export function outcomeAnnouncement(record: CalculationRecord, capabilities: Capabilities | null, messages: CalculationMessages): string {
  const { outcome } = record;
  if (outcome.kind === 'success') {
    const value = displayValue(outcome.value);
    return messages.announcement(value.text, value.approximate);
  }
  return `${messages.errorAnnouncement[outcome.error.stage]}: ${mathErrorText(outcome.error, capabilities, messages)}`;
}
