<script lang="ts">
  import { api } from './api';
  import Login from './Login.svelte';
  import Dashboard from './Dashboard.svelte';
  import Products from './Products.svelte';
  import Stock from './Stock.svelte';
  import Setup from './Setup.svelte';
  import History from './History.svelte';

  const tabs = [['dashboard', 'Dashboard'], ['products', 'Products'], ['stock', 'Stock'], ['history', 'History'], ['setup', 'Categories & locations']];
  let user: { name: string } | null = null;
  let ready = false;
  let tab = 'dashboard';

  api('/me').then((u) => (user = u)).catch(() => {}).finally(() => (ready = true));
  async function logout() {
    await api('/auth/logout', 'POST');
    user = null;
  }
</script>

{#if !ready}
  <main class="muted">Loading…</main>
{:else if !user}
  <Login on:done={(e) => (user = e.detail)} />
{:else}
  <header>
    <strong>Inventory</strong>
    {#each tabs as t}
      <button class:active={tab === t[0]} on:click={() => (tab = t[0])}>{t[1]}</button>
    {/each}
    <button on:click={logout}>Sign out</button>
  </header>
  <main>
    {#if tab === 'dashboard'}<Dashboard />
    {:else if tab === 'products'}<Products />
    {:else if tab === 'stock'}<Stock />
    {:else if tab === 'history'}<History />
    {:else}<Setup />{/if}
  </main>
{/if}
