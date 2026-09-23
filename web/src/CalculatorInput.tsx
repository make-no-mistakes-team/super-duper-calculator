import { useRef, type ReactNode } from 'react';
import type { AngleUnit } from './contracts';

type CalculatorInputProps = {
  expression: string;
  angleUnit: AngleUnit;
  onExpressionChange: (expression: string) => void;
  onAngleUnitChange: (angleUnit: AngleUnit) => void;
  onSubmit?: () => void;
  children?: ReactNode;
};

const keys = [
  ['7', '8', '9', '/', 'sqrt('],
  ['4', '5', '6', '*', 'sin('],
  ['1', '2', '3', '-', 'cos('],
  ['0', '.', '(', ')', 'tan('],
  ['pi', 'e', '^', '+', ','],
  ['abs(', 'exp(', 'ln(', 'log(', 'asin('],
  ['acos(', 'atan('],
];

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
  onExpressionChange,
  onAngleUnitChange,
  onSubmit,
  children,
}: CalculatorInputProps) {
  const inputRef = useRef<HTMLInputElement>(null);

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
    if (next.length > 1024) return;
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

  return (
    <section className="calculator-input" aria-labelledby="calculator-input-heading">
      <h2 id="calculator-input-heading">Выражение</h2>
      <label className="calculator-input-label" htmlFor="expression">
        Введите выражение или используйте кнопки
      </label>
      <input
        ref={inputRef}
        id="expression"
        className="calculator-expression"
        type="text"
        inputMode="text"
        autoComplete="off"
        spellCheck={false}
        maxLength={1024}
        value={expression}
        onChange={(event) => onExpressionChange(event.target.value)}
        onKeyDown={(event) => {
          if (event.key === 'Enter' && onSubmit) {
            event.preventDefault();
            onSubmit();
          }
        }}
      />

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
        <button type="button" className="calculator-submit" onClick={onSubmit}>
          Вычислить
        </button>
      )}
      {children}

      <div className="calculator-keys" aria-label="Кнопки калькулятора">
        {keys.flat().map((key) => (
          <button
            key={key}
            type="button"
            className="calculator-key"
            aria-label={`Вставить ${key}`}
            onClick={() => insert(key)}
          >
            {labels[key] ?? key}
          </button>
        ))}
        <button type="button" className="calculator-key calculator-key--wide" onClick={erase}>
          Стереть
        </button>
        <button
          type="button"
          className="calculator-key calculator-key--wide"
          onClick={() => {
            onExpressionChange('');
            focusAt(0);
          }}
        >
          Очистить
        </button>
      </div>
    </section>
  );
}
