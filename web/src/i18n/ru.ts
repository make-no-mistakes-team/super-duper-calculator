import type { Messages } from './types';

export const ru: Messages = {
  reduction: {
    heading: 'Шаги вычисления',
    action: 'Показать вычисление',
    record: 'Сохранённое выражение',
    loading: 'Загружаем шаги…',
    unavailable: 'Не удалось показать шаги для этой записи. Сохранённый результат остаётся в истории.',
    step: 'Шаг',
    next: 'Следующий шаг',
    skip: 'К ответу',
    close: 'Закрыть',
    replay: 'Повторить показ',
    final: 'Точное значение',
    noSteps: 'Это число уже является результатом: вычислительных шагов нет.',
  },
  history: {
    title: 'История вычислений',
    demo: {
      title: 'Демонстрация истории',
      languageLabel: 'Язык',
      scenarioLabel: 'Сценарий',
      scenarios: { normal: 'Обычная', empty: 'Пустая', error: 'Ошибка чтения' },
    },
    loading: 'Загрузка истории…',
    empty: 'Пока нет сохранённых вычислений',
    loadMore: 'Загрузить ещё',
    readError:
      'Не удалось загрузить историю. Основной калькулятор продолжает работать.',
    unsupported: 'Эта операция больше недоступна для новых вычислений',
    restoredLabel: 'В редактор восстановлено',
    angleUnitLabel: { deg: 'градусы', rad: 'радианы' },
    outcomeLabel: { success: 'Результат', error: 'Ошибка' },
    mathErrors: {
      SYNTAX_ERROR: 'Синтаксическая ошибка',
      UNKNOWN_IDENTIFIER: 'Неизвестная функция или константа',
      UNSUPPORTED_FEATURE: 'Операция недоступна',
      WRONG_ARITY: 'Неверное число аргументов',
      DIVISION_BY_ZERO: 'Деление на ноль',
      DOMAIN_ERROR: 'Значение вне области определения',
      NUMERIC_OVERFLOW: 'Числовое переполнение',
    },
    mathErrorUnknown: 'Математическая ошибка',
  },
};
