import { defineConfig } from 'vite';
import { svelte } from '@sveltejs/vite-plugin-svelte';

// In dev the browser talks only to :5173; /api is proxied to the Go backend (same-origin cookies).
export default defineConfig({
  plugins: [svelte()],
  server: { port: 5173, proxy: { '/api': 'http://localhost:8080' } },
});
