import type { Language, Messages } from './types';
import { ru } from './ru';
import { en } from './en';

export const catalogs: Record<Language, Messages> = { ru, en };

export function getMessages(language: Language): Messages {
  return catalogs[language];
}

export type { Language, Messages } from './types';