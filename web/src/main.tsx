import { StrictMode } from 'react';
import { createRoot } from 'react-dom/client';
import App from './App';
import '@fontsource/pt-sans/cyrillic-400.css';
import '@fontsource/pt-sans/latin-400.css';
import '@fontsource/pt-sans/cyrillic-700.css';
import '@fontsource/pt-sans/latin-700.css';
import '@fontsource/pt-serif/cyrillic-400.css';
import '@fontsource/pt-serif/latin-400.css';
import '@fontsource/pt-mono/latin-400.css';
import './styles.css';

const root = document.getElementById('root');
if (root === null) {
  throw new Error('Missing root element.');
}

createRoot(root).render(
  <StrictMode>
    <App />
  </StrictMode>,
);
