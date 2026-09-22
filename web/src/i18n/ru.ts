import type { Messages } from './types';

export const ru: Messages = {
  history: {
    title: 'История вычислений',
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