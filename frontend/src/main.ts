import 'driver.js/dist/driver.css';
import './app.css';
import { mount } from 'svelte';
import App from './App.svelte';

const savedTheme = localStorage.getItem('theme');
if (savedTheme === 'light' || savedTheme === 'dark') {
  document.documentElement.dataset.theme = savedTheme;
}

const target = document.getElementById('app');
if (!target) throw new Error('App mount target #app was not found.');
mount(App, { target });

if ('serviceWorker' in navigator && import.meta.env.PROD) {
  addEventListener('load', async () => {
    await navigator.serviceWorker.register('/sw.js');
    const reg = await navigator.serviceWorker.ready;
    const urls = performance.getEntriesByType('resource').map((e) => e.name)
      .filter((u) => u.startsWith(location.origin) && !u.includes('/api/'));
    reg.active?.postMessage({ type: 'precache', urls });
  });
}
