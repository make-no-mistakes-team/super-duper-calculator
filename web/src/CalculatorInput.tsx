import { useRef, type ReactNode } from 'react';
import type { AngleUnit, Capabilities } from './contracts';

type Tool = 'functions' | 'keypad' | 'history' | 'settings';

type CalculatorInputProps = {
  expression: string;
  angleUnit: AngleUnit;
  capabilities: Capabilities | null;
  activeTool: Tool | null;
  onToolChange: (tool: Tool | null) => void;
  onExpressionChange: (expression: string) => void;
  onAngleUnitChange: (angleUnit: AngleUnit) => void;
  onSubmit: () => void;
  history: ReactNode;
  settings: ReactNode;
  children: ReactNode;
};

const arithmeticKeys = ['7', '8', '9', '/', '4', '5', '6', '*', '1', '2', '3', '-', '0', '.', '(', ')', 'pi', 'e', '^', '+'];
const operatorKeys: Record<string, true> = { '/': true, '*': true, '-': true, '+': true, '^': true };
const labels: Record<string, string> = { '/': '÷', '*': '×', pi: 'π' };
const functions = [
  { name: 'sqrt', label: 'Корень', example: 'sqrt(81)' },
  { name: 'abs', label: 'Модуль', example: 'abs(-5)' },
  { name: 'sin', label: 'Синус', example: 'sin(30)' },
  { name: 'cos', label: 'Косинус', example: 'cos(60)' },
  { name: 'tan', label: 'Тангенс', example: 'tan(45)' },
  { name: 'atan', label: 'Арктангенс', example: 'atan(1)' },
  { name: 'asin', label: 'Арксинус', example: 'asin(0.5)' },
  { name: 'acos', label: 'Арккосинус', example: 'acos(0.5)' },
  { name: 'ln', label: 'Логарифм ln', example: 'ln(e)' },
  { name: 'log', label: 'Логарифм log', example: 'log(8, 2)' },
  { name: 'exp', label: 'Экспонента', example: 'exp(2)' },
];
const toolLabels: Record<Tool, string> = { functions: 'Функции', keypad: 'Клавиатура', history: 'История', settings: 'Настройки' };
const tools: Tool[] = ['functions', 'keypad', 'history'];

export function CalculatorInput({
  expression, angleUnit, capabilities, activeTool, onToolChange,
  onExpressionChange, onAngleUnitChange, onSubmit, history, settings, children,
}: CalculatorInputProps) {
  const inputRef = useRef<HTMLTextAreaElement>(null);
  const submitRef = useRef<HTMLButtonElement>(null);
  const toolBodyRef = useRef<HTMLDivElement>(null);
  const expressionLimit = capabilities?.limits.expressionLength ?? 1024;
  const nearLimit = expression.length >= expressionLimit * .9;
  const availableFunctions = functions.filter(({ name }) => capabilities === null || name in capabilities.functions);

  function focusAt(position: number) {
    requestAnimationFrame(() => {
      inputRef.current?.focus({ preventScroll: true });
      inputRef.current?.setSelectionRange(position, position);
    });
  }

  function insert(value: string, cursorOffset = value.length) {
    const input = inputRef.current;
    const start = input?.selectionStart ?? expression.length;
    const end = input?.selectionEnd ?? expression.length;
    const next = expression.slice(0, start) + value + expression.slice(end);
    if (next.length > expressionLimit) return;
    onExpressionChange(next);
    focusAt(start + cursorOffset);
  }

  function insertFunction(name: string) {
    const input = inputRef.current;
    const selected = expression.slice(input?.selectionStart ?? expression.length, input?.selectionEnd ?? expression.length);
    const value = `${name}(${selected})`;
    insert(value, value.length - 1);
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

  function closeTool() {
    const trigger = document.getElementById(`tool-${activeTool}`);
    onToolChange(null);
    trigger?.focus({ preventScroll: true });
  }

  function openTool(tool: Tool) {
    if (activeTool === tool) { closeTool(); return; }
    onToolChange(tool);
    requestAnimationFrame(() => document.getElementById('tool-bay-heading')?.focus({ preventScroll: true }));
  }

  function submitExpression() {
    if (window.matchMedia('(max-width: 900px)').matches) {
      const toolHadFocus = document.getElementById('tool-bay')?.contains(document.activeElement);
      onToolChange(null);
      if (toolHadFocus) requestAnimationFrame(() => submitRef.current?.focus({ preventScroll: true }));
    }
    onSubmit();
  }

  function renderKey(key: string) {
    return <button key={key} type="button" className={`calculator-key${operatorKeys[key] ? ' calculator-key--operator' : ''}`}
      aria-label={`Вставить ${labels[key] ?? key}`} onClick={() => insert(key)}>{labels[key] ?? key}</button>;
  }

  return (
    <div className="console-layout" data-tool-open={activeTool !== null} onKeyDown={(event) => {
      if (event.key === 'Escape' && !event.nativeEvent.isComposing && event.nativeEvent.keyCode !== 229 && activeTool !== null) {
        event.preventDefault();
        closeTool();
      }
    }}>
      <div className="editor-stack">
        <section className="editor-console" aria-label="Калькулятор">
          <div className="editor-topline">
            <svg className="prompt-mark" width="24" height="24" viewBox="0 0 24 24" fill="none" aria-hidden="true">
              <path d="m4 6 6 6-6 6M13 18h7" stroke="currentColor" strokeWidth="2" />
            </svg>
            <div className="editor-options">
              <label className="visually-hidden" htmlFor="angle-unit">Единицы углов</label>
              <select id="angle-unit" value={angleUnit} title="Единицы углов: DEG — градусы, RAD — радианы"
                onChange={(event) => onAngleUnitChange(event.target.value as AngleUnit)}>
                <option value="deg">DEG</option><option value="rad">RAD</option>
              </select>
              <button className="text-button" type="button" disabled={expression === ''} onClick={() => {
                onExpressionChange(''); focusAt(0);
              }}>Очистить</button>
            </div>
          </div>
          <label className="visually-hidden" htmlFor="expression">Выражение</label>
          <textarea ref={inputRef} id="expression" className="calculator-expression" rows={3}
            inputMode={activeTool === 'keypad' ? 'none' : 'text'} spellCheck={false} autoCapitalize="off" autoCorrect="off"
            maxLength={expressionLimit} aria-describedby={nearLimit ? 'expression-hint expression-length' : 'expression-hint'}
            placeholder="sqrt(81) + 2^3" value={expression}
            onChange={(event) => onExpressionChange(event.target.value)}
            onKeyDown={(event) => {
              if (event.key === 'Enter' && !event.shiftKey && !event.nativeEvent.isComposing && event.nativeEvent.keyCode !== 229) {
                event.preventDefault();
                if (expression.trim() !== '') submitExpression();
              }
            }} />
          <div className="editor-actions">
            <span id="expression-hint" className="input-hint"><kbd>Enter</kbd> вычислить <span>· Shift + Enter — новая строка</span></span>
            {nearLimit && <span id="expression-length" className="expression-length" role="status">{expression.length}/{expressionLimit}</span>}
            <button ref={submitRef} type="button" className="calculator-submit" disabled={expression.trim() === ''} onClick={submitExpression}>
              Вычислить
              <svg width="18" height="18" viewBox="0 0 24 24" fill="none" aria-hidden="true"><path d="M19 5v10H5m5-5-5 5 5 5" stroke="currentColor" strokeWidth="2" /></svg>
            </button>
          </div>
          {children}
        </section>
        <nav className="tool-switches" aria-label="Инструменты калькулятора">
          {tools.map((tool) => <button key={tool} id={`tool-${tool}`} type="button" aria-expanded={activeTool === tool}
            aria-controls={activeTool === tool ? 'tool-bay' : undefined} onClick={() => openTool(tool)}>
            <svg width="18" height="18" viewBox="0 0 24 24" fill="none" aria-hidden="true">
              {tool === 'functions' && <path d="M17 5h-4l-3 14H6M7 10h9" stroke="currentColor" strokeWidth="2" />}
              {tool === 'keypad' && <path d="M4 4h16v16H4zM8 8h2m4 0h2M8 12h2m4 0h2M8 16h2m4 0h2" stroke="currentColor" strokeWidth="2" />}
              {tool === 'history' && <path d="M4 10a8 8 0 1 1 1 7M4 4v6h6m2-3v5l3 2" stroke="currentColor" strokeWidth="2" />}
            </svg>
            {toolLabels[tool]}
          </button>)}
        </nav>
      </div>

      {activeTool !== null && <aside id="tool-bay" className="tool-bay" aria-labelledby="tool-bay-heading">
        <div className="tool-bay-header">
          <h2 id="tool-bay-heading" tabIndex={-1}>{toolLabels[activeTool]}</h2>
          <button className="close-tool" type="button" aria-label="Закрыть панель инструментов" title="Закрыть · Esc" onClick={closeTool}>
            <svg width="18" height="18" viewBox="0 0 24 24" fill="none" aria-hidden="true"><path d="m6 6 12 12M18 6 6 18" stroke="currentColor" strokeWidth="2" /></svg>
          </button>
        </div>
        <div className="tool-bay-body" ref={toolBodyRef} key={activeTool}>
          {activeTool === 'history' && history}
          {activeTool === 'settings' && settings}
          {activeTool === 'keypad' && <>
            <div className="calculator-keys" aria-label="Кнопки калькулятора">{arithmeticKeys.map(renderKey)}</div>
            <div className="keypad-actions">
              <button type="button" className="erase-key" aria-label="Стереть символ" onClick={erase}>Стереть</button>
              <button type="button" className="calculator-submit" disabled={expression.trim() === ''} onClick={submitExpression}>Вычислить</button>
            </div>
          </>}
          {activeTool === 'functions' && <>
            <div className="function-keys">
              {availableFunctions.map(({ name, label, example }) => <button type="button" key={name} className="function-key"
                title={`${label}: ${example}`} aria-label={`Вставить ${label}: ${example}`} onClick={() => insertFunction(name)}>
                <span>{name}</span><span>{label}</span>
              </button>)}
              {capabilities?.features.remainder && <button type="button" className="function-key" title="Остаток: mod(7, 3)"
                onClick={() => insertFunction('mod')}><span>mod</span><span>Остаток</span></button>}
            </div>
            <div className="syntax-keys" aria-label="Константы и аргументы">
              {['pi', 'e', '^', ','].map((key) => <button type="button" key={key} onClick={() => insert(key)}
                aria-label={key === ',' ? 'Запятая между аргументами' : `Вставить ${labels[key] ?? key}`}>
                {key === ',' ? <><span>,</span> аргумент</> : labels[key] ?? key}
              </button>)}
              {capabilities?.features.factorial && <button type="button" onClick={() => insert('!')} aria-label="Вставить факториал">!</button>}
              {capabilities?.features.percentage && <button type="button" onClick={() => insert('%')} aria-label="Вставить проценты">%</button>}
            </div>
            <details className="syntax-help" onToggle={(event) => {
              const help = event.currentTarget;
              const panel = toolBodyRef.current;
              if (!help.open || panel === null) return;
              panel.scrollTo({
                top: panel.scrollTop + help.getBoundingClientRect().top - panel.getBoundingClientRect().top - 12,
                behavior: window.matchMedia('(prefers-reduced-motion: reduce)').matches ? 'instant' : 'smooth',
              });
            }}>
              <summary>Синтаксис и ограничения</summary>
              <p>Выделите часть выражения, чтобы заключить её в функцию.</p>
              <dl><dt>log(8, 2)</dt><dd>Логарифм 8 по основанию 2. Запятая разделяет аргументы.</dd>
                <dt>sin(30)</dt><dd>Угловые функции используют выбранный режим DEG или RAD.</dd></dl>
              <p>До {expressionLimit} символов{capabilities && <>, {capabilities.limits.tokens} токенов, {capabilities.limits.nesting} уровней вложенности</>}.</p>
            </details>
          </>}
        </div>
      </aside>}
    </div>
  );
}
