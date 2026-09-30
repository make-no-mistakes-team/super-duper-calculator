import type { CSSProperties } from 'react';

// Presentation only. Names, descriptions, secrecy and ordering belong to the server.
const accents: Record<string, string> = {
  answer_found: '#ffe08a',
  six_seven: '#ff9bcc',
  nice_number: '#ffa08a',
  result_found: '#83e5ff',
  peer_review: '#a9f5a0',
  bracket_architect: '#c8b2ff',
  scientific_method: '#7de8d1',
  touch_grass: '#c9ed7b',
};

export function achievementAccent(id: string): CSSProperties {
  return { '--award-accent': accents[id] ?? 'var(--accent)' } as CSSProperties;
}

export function achievementArtwork(id: string) {
  return `/achievements/${encodeURIComponent(id)}.png`;
}
