import { useEffect, useLayoutEffect, useRef, useState, type RefObject } from 'react';

const GLYPHS = '#%?+=/*[]{}';
const CLOSED_PATTERN = '[?] #+% /=*\n%+= [?] #/?';
const CIPHER_POSITIONS = Array.from(CLOSED_PATTERN, (character, index) => character.trim() ? index : -1).filter((index) => index >= 0);
const EMPTY_IDS: readonly string[] = [];
const DECODE_MS = 600;

// The concealed pattern is authored independently of every catalog condition.
// Only two non-spacing glyphs change per tick; its geometry never changes.
function cipherFrame(previous: string, tick: number) {
  const frame = Array.from(previous);
  for (let offset = 0; offset < 2; offset++) {
    const position = CIPHER_POSITIONS[(tick * 2 + offset) % CIPHER_POSITIONS.length];
    if (position === undefined) continue;
    frame[position] = GLYPHS.charAt((tick + position + offset) % GLYPHS.length);
  }
  return frame.join('');
}

export function AchievementDescription({
  text, locked, concealed, active, rootRef, freshAwardIds = EMPTY_IDS, awardId, freshCeremony = false, className,
}: {
  text: string | null;
  locked: boolean;
  concealed: boolean;
  active: boolean;
  rootRef?: RefObject<HTMLDivElement | null>;
  freshAwardIds?: readonly string[];
  awardId: string;
  freshCeremony?: boolean;
  className: string;
}) {
  const element = useRef<HTMLParagraphElement>(null);
  const [visible, setVisible] = useState(false);
  const [foreground, setForeground] = useState(!document.hidden);
  const [reducedMotion, setReducedMotion] = useState(() => matchMedia('(prefers-reduced-motion: reduce)').matches);
  const [cipher, setCipher] = useState(CLOSED_PATTERN);
  const [progress, setProgress] = useState(freshCeremony ? 0 : 1);
  const elapsed = useRef(0);
  const consumedGrant = useRef(false);
  const previous = useRef({ active: false, locked, concealed, visible: false, foreground: !document.hidden });

  useLayoutEffect(() => {
    const node = element.current;
    if (!active || !node) { setVisible(false); return; }
    const observer = new IntersectionObserver(([entry]) => setVisible(Boolean(entry && entry.isIntersecting && entry.intersectionRatio > 0)), {
      root: rootRef?.current ?? null,
      threshold: .01,
    });
    observer.observe(node);
    return () => observer.disconnect();
  }, [active, rootRef]);

  useEffect(() => {
    if (!active) return;
    const motion = matchMedia('(prefers-reduced-motion: reduce)');
    const onVisibility = () => setForeground(!document.hidden);
    const onMotion = () => setReducedMotion(motion.matches);
    onVisibility();
    onMotion();
    document.addEventListener('visibilitychange', onVisibility);
    motion.addEventListener('change', onMotion);
    return () => {
      document.removeEventListener('visibilitychange', onVisibility);
      motion.removeEventListener('change', onMotion);
    };
  }, [active]);

  useLayoutEffect(() => {
    const was = previous.current;
    const fresh = freshAwardIds.includes(awardId);
    // Snapshot every grant, including those arriving while closed/offscreen.
    // Only an already-visible locked card in this same opening can decode.
    if (fresh && !consumedGrant.current) {
      consumedGrant.current = true;
      if (was.active && active && was.locked && was.concealed && !locked && was.visible && was.foreground && !document.hidden) {
        elapsed.current = 0;
        setProgress(reducedMotion ? 1 : 0);
      }
    }
    previous.current = { active, locked, concealed, visible, foreground };
    if (!active || reducedMotion) setProgress(1);
  }, [active, locked, concealed, visible, foreground, reducedMotion, freshAwardIds, awardId]);

  const running = active && visible && foreground && !reducedMotion;
  useEffect(() => {
    if (!concealed || !running) return;
    let tick = 0;
    const timer = window.setInterval(() => {
      if (!document.hidden) setCipher((frame) => cipherFrame(frame, ++tick));
    }, 250);
    return () => window.clearInterval(timer);
  }, [concealed, running]);

  const decoding = progress < 1;
  useEffect(() => {
    if (!decoding || !running) return;
    const started = performance.now();
    const update = () => setProgress(Math.min(1, (elapsed.current + performance.now() - started) / DECODE_MS));
    const interval = window.setInterval(update, 100);
    const completion = window.setTimeout(() => setProgress(1), Math.max(0, DECODE_MS - elapsed.current));
    return () => {
      elapsed.current = Math.min(DECODE_MS, elapsed.current + performance.now() - started);
      window.clearInterval(interval);
      window.clearTimeout(completion);
    };
  }, [decoding, running]);

  const publicText = text ?? '';
  const characters = Array.from(publicText);
  const visualText = concealed ? cipher : decoding
    ? characters.map((character, index) => character.trim() && index >= characters.length * progress ? GLYPHS[index % GLYPHS.length] : character).join('')
    : publicText;

  return (
    <p ref={element} className={`${className}${decoding && !concealed ? ' achievement-description--decoding' : ''}`}>
      {concealed || decoding ? <>
        <span className={concealed ? 'achievement-description__cipher' : undefined} aria-hidden="true">{visualText}</span>
        <span className="visually-hidden">{concealed ? 'Условие пока скрыто' : publicText}</span>
      </> : publicText}
    </p>
  );
}
