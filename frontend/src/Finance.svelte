<script lang="ts">
  import { onMount, onDestroy } from 'svelte';
  import { Wallet, ArrowDownToLine, ArrowUpFromLine, Plus, Trash2, Search, Calendar, Tag, Banknote, Inbox, HelpCircle } from 'lucide-svelte';
  import { api, msg, idr } from './api';
  import { cached } from './offline';
  import Select from './Select.svelte';
  import Pagination from './Pagination.svelte';
  import { startPageTour, shouldAutoTour } from './tour';

  type Entry = {
    id: number;
    entry_type: 'income' | 'expense';
    category: string;
    amount: number;
    entry_date: string;
    notes: string;
  };
  type Summary = { total_income: number; total_expense: number; balance: number };

  const today = () => new Date().toISOString().slice(0, 10);
  const presets: Record<'income' | 'expense', string[]> = {
    income: ['Penjualan', 'Modal', 'Piutang Masuk', 'Lainnya'],
    expense: ['Pembelian Stok', 'Operasional', 'Gaji', 'Listrik & Internet', 'Sewa', 'Lainnya'],
  };

  const blank = () => ({
    entry_type: 'income' as 'income' | 'expense',
    selectedCat: '',
    customCat: '',
    amount: '' as string | number,
    entry_date: today(),
    notes: '',
  });

  let entries: Entry[] = [];
  let summary: Summary = { total_income: 0, total_expense: 0, balance: 0 };
  let f = blank();
  let filterType: '' | 'income' | 'expense' = '';
  let q = '';
  let page = 1;
  const perPage = 10;
  let error = '';

  async function load() {
    const [list, sum] = await Promise.all([
      cached<Entry[]>('/finance?limit=100'),
      cached<Summary>('/finance/summary'),
    ]);
    entries = list;
    summary = sum;
  }

  async function run(fn: () => Promise<unknown>) {
    error = '';
    try {
      await fn();
      await load();
    } catch (e) {
      error = msg(e);
    }
  }

  function switchType(t: 'income' | 'expense') {
    f.entry_type = t;
    if (f.selectedCat && !presets[t].includes(f.selectedCat)) {
      f.selectedCat = '';
    }
  }

  const add = () =>
    run(async () => {
      const category = f.customCat.trim() || f.selectedCat;
      if (!category) throw new Error('Pilih atau tulis kategori terlebih dahulu.');
      await api('/finance', 'POST', {
        entry_type: f.entry_type,
        category,
        amount: Number(f.amount) || 0,
        entry_date: f.entry_date,
        notes: f.notes,
      });
      const prevType = f.entry_type;
      f = blank();
      switchType(prevType);
    });

  const del = (e: Entry) =>
    confirm(`Hapus catatan "${e.category}" (${idr(e.amount)})?`) &&
    run(() => api(`/finance/${e.id}`, 'DELETE'));

  $: catOptions = presets[f.entry_type].map((c) => ({ value: c, label: c }));
  $: shown = entries.filter((e) => {
    if (filterType && e.entry_type !== filterType) return false;
    if (!q.trim()) return true;
    const term = q.toLowerCase();
    return (e.category + ' ' + e.notes + ' ' + e.entry_date).toLowerCase().includes(term);
  });
  $: if (filterType !== undefined || q !== undefined) page = 1;
  $: paged = shown.slice((page - 1) * perPage, page * perPage);

  load().catch((e) => (error = msg(e)));

  let activeTour: ReturnType<typeof startPageTour> | null = null;
  function startTour() {
    activeTour?.destroy();
    activeTour = startPageTour('tour_finance_done', [
      {
        element: '[data-tour="fin-stats"]',
        popover: {
          title: 'Ringkasan Keuangan',
          description: 'Pantau total pemasukan, total pengeluaran, dan saldo bersih secara otomatis.',
          side: 'bottom',
          align: 'start',
        },
      },
      {
        element: '[data-tour="fin-form"]',
        popover: {
          title: 'Catat Transaksi Keuangan',
          description: 'Pilih Pemasukan atau Pengeluaran, tentukan tanggal, pilih atau tulis kategori, isi nominal, lalu simpan.',
          side: 'bottom',
          align: 'start',
        },
      },
      {
        element: '[data-tour="fin-table"]',
        popover: {
          title: 'Daftar Catatan Keuangan',
          description: 'Filter berdasarkan jenis transaksi atau cari catatan. Stok masuk dan keluar juga tercatat otomatis di sini.',
          side: 'top',
          align: 'start',
        },
      },
    ]);
  }

  onMount(() => {
    if (shouldAutoTour('tour_finance_done')) {
      setTimeout(startTour, 300);
    }
  });
  onDestroy(() => activeTour?.destroy());
</script>

<div class="row" style="justify-content: flex-end; margin: 0 0 12px;">
  <button type="button" class="btn ghost sm" on:click={startTour}>
    <HelpCircle size={16} /> Panduan Keuangan
  </button>
</div>

<div class="stats" data-tour="fin-stats">
  <div class="stat g2">
    <span class="ico"><ArrowDownToLine size={22} /></span>
    <b>{idr(summary.total_income)}</b>Total Pemasukan
  </div>
  <div class="stat g3">
    <span class="ico"><ArrowUpFromLine size={22} /></span>
    <b>{idr(summary.total_expense)}</b>Total Pengeluaran
  </div>
  <div class="stat g1">
    <span class="ico"><Wallet size={22} /></span>
    <b>{idr(summary.balance)}</b>Saldo Bersih
  </div>
</div>

<div class="card" data-tour="fin-form">
  <h3><Plus size={18} /> Catat transaksi keuangan</h3>
  <div class="seg one-row" role="tablist" aria-label="Jenis transaksi">
    <button type="button" class:on={f.entry_type === 'income'} on:click={() => switchType('income')}>
      <ArrowDownToLine size={16} /> Pemasukan
    </button>
    <button type="button" class:on={f.entry_type === 'expense'} on:click={() => switchType('expense')}>
      <ArrowUpFromLine size={16} /> Pengeluaran
    </button>
  </div>
  <form on:submit|preventDefault={add}>
    <label class="field"><Calendar size={18} /><input type="date" bind:value={f.entry_date} required /></label>
    <Select bind:value={f.selectedCat} label="Kategori" placeholder="Pilih kategori" searchable options={catOptions} />
    <label class="field"><Tag size={18} /><input placeholder="Atau tulis kategori" bind:value={f.customCat} /></label>
    <label class="field"><Banknote size={18} /><input type="number" min="1" placeholder="Nominal (Rp)" bind:value={f.amount} required /></label>
    <label class="field"><input placeholder="Keterangan (opsional)" bind:value={f.notes} /></label>
    <button class="btn"><Plus size={18} /> Simpan transaksi</button>
  </form>
  {#if error}<p class="err" role="alert">{error}</p>{/if}
</div>

<div class="card" data-tour="fin-table">
  <div class="row" style="margin: 0 0 12px; flex-wrap: wrap;">
    <div class="seg one-row" style="margin: 0; flex: 1 1 280px;">
      <button type="button" class:on={filterType === ''} on:click={() => (filterType = '')}>Semua</button>
      <button type="button" class:on={filterType === 'income'} on:click={() => (filterType = 'income')}>Pemasukan</button>
      <button type="button" class:on={filterType === 'expense'} on:click={() => (filterType = 'expense')}>Pengeluaran</button>
    </div>
    <label class="field" style="flex: 1 1 220px;"><Search size={18} /><input placeholder="Cari kategori atau keterangan" bind:value={q} /></label>
  </div>

  <div class="wrap">
    <table>
      <thead><tr><th>Tanggal</th><th>Jenis</th><th>Kategori</th><th>Keterangan</th><th>Nominal</th><th></th></tr></thead>
      <tbody>
        {#each paged as e (e.id)}
          <tr>
            <td>{new Date(e.entry_date + 'T00:00:00').toLocaleDateString('id-ID')}</td>
            <td>
              {#if e.entry_type === 'income'}
                <span class="chip in"><ArrowDownToLine size={14} /> Pemasukan</span>
              {:else}
                <span class="chip out"><ArrowUpFromLine size={14} /> Pengeluaran</span>
              {/if}
            </td>
            <td>{e.category}</td>
            <td class="muted">{e.notes || '—'}</td>
            <td><b>{e.entry_type === 'income' ? '+' : '−'}{idr(e.amount)}</b></td>
            <td><button class="btn danger sm" on:click={() => del(e)} aria-label="Hapus catatan"><Trash2 size={14} /> Hapus</button></td>
          </tr>
        {:else}
          <tr><td colspan="6"><div class="empty"><Inbox size={30} />{entries.length ? 'Tidak ada catatan yang cocok.' : 'Belum ada catatan keuangan. Tambahkan transaksi pertama Anda di atas.'}</div></td></tr>
        {/each}
      </tbody>
    </table>
  </div>
  <Pagination bind:page total={shown.length} {perPage} />
</div>

