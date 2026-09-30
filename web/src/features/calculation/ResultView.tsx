import type { CalculationRecord, CalculationRequest, Capabilities } from '../../contracts';
import type { CalculationMessages } from '../../i18n';
import { displayValue, mathErrorText, type CopyStatus, type ResultState } from './presentation';

type ResultViewProps = {
  result: ResultState;
  expression: string;
  capabilities: Capabilities | null;
  messages: CalculationMessages;
  copyStatus: CopyStatus;
  onCopy: (value: string) => void;
  onRetry: (request: CalculationRequest) => void;
  onCorrect: (record: CalculationRecord) => void;
};

export function ResultView({ result, expression, capabilities, messages, copyStatus, onCopy, onRetry, onCorrect }: ResultViewProps) {
  if (result.kind === 'idle') return null;
  const outcome = result.kind === 'record' ? result.record.outcome : null;
  const source = result.kind === 'record' ? result.record.expression : '';
  const span = outcome?.kind === 'error' ? outcome.error.span : null;
  const display = outcome?.kind === 'success' ? displayValue(outcome.value) : null;

  return (
    <section className={`calculation-result${result.kind === 'record' ? ' calculation-result--committed' : ''}`} aria-labelledby="result-heading"
      key={result.kind === 'record' ? result.record.id : result.kind}>
      <h2 id="result-heading" className="visually-hidden">{outcome?.kind === 'error' || result.kind === 'failed' ? messages.outcomeLabel.error : messages.outcomeLabel.success}</h2>
      {result.kind === 'loading' && (
        <p className="result-source result-pending">
          {messages.loadingSource} <code>{result.request.expression}</code>
        </p>
      )}
      {result.kind === 'failed' && (
        <div>
          <p className="result-source">
            <code>{result.request.expression}</code>
          </p>
          <p className="result-error">{result.message}</p>
          {result.retryable && (
            <button className="secondary-button" type="button" onClick={() => onRetry(result.request)}>{messages.retry}</button>
          )}
        </div>
      )}
      {result.kind === 'record' && (
        <div>
          <p className="result-source">
            <code>{source}</code>
          </p>
          {result.publication === 'published' && <p className="publication-status">{messages.published}</p>}
          {result.publication === 'unavailable' && <p className="publication-status publication-status--warning">{messages.publicationUnavailable}</p>}
          {outcome?.kind === 'success' ? (
            <>
              <div className="result-answer">
                <p className="result-value"><span>{display?.approximate ? '≈' : '='}</span> {display?.text}</p>
                <button type="button" className="copy-result" title={messages.copyExact} data-copied={copyStatus === 'copied'}
                  aria-label={copyStatus === 'copied' ? messages.copied : messages.copyExact}
                  onClick={() => onCopy(outcome.value)}>
                  <svg width="18" height="18" viewBox="0 0 24 24" fill="none" aria-hidden="true">
                    <path className="copy-icon" d="M9 9h11v11H9zM5 15H3V3h12v2" stroke="currentColor" strokeWidth="2" />
                    <path className="copy-check" d="m4 12 5 5L20 6" stroke="currentColor" strokeWidth="2" />
                  </svg>
                  <span>{copyStatus === 'copied' ? messages.copied : messages.copy}</span>
                </button>
              </div>
              {display?.approximate && (
                <details className="full-value"><summary>{messages.exactValue}</summary><code>{outcome.value}</code></details>
              )}
              {copyStatus === 'failed' && <p className="copy-error">{messages.copyFailed} <code>{outcome.value}</code></p>}
            </>
          ) : outcome?.kind === 'error' ? (
            <>
              <p className="result-error">{mathErrorText(outcome.error, capabilities, messages)}</p>
              <p className="error-stage">{messages.errorStage[outcome.error.stage]}</p>
              <p className="error-advice">{messages.errorAdvice[outcome.error.code] ?? messages.defaultErrorAdvice}</p>
              {span && (
                <code className="result-highlight">
                  {source.slice(0, span.start)}
                  <mark>{source.slice(span.start, span.end) || '│'}</mark>
                  {source.slice(span.end)}
                </code>
              )}
              {(expression !== source || span !== null) && <button type="button" className="secondary-button" onClick={() => onCorrect(result.record)}>
                {expression === source ? messages.goToError : messages.restoreExpression}
              </button>}
            </>
          ) : null}
        </div>
      )}
    </section>
  );
}
