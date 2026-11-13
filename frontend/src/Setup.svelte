<script lang="ts">
  import { onMount, onDestroy } from 'svelte';
  import { Tags, MapPin, Plus, Trash2, Inbox, Sun, Moon, Monitor, HelpCircle } from 'lucide-svelte';
  import { api, msg } from './api';
  import { cached } from './offline';
  import { startPageTour, shouldAutoTour } from './tour';

  type Theme = 'light' | 'dark' | 'system';
  const themes: { v: Theme; label: string; icon: any }[] = [
    { v: 'light', label: 'Mode Terang', icon: Sun },
    { v: 'dark', label: 'Mode Gelap', icon: Moon },
    { v: 'system', label: 'Ikuti Sistem', icon: Monitor },
  ];
  const saved = localStorage.getItem('theme');
  let theme: Theme = saved === 'light' || saved === 'dark' ? saved : 'system';

  function setTheme(t: Theme) {
    theme = t;
    if (t === 'system') {
      localStorage.removeItem('theme');
      delete document.documentElement.dataset.theme;
    } else {
      localStorage.setItem('theme', t);
      document.documentElement.dataset.theme = t;
    }
  }

  type Row = { id: number; name: string };
  const kinds = [
    { k: 'categories', title: 'Kategori', add: 'Tambah kategori', empty: 'Belum ada kategori.', icon: Tags },
    { k: 'locations', title: 'Lokasi penyimpanan', add: 'Tambah lokasi', empty: 'Belum ada lokasi. Tambahkan lokasi sebelum mencatat stok.', icon: MapPin },
  ];
  let data: Record<string, Row[]> = { categories: [], locations: [] };
  let names: Record<string, string> = { categories: '', locations: '' };
  let error = '';

  async function load() {
    for (const { k } of kinds) data[k] = await cached(`/${k}?limit=100`);
    data = data;
  }
  async function run(fn: () => Promise<unknown>) {
    error = '';
    try { await fn(); await load(); } catch (e) { error = msg(e); }
  }
  const add = (k: string) => run(async () => { await api(`/${k}`, 'POST', { name: names[k] }); names[k] = ''; });
  const del = (k: string, r: Row) => confirm(`Hapus "${r.name}"?`) && run(() => api(`/${k}/${r.id}`, 'DELETE'));
  load().catch((e) => (error = msg(e)));

  let activeTour: ReturnType<typeof startPageTour> | null = null;
  function startTour() {
    activeTour?.destroy();
    activeTour = startPageTour('tour_setup_done', [
      {
        element: '[data-tour="setup-theme"]',
        popover: {
          title: 'Tampilan Aplikasi',
          description: 'Pilih Mode Terang, Mode Gelap, atau Ikuti Sistem sesuai kenyamanan Anda.',
          side: 'bottom',
          align: 'start',
        },
      },
      {
        element: '[data-tour="setup-categories"]',
        popover: {
          title: 'Kategori Produk',
          description: 'Tambahkan atau kelola kategori untuk mengelompokkan produk Anda.',
          side: 'top',
          align: 'start',
        },
      },
      {
        element: '[data-tour="setup-locations"]',
        popover: {
          title: 'Lokasi Penyimpanan',
          description: 'Tambahkan gudang, toko, atau rak penyimpanan sebelum mencatat pergerakan stok.',
          side: 'top',
          align: 'start',
        },
      },
    ]);
  }

  onMount(() => {
    if (shouldAutoTour('tour_setup_done')) {
      setTimeout(startTour, 300);
    }
  });
  onDestroy(() => activeTour?.destroy());
</script>

{#if error}<p class="err" role="alert">{error}</p>{/if}
<div class="row" style="justify-content: flex-end; margin: 0 0 12px;">
  <button type="button" class="btn ghost sm" on:click={startTour}>
    <HelpCircle size={16} /> Panduan Pengaturan
  </button>
</div>
<div class="card" data-tour="setup-theme">
  <h3><Sun size={18} /> Tampilan</h3>
  <div class="seg one-row" role="radiogroup" aria-label="Pilih tema tampilan" style="margin-bottom: 0;">
    {#each themes as t}
      <button type="button" class:on={theme === t.v} on:click={() => setTheme(t.v)}>
        <svelte:component this={t.icon} size={16} />{t.label}
      </button>
    {/each}
  </div>
</div>
<div class="cols">
  {#each kinds as kind}
    <div class="card" data-tour={'setup-' + kind.k}>
      <h3><svelte:component this={kind.icon} size={18} /> {kind.title}</h3>
      <form on:submit|preventDefault={() => add(kind.k)}>
        <label class="field"><input placeholder="Nama" bind:value={names[kind.k]} required /></label>
        <button class="btn"><Plus size={18} /> {kind.add}</button>
      </form>
      {#each data[kind.k] as row (row.id)}
        <div class="row"><span>{row.name}</span>
          <button class="btn danger sm" on:click={() => del(kind.k, row)} aria-label="Hapus {row.name}"><Trash2 size={14} /> Hapus</button></div>
      {:else}
        <div class="empty"><Inbox size={30} />{kind.empty}</div>
      {/each}
    </div>
  {/each}
</div>
