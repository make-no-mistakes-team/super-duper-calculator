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
  second_wind: '#e0ee92',
  alternate_routes: '#a8ece1',
  quiet_after_storm: '#86a6aa',
  paper_tiger: '#f4ce82',
  gaining_altitude: '#d3dfcf',
  mirror_room: '#54b9bc',
  trouble_collector: '#d99847',
  unscathed: '#f4f3d5',
  grand_scale: '#83e5ff',
  last_pixel: '#a8cf5b',
  parallel_worlds: '#7de8d1',
  time_loop: '#e8bb73',
  fourth_wall: '#b3ddcf',
  unexpected_tail: '#edac61',
};

export function achievementAccent(id: string): CSSProperties {
  return { '--award-accent': accents[id] ?? 'var(--accent)' } as CSSProperties;
}

export function achievementArtwork(id: string) {
  return `/achievements/${encodeURIComponent(id)}.png`;
}
