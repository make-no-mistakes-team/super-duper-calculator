import type { Achievement, DiscoveryDefinition } from '../../contracts';
import './discoveries.css';

export type AchievementCollectionProps = {
  catalog: DiscoveryDefinition[];
  achievements: Achievement[];
  available: boolean;
  loading: boolean;
  onRetry: () => void;
  open: boolean;
  onToggle: (open: boolean) => void;
};

const earnedDate = new Intl.DateTimeFormat('ru-RU', {
  day: 'numeric',
  month: 'long',
  year: 'numeric',
  hour: '2-digit',
  minute: '2-digit',
});

export function AchievementCollection({
  catalog,
  achievements,
  available,
  loading,
  onRetry,
  open,
  onToggle,
}: AchievementCollectionProps) {
  const earnedById = new Map(achievements.map((achievement) => [achievement.id, achievement.earnedAt]));
  const earnedCount = catalog.filter((definition) => earnedById.has(definition.id)).length;

  return (
    <details
      id="achievement-collection"
      className="achievement-collection"
      open={open}
      onToggle={(event) => onToggle(event.currentTarget.open)}
    >
      <summary className="achievement-collection__summary">
        <span>Достижения</span>
        {available && catalog.length > 0 && (
          <span className="achievement-collection__count" aria-label={`Получено ${earnedCount} из ${catalog.length}`}>
            {earnedCount} / {catalog.length}
          </span>
        )}
      </summary>

      {!available || catalog.length === 0 ? (
        <div className="achievement-collection__unavailable">
          <p>
            {loading
              ? 'Загружаем коллекцию…'
              : !available
              ? 'Коллекция сейчас недоступна. Полученные достижения нельзя проверить.'
              : 'Каталог достижений не загрузился.'}
          </p>
          <button type="button" disabled={loading} onClick={onRetry}>Повторить загрузку</button>
        </div>
      ) : (
        <ul className="achievement-collection__list">
          {catalog.map((definition) => {
            const earnedAt = earnedById.get(definition.id);
            const earnedTime = earnedAt === undefined ? null : new Date(earnedAt);
            const earnedDateText = earnedTime !== null && Number.isFinite(earnedTime.getTime())
              ? earnedDate.format(earnedTime)
              : null;
            return (
              <li className="achievement-collection__item" key={definition.id}>
                <span className="achievement-collection__name">{definition.ru.name}</span>
                <p className="achievement-collection__description">{definition.ru.description}</p>
                <span className={`achievement-collection__state${earnedAt === undefined ? '' : ' achievement-collection__state--earned'}`}>
                  {earnedAt === undefined
                    ? 'Не получено'
                    : earnedDateText !== null
                      ? <>Получено <time dateTime={earnedAt}>{earnedDateText}</time></>
                      : 'Получено · дата недоступна'}
                </span>
              </li>
            );
          })}
        </ul>
      )}
    </details>
  );
}
