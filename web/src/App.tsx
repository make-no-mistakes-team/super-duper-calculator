import { useCallback, useEffect, useRef, useState } from 'react';

type HealthState =
  | { kind: 'loading' }
  | { kind: 'success' }
  | { kind: 'failure'; message: string };

export default function App() {
  const [health, setHealth] = useState<HealthState>({ kind: 'loading' });
  const activeRequest = useRef<AbortController | null>(null);

  const checkHealth = useCallback(async () => {
    activeRequest.current?.abort();
    const controller = new AbortController();
    activeRequest.current = controller;
    setHealth({ kind: 'loading' });

    try {
      const response = await fetch('/health/ready', {
        signal: controller.signal,
        cache: 'no-store',
        headers: { Accept: 'application/json' },
      });
      if (!response.ok) {
        throw new Error(`Сервер вернул ошибку HTTP ${response.status}.`);
      }

      const body: unknown = await response.json();
      if (
        typeof body !== 'object' ||
        body === null ||
        !('status' in body) ||
        body.status !== 'ok'
      ) {
        throw new Error('Ответ сервера не содержит ожидаемый статус «ok».');
      }

      if (!controller.signal.aborted) {
        setHealth({ kind: 'success' });
      }
    } catch (error) {
      if (controller.signal.aborted) return;

      const message =
        error instanceof TypeError
          ? 'Не удалось получить ответ. Проверьте, запущен ли сервер.'
          : error instanceof SyntaxError
            ? 'Сервер вернул некорректный JSON.'
            : error instanceof Error
              ? error.message
              : 'Не удалось проверить сервер.';
      setHealth({ kind: 'failure', message });
    }
  }, []);

  useEffect(() => {
    void checkHealth();
    return () => activeRequest.current?.abort();
  }, [checkHealth]);

  return (
    <main className="workspace">
      <header>
        <p className="eyebrow">Среда разработки</p>
        <h1>Супер-дупер калькулятор</h1>
        <p className="intro">
          Проверка запуска и доступности сервера.
        </p>
      </header>

      <section className="health" aria-labelledby="health-heading">
        <div className="section-heading">
          <h2 id="health-heading">Готовность сервера</h2>
          <code>GET /health/ready</code>
        </div>
        <div className="health-result" role="status" aria-atomic="true">
          <p className={`status status--${health.kind}`}>
            <span className="status-dot" aria-hidden="true" />
            {health.kind === 'loading'
              ? 'Проверяем соединение'
              : health.kind === 'success'
                ? 'Сервер готов'
                : 'Проверка не прошла'}
          </p>
          <p className="status-detail">
            {health.kind === 'loading'
              ? 'Ждём ответ на запрос.'
              : health.kind === 'success'
                ? 'Сервер отвечает и имеет доступ к базе данных.'
                : health.message}
          </p>
        </div>
        <button type="button" onClick={() => void checkHealth()}>
          Проверить снова
        </button>
        <p className="hint">
          Статус обновляется при открытии страницы и вручную.
        </p>
      </section>

      <nav className="project-links" aria-label="Материалы проекта">
        <a href="https://github.com/make-no-mistakes-team/super-duper-calculator">
          Репозиторий
        </a>
        <a href="https://github.com/make-no-mistakes-team/super-duper-calculator/blob/main/SPEC_INDEX.md">
          Спецификации
        </a>
      </nav>
    </main>
  );
}
