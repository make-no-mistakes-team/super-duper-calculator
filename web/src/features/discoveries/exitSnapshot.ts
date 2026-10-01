// Preserve a final painted frame, never the live modal/award or its semantics.
// Callers release focus/state immediately and cancel this cleanup on replacement.
export function animateExitSnapshot(element: HTMLElement, className: string, duration: number): () => void {
  const reducedMotion = window.matchMedia('(prefers-reduced-motion: reduce)');
  if (!element.isConnected || document.hidden || reducedMotion.matches) return () => {};
  const rect = element.getBoundingClientRect();
  const appearance = getComputedStyle(element);
  const transform = new DOMMatrixReadOnly(appearance.transform);
  const [originX = 0, originY = 0] = appearance.transformOrigin.split(' ').map(Number.parseFloat);
  const snapshot = document.createElement('div');
  snapshot.className = `${element.className} ${className}`;
  snapshot.style.cssText = element.style.cssText;
  snapshot.inert = true;
  snapshot.setAttribute('aria-hidden', 'true');
  snapshot.setAttribute('data-exit-snapshot', '');
  snapshot.style.position = 'fixed';
  // Rects already include an opening squash/translation. Recover the original
  // layout frame so its current transform isn't applied twice during exit.
  snapshot.style.left = `${rect.left - originX * (1 - transform.a) - transform.e}px`;
  snapshot.style.top = `${rect.top - originY * (1 - transform.d) - transform.f}px`;
  snapshot.style.right = 'auto';
  snapshot.style.bottom = 'auto';
  snapshot.style.margin = '0';
  snapshot.style.width = `${element.offsetWidth}px`;
  snapshot.style.height = `${element.offsetHeight}px`;
  snapshot.style.maxHeight = 'none';
  snapshot.style.pointerEvents = 'none';
  snapshot.style.userSelect = 'none';
  snapshot.style.setProperty('--exit-snapshot-clip', appearance.clipPath);
  snapshot.style.setProperty('--exit-snapshot-transform', appearance.transform);
  snapshot.style.setProperty('--exit-snapshot-opacity', appearance.opacity);
  for (const child of element.childNodes) snapshot.append(child.cloneNode(true));
  // Copy only the visible text/cipher. No second renderer, secret decode,
  // accessibility announcements, duplicate IDs or live award identifiers.
  for (const child of snapshot.querySelectorAll('*')) {
    for (const attribute of [...child.attributes]) {
      if (attribute.name === 'id' || attribute.name === 'role' ||
        attribute.name.startsWith('aria-') || attribute.name === 'autofocus' ||
        attribute.name === 'data-achievement-id' || attribute.name === 'data-speech-protected') {
        child.removeAttribute(attribute.name);
      }
    }
    if (child instanceof HTMLElement && child.matches('button, a, input, select, textarea, [tabindex]')) {
      child.tabIndex = -1;
    }
  }
  element.parentElement?.append(snapshot);
  const originals = element.querySelectorAll('*');
  const copies = snapshot.querySelectorAll('*');
  originals.forEach((original, index) => {
    const copy = copies[index];
    if (!copy) return;
    copy.scrollTop = original.scrollTop;
    copy.scrollLeft = original.scrollLeft;
  });
  let timeout = 0;
  const cleanup = () => {
    window.clearTimeout(timeout);
    window.removeEventListener('resize', cleanup);
    document.removeEventListener('visibilitychange', cleanup);
    reducedMotion.removeEventListener('change', cleanup);
    snapshot.remove();
    snapshot.replaceChildren();
  };
  snapshot.addEventListener('animationend', (event) => {
    if (event.elapsedTime * 1000 >= duration - 1) cleanup();
  });
  snapshot.addEventListener('animationcancel', (event) => {
    if (event.target === snapshot) cleanup();
  });
  window.addEventListener('resize', cleanup);
  document.addEventListener('visibilitychange', cleanup);
  reducedMotion.addEventListener('change', cleanup);
  // CSS can be disabled/interrupted: no closing frame may become an orphan.
  timeout = window.setTimeout(cleanup, duration + 50);
  return cleanup;
}
