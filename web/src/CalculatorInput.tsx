import { useLayoutEffect, useRef, useState, type ReactNode, type RefObject } from 'react';
import type { Capabilities } from './contracts';
import type { CalculatorTool, ToolController } from './features/tools/useToolController';

type CalculatorInputProps = {
  expression: string;
  capabilities: Capabilities | null;
  activeTool: CalculatorTool | null;
  toolController: ToolController;
  consoleRef: RefObject<HTMLElement | null>;
  effectsEnabled: boolean;
  onExpressionChange: (expression: string) => void;
  onSubmit: (trusted: boolean) => void;
  history: ReactNode;
  statistics: ReactNode;
  settings: ReactNode;
  children: ReactNode;
};

const arithmeticKeys = ['7', '8', '9', '/', '4', '5', '6', '*', '1', '2', '3', '-', '0', '.', '(', ')', 'pi', 'e', '^', '+', '°'];
const operatorKeys: Record<string, true> = { '/': true, '*': true, '-': true, '+': true, '^': true };
const labels: Record<string, string> = { '/': '÷', '*': '×', pi: 'π' };
const functions = [
  { name: 'sqrt', label: 'Корень', example: 'sqrt(81)' },
  { name: 'abs', label: 'Модуль', example: 'abs(-5)' },
  { name: 'sin', label: 'Синус', example: 'sin(30°)' },
  { name: 'cos', label: 'Косинус', example: 'cos(60°)' },
  { name: 'tan', label: 'Тангенс', example: 'tan(45°)' },
  { name: 'atan', label: 'Арктангенс', example: 'atan(1)' },
  { name: 'asin', label: 'Арксинус', example: 'asin(0.5)' },
  { name: 'acos', label: 'Арккосинус', example: 'acos(0.5)' },
  { name: 'ln', label: 'Логарифм ln', example: 'ln(e)' },
  { name: 'log', label: 'Логарифм log', example: 'log(8, 2)' },
  { name: 'exp', label: 'Экспонента', example: 'exp(2)' },
];
const toolLabels: Record<CalculatorTool, string> = { functions: 'Функции', keypad: 'Клавиатура', history: 'История', statistics: 'Статистика', settings: 'Настройки' };
const tools: CalculatorTool[] = ['functions', 'keypad', 'history', 'statistics'];

export function CalculatorInput({
  expression, capabilities, activeTool, toolController,
  consoleRef, effectsEnabled, onExpressionChange, onSubmit, history, statistics, settings, children,
}: CalculatorInputProps) {
  const inputRef = useRef<HTMLTextAreaElement>(null);
  const submitRef = useRef<HTMLButtonElement>(null);
  const toolBodyRef = useRef<HTMLDivElement>(null);
  const [displayedTool, setDisplayedTool] = useState(activeTool);
  const expressionLimit = capabilities?.limits.expressionLength ?? 1024;
  const nearLimit = expression.length >= expressionLimit * .9;
  const availableFunctions = functions.filter(({ name }) => capabilities === null || name in capabilities.functions);

  useLayoutEffect(() => {
    if (activeTool !== null) {
      setDisplayedTool(activeTool);
      return;
    }
    if (!effectsEnabled || window.matchMedia('(prefers-reduced-motion: reduce)').matches) {
      setDisplayedTool(null);
      return;
    }
    const timer = window.setTimeout(() => setDisplayedTool(null), 190);
    return () => window.clearTimeout(timer);
  }, [activeTool, effectsEnabled]);

  function insert(value: string, cursorOffset = value.length) {
    const input = inputRef.current;
    const start = input?.selectionStart ?? expression.length;
    const end = input?.selectionEnd ?? expression.length;
    const next = expression.slice(0, start) + value + expression.slice(end);
    if (next.length > expressionLimit) return;
    onExpressionChange(next);
    toolController.focusEditor({
      closeTool: false,
      preventScroll: true,
      selection: { start: start + cursorOffset, end: start + cursorOffset },
    });
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
    toolController.focusEditor({ closeTool: false, preventScroll: true, selection: { start, end: start } });
  }

  function submitExpression(trusted: boolean) {
    toolController.prepareSubmit(submitRef.current);
    onSubmit(trusted);
  }

  function renderKey(key: string) {
    return <button key={key} type="button" className={`calculator-key${operatorKeys[key] ? ' calculator-key--operator' : ''}${key === '°' ? ' calculator-key--degree' : ''}`}
      aria-label={`Вставить ${labels[key] ?? key}`} onClick={() => insert(key)}>{labels[key] ?? key}</button>;
  }

  return (
    <div className="console-layout" data-tool-open={displayedTool !== null}>
      <div className="editor-stack">
        <section ref={consoleRef} className="editor-console" aria-label="Калькулятор">
          <div className="editor-topline">
            <svg className="prompt-mark" width="24" height="24" viewBox="0 0 24 24" fill="none" aria-hidden="true">
              <path d="m4 6 6 6-6 6M13 18h7" stroke="currentColor" strokeWidth="2" />
            </svg>
            <div className="editor-options" data-speech-protected>
              <button className="text-button" type="button" disabled={expression === ''} onClick={() => {
                onExpressionChange('');
                toolController.focusEditor({ closeTool: false, preventScroll: true, selection: { start: 0, end: 0 } });
              }}>Очистить</button>
            </div>
          </div>
          <label className="visually-hidden" htmlFor="expression">Выражение</label>
          <textarea ref={inputRef} id="expression" className="calculator-expression" rows={3}
            data-speech-protected
            inputMode={activeTool === 'keypad' ? 'none' : 'text'} spellCheck={false} autoCapitalize="off" autoCorrect="off"
            maxLength={expressionLimit} aria-describedby={nearLimit ? 'expression-hint expression-length' : 'expression-hint'}
            placeholder="sqrt(81) + 2^3" value={expression}
            onChange={(event) => onExpressionChange(event.target.value)}
            onKeyDown={(event) => {
              if (event.key === 'Enter' && !event.shiftKey && !event.nativeEvent.isComposing && event.nativeEvent.keyCode !== 229) {
                event.preventDefault();
                if (expression.trim() !== '') submitExpression(event.isTrusted);
              }
            }} />
          <div className="editor-actions" data-speech-protected>
            <span id="expression-hint" className="input-hint"><kbd>Enter</kbd> вычислить <span>· Shift + Enter — новая строка</span></span>
            {nearLimit && <span id="expression-length" className="expression-length" role="status">{expression.length}/{expressionLimit}</span>}
            <button ref={submitRef} type="button" className="calculator-submit" data-button-cue="none"
              disabled={expression.trim() === ''} onClick={(event) => submitExpression(event.isTrusted)}>
              Вычислить
              <svg width="18" height="18" viewBox="0 0 24 24" fill="none" aria-hidden="true"><path d="M19 5v10H5m5-5-5 5 5 5" stroke="currentColor" strokeWidth="2" /></svg>
            </button>
          </div>
          {children}
        </section>
        <nav className="tool-switches" aria-label="Инструменты калькулятора">
          {tools.map((tool) => <button key={tool} id={`tool-${tool}`} type="button" aria-expanded={activeTool === tool}
            data-button-cue="panel" data-speech-protected
            aria-controls={activeTool === tool ? 'tool-bay' : undefined} onClick={() => toolController.toggle(tool)}>
            <svg width="18" height="18" viewBox="0 0 24 24" fill="none" aria-hidden="true">
              {tool === 'functions' && <path d="M17 5h-4l-3 14H6M7 10h9" stroke="currentColor" strokeWidth="2" />}
              {tool === 'keypad' && <path d="M4 4h16v16H4zM8 8h2m4 0h2M8 12h2m4 0h2M8 16h2m4 0h2" stroke="currentColor" strokeWidth="2" />}
              {tool === 'history' && <path d="M4 10a8 8 0 1 1 1 7M4 4v6h6m2-3v5l3 2" stroke="currentColor" strokeWidth="2" />}
              {tool === 'statistics' && <path d="M4 20h16M6 16V9m6 7V4m6 12v-5" stroke="currentColor" strokeWidth="2" />}
            </svg>
            {toolLabels[tool]}
          </button>)}
        </nav>
      </div>

      {displayedTool !== null && <aside id="tool-bay" className="tool-bay" aria-labelledby="tool-bay-heading"
        data-speech-protected data-closing={activeTool === null} aria-hidden={activeTool === null} inert={activeTool === null}>
        <div className="tool-bay-header">
          <h2 id="tool-bay-heading" tabIndex={-1}>{toolLabels[displayedTool]}</h2>
          <button className="close-tool" type="button" data-button-cue="close" aria-label="Закрыть панель инструментов" title="Закрыть · Esc" onClick={() => toolController.close()}>
            <svg width="18" height="18" viewBox="0 0 24 24" fill="none" aria-hidden="true"><path d="m6 6 12 12M18 6 6 18" stroke="currentColor" strokeWidth="2" /></svg>
          </button>
        </div>
        <div className="tool-bay-body" ref={toolBodyRef} key={displayedTool}>
          {displayedTool === 'history' && history}
          {displayedTool === 'statistics' && statistics}
          {displayedTool === 'settings' && settings}
          {displayedTool === 'keypad' && <>
            <div className="calculator-keys" aria-label="Кнопки калькулятора">{arithmeticKeys.map(renderKey)}</div>
            <div className="keypad-actions">
              <button type="button" className="erase-key" aria-label="Стереть символ" onClick={erase}>Стереть</button>
              <button type="button" className="calculator-submit" data-button-cue="none" disabled={expression.trim() === ''}
                onClick={(event) => submitExpression(event.isTrusted)}>Вычислить</button>
            </div>
          </>}
          {displayedTool === 'functions' && <>
            <div className="function-keys">
              {availableFunctions.map(({ name, label, example }) => <button type="button" key={name} className="function-key"
                title={`${label}: ${example}`} aria-label={`Вставить ${label}: ${example}`} onClick={() => insertFunction(name)}>
                <span>{name}</span><span>{label}</span>
              </button>)}
              {capabilities?.features.remainder && <button type="button" className="function-key" title="Остаток: mod(7, 3)"
                onClick={() => insertFunction('mod')}><span>mod</span><span>Остаток</span></button>}
            </div>
            <div className="syntax-keys" role="group" aria-label="Константы и символы">
              {['pi', 'e', '^', '°', ','].map((key) => <button type="button" key={key} onClick={() => insert(key)}
                aria-label={key === ',' ? 'Запятая' : `Вставить ${labels[key] ?? key}`}>
                {labels[key] ?? key}
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
              <dl><dt>log(8, 2)</dt><dd>Логарифм 8 по основанию 2.</dd>
                <dt>sin(90°) = sin(pi/2)</dt><dd>Тригонометрические функции принимают радианы. Знак ° переводит число или группу в радианы: sin((30+60)°).</dd>
                <dt>asin(1) * 180 / pi</dt><dd>Обратные функции возвращают радианы. Умножение на 180/pi переводит ответ в градусы.</dd></dl>
              <p>До {expressionLimit} символов{capabilities && <>, {capabilities.limits.tokens} токенов, {capabilities.limits.nesting} уровней вложенности</>}.</p>
            </details>
          </>}
        </div>
      </aside>}
    </div>
  );
}
