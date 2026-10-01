import { StrictMode } from 'react';
import { createRoot } from 'react-dom/client';
import App from './App';
import '@fontsource/tiny5/cyrillic-400.css';
import '@fontsource/tiny5/latin-400.css';
import '@fontsource/pt-mono/latin-400.css';
import '@fontsource/pt-mono/cyrillic-400.css';
import './fonts.css';
import './themes.css';
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
