import './app.css';
import App from './App.svelte';

new App({ target: document.getElementById('app')! });

if ('serviceWorker' in navigator && import.meta.env.PROD) {
  addEventListener('load', async () => {
    await navigator.serviceWorker.register('/sw.js');
    const reg = await navigator.serviceWorker.ready;
    const urls = performance.getEntriesByType('resource').map((e) => e.name)
      .filter((u) => u.startsWith(location.origin) && !u.includes('/api/'));
    reg.active?.postMessage({ type: 'precache', urls });
  });
}
