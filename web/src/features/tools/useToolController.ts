import { useEffect, useRef, type KeyboardEvent } from 'react';
import type { SourceSpan } from '../../contracts';

export type CalculatorTool = 'functions' | 'keypad' | 'history' | 'statistics' | 'settings';

type EditorFocus = {
  selection?: SourceSpan;
  closeTool?: boolean;
  preventScroll?: boolean;
};

export type ToolController = {
  open: (tool: CalculatorTool) => void;
  toggle: (tool: CalculatorTool) => void;
  close: (restoreTrigger?: boolean) => void;
  onKeyDown: (event: KeyboardEvent<HTMLElement>) => void;
  prepareSubmit: (submitButton: HTMLButtonElement | null) => void;
  focusEditor: (options?: EditorFocus) => void;
};

export function useToolController(
  activeTool: CalculatorTool | null,
  onToolChange: (tool: CalculatorTool | null) => void,
  sceneActive: boolean,
): ToolController {
  const pendingFocus = useRef<number | null>(null);

  function cancelFocus() {
    if (pendingFocus.current !== null) cancelAnimationFrame(pendingFocus.current);
    pendingFocus.current = null;
  }

  useEffect(() => () => {
    if (pendingFocus.current !== null) cancelAnimationFrame(pendingFocus.current);
  }, []);

  function afterRender(focus: () => void) {
    cancelFocus();
    pendingFocus.current = requestAnimationFrame(() => {
      pendingFocus.current = null;
      focus();
    });
  }

  function close(restoreTrigger = true) {
    cancelFocus();
    const trigger = activeTool === null ? null : document.getElementById(`tool-${activeTool}`);
    onToolChange(null);
    if (restoreTrigger) trigger?.focus({ preventScroll: true });
  }

  function open(tool: CalculatorTool) {
    onToolChange(tool);
    afterRender(() => document.getElementById('tool-bay-heading')?.focus({ preventScroll: true }));
  }

  function toggle(tool: CalculatorTool) {
    if (activeTool === tool) close();
    else open(tool);
  }

  function onKeyDown(event: KeyboardEvent<HTMLElement>) {
    if (event.defaultPrevented || sceneActive || event.key !== 'Escape' || event.nativeEvent.isComposing || event.nativeEvent.keyCode === 229 || activeTool === null) return;
    event.preventDefault();
    close();
  }

  function prepareSubmit(submitButton: HTMLButtonElement | null) {
    if (!window.matchMedia('(max-width: 900px)').matches) return;
    const toolHadFocus = document.getElementById('tool-bay')?.contains(document.activeElement);
    close(false);
    if (toolHadFocus) afterRender(() => submitButton?.focus({ preventScroll: true }));
  }

  function focusEditor({ selection, closeTool = true, preventScroll = false }: EditorFocus = {}) {
    if (closeTool) close(false);
    afterRender(() => {
      const input = document.getElementById('expression');
      if (!(input instanceof HTMLTextAreaElement)) return;
      input.focus({ preventScroll });
      if (selection) input.setSelectionRange(selection.start, selection.end);
    });
  }

  return { open, toggle, close, onKeyDown, prepareSubmit, focusEditor };
}
