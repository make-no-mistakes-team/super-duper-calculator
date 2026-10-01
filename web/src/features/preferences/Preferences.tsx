import { useCallback, useEffect, useLayoutEffect, useState } from 'react';
import './preferences.css';

export type Preferences = {
  theme: 'violet' | 'amber';
  humor: boolean;
  largeEffects: boolean;
  soundEnabled: boolean;
};

export type PreferencesPanelProps = {
  preferences: Preferences;
  onChange: (changes: Partial<Preferences>) => void;
  achievementsAvailable: boolean;
  themesAvailable: boolean;
};

const storageKey = 'calculator.preferences.v1';
const defaults: Preferences = { theme: 'violet', humor: true, largeEffects: true, soundEnabled: true };

function readPreferences(): Preferences {
  if (typeof window === 'undefined') return defaults;

  try {
    const stored: unknown = JSON.parse(window.localStorage.getItem(storageKey) ?? 'null');
    if (stored === null || typeof stored !== 'object' || Array.isArray(stored)) return defaults;
    const values = stored as Record<string, unknown>;
    return {
      theme: values.theme === 'amber' || values.theme === 'violet' ? values.theme : defaults.theme,
      humor: typeof values.humor === 'boolean' ? values.humor : defaults.humor,
      largeEffects: typeof values.largeEffects === 'boolean' ? values.largeEffects : defaults.largeEffects,
      soundEnabled: typeof values.soundEnabled === 'boolean' ? values.soundEnabled : defaults.soundEnabled,
    };
  } catch {
    // Browsing without storage must not disable settings for this visit.
    return defaults;
  }
}

export function usePreferences(): {
  preferences: Preferences;
  updatePreferences: (changes: Partial<Preferences>) => void;
} {
  const [preferences, setPreferences] = useState<Preferences>(readPreferences);
  const updatePreferences = useCallback((changes: Partial<Preferences>) => {
    setPreferences((current) => ({ ...current, ...changes }));
  }, []);

  // The document theme is an external browser state, not derived React state.
  // Apply before paint so a saved amber theme does not flash violet on mount.
  useLayoutEffect(() => {
    document.documentElement.dataset.theme = preferences.theme;
  }, [preferences.theme]);

  useEffect(() => {
    try {
      window.localStorage.setItem(storageKey, JSON.stringify(preferences));
    } catch {
      // Memory-only changes remain usable when storage is disabled or full.
    }
  }, [preferences]);

  return { preferences, updatePreferences };
}

export function PreferencesPanel({ preferences, onChange, achievementsAvailable, themesAvailable }: PreferencesPanelProps) {
  return (
    <div className="preferences-panel">
      {themesAvailable && (
        <fieldset className="preferences-group">
          <legend>Тема</legend>
          <div className="preferences-options">
            <label className="preferences-choice">
              <input type="radio" name="calculator-theme" value="violet" checked={preferences.theme === 'violet'}
                onChange={() => onChange({ theme: 'violet' })} />
              <span className="preferences-swatch preferences-swatch--violet" aria-hidden="true" />
              <span>Фиолетовая</span>
            </label>
            <label className="preferences-choice">
              <input type="radio" name="calculator-theme" value="amber" checked={preferences.theme === 'amber'}
                onChange={() => onChange({ theme: 'amber' })} />
              <span className="preferences-swatch preferences-swatch--amber" aria-hidden="true" />
              <span>Янтарная</span>
            </label>
          </div>
        </fieldset>
      )}
      {achievementsAvailable && (
        <fieldset className="preferences-group">
          <legend>Открытия</legend>
          <div className="preferences-options">
            <label className="preferences-choice">
              <input type="checkbox" checked={preferences.humor}
                onChange={(event) => onChange({ humor: event.currentTarget.checked })} />
              <span className="preferences-label">Шутки<small>Реплики и характер калькулятора</small></span>
            </label>
            <label className="preferences-choice">
              <input type="checkbox" checked={preferences.largeEffects}
                onChange={(event) => onChange({ largeEffects: event.currentTarget.checked })} />
              <span className="preferences-label">«Спецэффекты»<small>Живые анимации, праздник достижений и шуточные сцены.</small></span>
            </label>
          </div>
        </fieldset>
      )}
      {!themesAvailable && !achievementsAvailable && <p className="preferences-empty">Настройки пока недоступны.</p>}
    </div>
  );
}
