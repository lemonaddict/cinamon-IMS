<script lang="ts">
  import { onMount } from 'svelte';
  import { LayoutDashboard, Package, ArrowLeftRight, History as IconHistory, RefreshCw, Settings, LogOut, Boxes, Wifi, WifiOff, CloudOff } from 'lucide-svelte';
  import { api } from './api';
  import { online, pending, setUser, sync, clearCache, cached } from './offline';
  import Login from './Login.svelte';
  import Dashboard from './Dashboard.svelte';
  import Products from './Products.svelte';
  import Stock from './Stock.svelte';
  import History from './History.svelte';
  import Sync from './Sync.svelte';
  import Setup from './Setup.svelte';

  // sec = secondary: shown in the sidebar on desktop, moved to the header on phones
  const tabs = [
    { id: 'dashboard', label: 'Beranda', icon: LayoutDashboard, sec: false },
    { id: 'products', label: 'Produk', icon: Package, sec: false },
    { id: 'stock', label: 'Stok', icon: ArrowLeftRight, sec: false },
    { id: 'history', label: 'Riwayat', icon: IconHistory, sec: false },
    { id: 'sync', label: 'Sinkron', icon: RefreshCw, sec: true },
    { id: 'setup', label: 'Pengaturan', icon: Settings, sec: true },
  ];
  const pages: Record<string, any> = { dashboard: Dashboard, products: Products, stock: Stock, history: History, sync: Sync, setup: Setup };

  let user: { id: number; name: string } | null = null;
  let ready = false;
  let tab = 'dashboard';

  function start(u: { id: number; name: string }) {
    user = u;
    setUser(u.id);
    sync();
  }
  onMount(async () => {
    try { start(await cached('/me')); } catch { /* belum masuk */ }
    ready = true;
  });
  async function logout() {
    try { await api('/auth/logout', 'POST'); } catch { /* offline: sesi dihapus saat online berikutnya */ }
    await clearCache();
    user = null;
  }
</script>

{#if !ready}
  <div class="boot muted">Memuat…</div>
{:else if !user}
  <Login on:done={(e) => start(e.detail)} />
{:else}
  <div class="shell">
    <aside class="side">
      <div class="brand"><Boxes size={26} /> Inventori</div>
      <nav>
        {#each tabs as t}
          <button class:on={tab === t.id} class:sec={t.sec} aria-current={tab === t.id ? 'page' : undefined} on:click={() => (tab = t.id)}>
            <svelte:component this={t.icon} size={22} /><span>{t.label}</span>
            {#if t.id === 'sync' && $pending > 0}<em>{$pending}</em>{/if}
          </button>
        {/each}
      </nav>
    </aside>
    <div class="content">
      <header class="top">
        <div>
          <h1>{tabs.find((t) => t.id === tab)?.label}</h1>
          <span class="muted">Halo, {user.name}</span>
        </div>
        <div class="cell">
          <button class="pill" class:off={!$online} on:click={() => (tab = 'sync')} aria-label="Status koneksi dan sinkronisasi">
            {#if $online}<Wifi size={16} /> Online{:else}<WifiOff size={16} /> Offline{/if}
            {#if $pending > 0}<em>{$pending}</em>{/if}
          </button>
          <button class="btn ghost sm icon m-only" aria-label="Pengaturan" on:click={() => (tab = 'setup')}><Settings size={18} /></button>
          <button class="btn ghost sm" aria-label="Keluar" on:click={logout}><LogOut size={16} /><span class="lbl">Keluar</span></button>
        </div>
      </header>
      {#if !$online}
        <div class="note"><CloudOff size={18} /> Mode offline: perubahan stok disimpan di perangkat dan dikirim otomatis saat tersambung.</div>
      {/if}
      <main><svelte:component this={pages[tab]} /></main>
    </div>
  </div>
{/if}
