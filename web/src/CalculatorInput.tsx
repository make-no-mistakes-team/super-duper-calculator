import { useRef, type ReactNode } from 'react';
import type { AngleUnit, Capabilities } from './contracts';

type CalculatorInputProps = {
  expression: string;
  angleUnit: AngleUnit;
  capabilities?: Capabilities | null;
  onExpressionChange: (expression: string) => void;
  onAngleUnitChange: (angleUnit: AngleUnit) => void;
  onSubmit?: () => void;
  children?: ReactNode;
};

const arithmeticKeys = ['7', '8', '9', '/', '4', '5', '6', '*', '1', '2', '3', '-', '0', '.', '(', ')', 'pi', 'e', '^', '+'];
const scientificKeys = ['sqrt(', 'sin(', 'cos(', 'tan(', 'abs(', 'exp(', 'ln(', 'log(', 'asin(', 'acos(', 'atan(', ','];
const operatorKeys = new Set(['/', '*', '-', '+', '^']);

const labels: Record<string, string> = {
  '/': '÷',
  '*': '×',
  pi: 'π',
  'sqrt(': '√',
  'sin(': 'sin',
  'cos(': 'cos',
  'tan(': 'tan',
};

export function CalculatorInput({
  expression,
  angleUnit,
  capabilities,
  onExpressionChange,
  onAngleUnitChange,
  onSubmit,
  children,
}: CalculatorInputProps) {
  const inputRef = useRef<HTMLTextAreaElement>(null);
  const expressionLimit = capabilities?.limits.expressionLength ?? 1024;
  const availableScientificKeys = [
    ...scientificKeys.filter((key) =>
      key === ',' || capabilities === null || capabilities === undefined || key.slice(0, -1) in capabilities.functions),
    ...(capabilities?.features.factorial ? ['!'] : []),
    ...(capabilities?.features.percentage ? ['%'] : []),
    ...(capabilities?.features.remainder ? ['mod('] : []),
  ];

  function focusAt(position: number) {
    requestAnimationFrame(() => {
      inputRef.current?.focus();
      inputRef.current?.setSelectionRange(position, position);
    });
  }

  function insert(value: string) {
    const input = inputRef.current;
    const start = input?.selectionStart ?? expression.length;
    const end = input?.selectionEnd ?? expression.length;
    const next = expression.slice(0, start) + value + expression.slice(end);
    if (next.length > expressionLimit) return;
    onExpressionChange(next);
    focusAt(start + value.length);
  }

  function erase() {
    const input = inputRef.current;
    const end = input?.selectionEnd ?? expression.length;
    let start = input?.selectionStart ?? end;
    if (start === end && start > 0) {
      start--;
      const code = expression.charCodeAt(start);
      if (code >= 0xdc00 && code <= 0xdfff && start > 0) start--;
    }
    onExpressionChange(expression.slice(0, start) + expression.slice(end));
    focusAt(start);
  }

  function renderKey(key: string, scientific = false) {
    return (
      <button
        key={key}
        type="button"
        className={`calculator-key${scientific ? ' calculator-key--scientific' : ''}${operatorKeys.has(key) ? ' calculator-key--operator' : ''}`}
        aria-label={`Вставить ${key}`}
        onClick={() => insert(key)}
      >
        {labels[key] ?? key}
      </button>
    );
  }

  return (
    <section className="calculator-input" aria-labelledby="calculator-input-heading">
      <div className="input-heading-row"><h2 id="calculator-input-heading">Выражение</h2><span>ВВОД / 01</span></div>
      <div className="input-help">
        <label className="calculator-input-label" htmlFor="expression">Введите выражение или используйте клавиши ниже</label>
        <span id="expression-length" aria-label={`Длина выражения: ${expression.length} из ${expressionLimit}`}>{expression.length}/{expressionLimit}</span>
      </div>
      <textarea
        ref={inputRef}
        id="expression"
        className="calculator-expression"
        rows={2}
        inputMode="text"
        spellCheck={false}
        maxLength={expressionLimit}
        aria-describedby={capabilities ? 'expression-length expression-limits' : 'expression-length'}
        value={expression}
        onChange={(event) => onExpressionChange(event.target.value)}
        onKeyDown={(event) => {
          if (event.key === 'Enter' && expression.trim() !== '' && !event.shiftKey
            && !event.nativeEvent.isComposing && event.nativeEvent.keyCode !== 229 && onSubmit) {
            event.preventDefault();
            onSubmit();
          }
        }}
      />
      {capabilities && (
        <p id="expression-limits" className="expression-limits">
          До {capabilities.limits.tokens} токенов · вложенность до {capabilities.limits.nesting} уровней
        </p>
      )}

      <div className="calculator-actions">
        <div className="calculator-settings">
          <label htmlFor="angle-unit">Углы</label>
          <select
            id="angle-unit"
            value={angleUnit}
            onChange={(event) => onAngleUnitChange(event.target.value as AngleUnit)}
          >
            <option value="deg">Градусы</option>
            <option value="rad">Радианы</option>
          </select>
        </div>
        {onSubmit && (
          <button type="button" className="calculator-submit" disabled={expression.trim() === ''} onClick={onSubmit}>
            Вычислить <span aria-hidden="true">↗</span>
          </button>
        )}
      </div>
      {children}

      <div className="keypad-heading"><h2>Клавиши</h2><span>АРИФМЕТИКА</span></div>
      <div className="calculator-keys" aria-label="Кнопки калькулятора">
        {arithmeticKeys.map((key) => renderKey(key))}
      </div>
      <div className="scientific-heading">НАУЧНЫЕ ФУНКЦИИ</div>
      <div className="scientific-keys" aria-label="Научные функции">
        {availableScientificKeys.map((key) => renderKey(key, true))}
      </div>
      <div className="editor-tools">
        <button type="button" onClick={erase}>⌫ <span>Стереть</span></button>
        <button
          type="button"
          onClick={() => {
            onExpressionChange('');
            focusAt(0);
          }}
        >
          Очистить ввод
        </button>
      </div>
    </section>
  );
}
