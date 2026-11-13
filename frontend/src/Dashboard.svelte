<script lang="ts">
  import { onMount, onDestroy } from 'svelte';
  import { driver } from 'driver.js';
  import {
    Package, MapPin, Boxes, CloudUpload, ArrowDownToLine, ArrowUpFromLine,
    SlidersHorizontal, ArrowLeftRight, Inbox, FileSpreadsheet, Download, HelpCircle,
  } from 'lucide-svelte';
  import { msg, downloadExport } from './api';
  import { cached, pending } from './offline';

  const icons: Record<string, any> = { in: ArrowDownToLine, out: ArrowUpFromLine, adjustment: SlidersHorizontal };
  const label: Record<string, string> = { in: 'Masuk', out: 'Keluar', adjustment: 'Penyesuaian' };

  let stats = { products: 0, locations: 0, total_units: 0 };
  let balances: { location: string; quantity: number }[] = [];
  let moves: any[] = [];
  let names: Record<string, string> = {};
  let error = '';

  // export UI state
  let from = '';
  let to = '';
  let exporting = false;
  let exportInfo = '';
  let exportErr = '';

  // default: 90 hari terakhir (selaras backend)
  function defaultRange() {
    const end = new Date();
    const start = new Date();
    start.setDate(end.getDate() - 90);
    to = end.toISOString().slice(0, 10);
    from = start.toISOString().slice(0, 10);
  }
  defaultRange();

  Promise.all([
    cached('/dashboard'),
    cached('/stock/balances?limit=100'),
    cached('/stock/movements?limit=5'),
    cached('/products?limit=100'),
    cached('/locations?limit=100'),
  ])
    .then(([s, b, m, p, l]) => {
      stats = s;
      balances = b;
      moves = m;
      for (const x of p) names['p' + x.id] = x.name;
      for (const x of l) names['l' + x.id] = x.name;
      names = names;
    })
    .catch((e) => (error = msg(e)));

  $: perLocation = Object.entries(
    balances.reduce((a: Record<string, number>, b) => ({ ...a, [b.location]: (a[b.location] ?? 0) + b.quantity }), {}),
  ).sort((a, b) => b[1] - a[1]);
  $: max = Math.max(1, ...perLocation.map(([, q]) => q));

  async function doExport() {
    exportInfo = exportErr = '';
    if (!from || !to) {
      exportErr = 'Isi tanggal mulai dan akhir.';
      return;
    }
    if (from > to) {
      exportErr = 'Tanggal mulai harus sebelum tanggal akhir.';
      return;
    }
    exporting = true;
    try {
      const filename = `inventory-${from}_${to}.xlsx`;
      await downloadExport('/export/inventory', filename, { from, to });
      exportInfo = 'File Excel berhasil diunduh.';
    } catch (e) {
      exportErr = msg(e);
    } finally {
      exporting = false;
    }
  }

  let activeTour: ReturnType<typeof driver> | null = null;
  function startTour() {
    activeTour?.destroy();
    activeTour = driver({
      showProgress: true,
      nextBtnText: 'Lanjut',
      prevBtnText: 'Kembali',
      doneBtnText: 'Selesai',
      progressText: '{{current}} dari {{total}}',
      onDestroyed: () => localStorage.setItem('tour_dashboard_done', '1'),
      steps: [
        {
          element: '[data-tour="dash-stats"]',
          popover: {
            title: 'Ringkasan Inventori',
            description: 'Pantau jumlah produk, lokasi penyimpanan, total unit stok, dan antrean data yang menunggu sinkronisasi.',
            side: 'bottom',
            align: 'start',
          },
        },
        {
          element: '[data-tour="dash-export"]',
          popover: {
            title: 'Export Laporan Excel',
            description: 'Pilih rentang tanggal lalu klik Unduh Excel untuk mengunduh laporan lengkap (Ringkasan Stok, Riwayat, dan Keuangan).',
            side: 'bottom',
            align: 'start',
          },
        },
        {
          element: '[data-tour="dash-locations"]',
          popover: {
            title: 'Stok per Lokasi',
            description: 'Lihat distribusi jumlah stok di setiap gudang atau lokasi penyimpanan.',
            side: 'top',
            align: 'start',
          },
        },
        {
          element: '[data-tour="dash-activity"]',
          popover: {
            title: 'Aktivitas Terbaru',
            description: 'Menampilkan 5 pergerakan stok masuk, keluar, atau penyesuaian terakhir.',
            side: 'top',
            align: 'start',
          },
        },
      ],
    });
    activeTour.drive();
  }

  onMount(() => {
    if (!localStorage.getItem('tour_dashboard_done')) {
      setTimeout(startTour, 300);
    }
  });
  onDestroy(() => activeTour?.destroy());
</script>

{#if error}<p class="err">{error}</p>{/if}

<div class="row" style="justify-content: flex-end; margin: 0 0 12px;">
  <button type="button" class="btn ghost sm" on:click={startTour}>
    <HelpCircle size={16} /> Panduan Beranda
  </button>
</div>

<div class="stats" data-tour="dash-stats">
  <div class="stat g1"><span class="ico"><Package size={22} /></span><b>{stats.products}</b>Produk</div>
  <div class="stat g2"><span class="ico"><MapPin size={22} /></span><b>{stats.locations}</b>Lokasi penyimpanan</div>
  <div class="stat g3"><span class="ico"><Boxes size={22} /></span><b>{stats.total_units}</b>Total unit stok</div>
  <div class="stat g4"><span class="ico"><CloudUpload size={22} /></span><b>{$pending}</b>Menunggu sinkron</div>
</div>

<!-- Export Excel -->
<div class="card" data-tour="dash-export">
  <h3><FileSpreadsheet size={18} /> Export data ke Excel</h3>
  <p class="muted" style="margin:0 0 12px">
    Sheet 1: ringkasan stok saat ini. Sheet 2: riwayat keluar/masuk. Sheet 3: catatan keuangan sesuai periode.
  </p>
  <form on:submit|preventDefault={doExport}>
    <label class="field">
      <span class="muted" style="white-space:nowrap">Dari</span>
      <input type="date" bind:value={from} required aria-label="Tanggal mulai" />
    </label>
    <label class="field">
      <span class="muted" style="white-space:nowrap">Sampai</span>
      <input type="date" bind:value={to} required aria-label="Tanggal akhir" />
    </label>
    <button class="btn" type="submit" disabled={exporting}>
      <Download size={18} />
      {exporting ? 'Menyiapkan…' : 'Unduh Excel'}
    </button>
  </form>
  {#if exportInfo}<p class="note ok" style="margin-top:12px;margin-bottom:0">{exportInfo}</p>{/if}
  {#if exportErr}<p class="err" style="margin-top:12px;margin-bottom:0" role="alert">{exportErr}</p>{/if}
</div>

<div class="cols">
  <div class="card" data-tour="dash-locations">
    <h3><MapPin size={18} /> Stok per lokasi</h3>
    {#each perLocation as [name, qty]}
      <div class="row"><span>{name}</span><b>{qty}</b></div>
      <div class="bar"><span style="width:{(qty / max) * 100}%"></span></div>
    {:else}
      <div class="empty"><Inbox size={30} />Belum ada stok. Catat stok masuk pertama Anda di menu Stok.</div>
    {/each}
  </div>
  <div class="card" data-tour="dash-activity">
    <h3><ArrowLeftRight size={18} /> Aktivitas terbaru</h3>
    {#each moves as m}
      <div class="row">
        <span class="chip {m.type === 'in' ? 'in' : m.type === 'out' ? 'out' : 'adj'}">
          <svelte:component this={icons[m.type]} size={14} />{label[m.type]}
        </span>
        <span class="muted">{new Date(m.created_at).toLocaleDateString('id-ID')}</span>
      </div>
      {#each m.items as i}
        <div class="muted">{i.direction === 'in' ? '+' : '−'}{i.quantity} {names['p' + i.product_id]} di {names['l' + i.location_id]}</div>
      {/each}
    {:else}
      <div class="empty"><Inbox size={30} />Belum ada aktivitas.</div>
    {/each}
  </div>
</div>