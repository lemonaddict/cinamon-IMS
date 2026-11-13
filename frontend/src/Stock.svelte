<script lang="ts">
  import { ArrowDownToLine, ArrowUpFromLine, SlidersHorizontal, ClipboardCheck, ArrowLeftRight, Package, MapPin, Banknote, Plus, Trash2, Save, Inbox, CloudOff, CheckCircle2 } from 'lucide-svelte';
  import { msg, idr } from './api';
  import { cached, submit } from './offline';
  import Select from './Select.svelte';

  type Opt = { id: number; name: string; sku?: string; cost_price?: number; selling_price?: number };
  type Bal = { sku: string; product: string; location: string; quantity: number };
  type Row = { product_id: number; location_id: number; quantity: number; direction: 'in' | 'out' };
  const types = [
    { v: 'in', label: 'Stok masuk', icon: ArrowDownToLine },
    { v: 'out', label: 'Stok keluar', icon: ArrowUpFromLine },
    { v: 'adjustment', label: 'Penyesuaian', icon: SlidersHorizontal },
  ] as const;
  let products: Opt[] = [];
  let locations: Opt[] = [];
  let balances: Bal[] = [];
  let moveType: 'in' | 'out' | 'adjustment' = 'in';
  let notes = '';
  let rows: Row[] = [{ product_id: 0, location_id: 0, quantity: 1, direction: 'in' }];
  let opLoc = 0;
  let counts: Record<number, string> = {};
  let info = '', error = '', queued = false;
  $: productOptions = products.map((p) => ({ value: p.id, label: `${p.name} (${p.sku})` }));
  $: locationOptions = locations.map((l) => ({ value: l.id, label: l.name }));
  const dirOptions = [{ value: 'in', label: 'Tambah' }, { value: 'out', label: 'Kurangi' }];

  const newRow = (): Row => ({ product_id: 0, location_id: 0, quantity: 1, direction: 'in' });
  const addRow = () => { if (rows.length < 100) rows = [...rows, newRow()]; };
  const removeRow = (idx: number) => { if (rows.length > 1) rows = rows.filter((_, i) => i !== idx); };

  function rowPrice(r: Row, type: string, prods: Opt[]) {
    const p = prods.find((x) => Number(x.id) === Number(r.product_id));
    if (!p) return 0;
    const isIn = type === 'in' || (type === 'adjustment' && r.direction === 'in');
    return isIn ? (p.cost_price ?? 0) : (p.selling_price ?? 0);
  }

  $: totalQty = rows.reduce((sum, r) => sum + Math.max(0, Number(r.quantity) || 0), 0);
  $: grandTotal = rows.reduce((sum, r) => sum + rowPrice(r, moveType, products) * Math.max(0, Number(r.quantity) || 0), 0);

  async function load() {
    [products, locations, balances] = await Promise.all([cached('/products?limit=100'), cached('/locations?limit=100'), cached('/stock/balances?limit=100')]);
  }
  async function run(fn: () => Promise<{ queued: boolean }>, done: string) {
    info = error = '';
    try {
      const r = await fn();
      queued = r.queued;
      info = r.queued ? 'Tersimpan di perangkat. Akan dikirim otomatis saat online.' : done;
      await load();
    } catch (e) { error = msg(e); }
  }
  const post = () => run(async () => {
    const items = rows.map((r) => ({
      product_id: Number(r.product_id),
      location_id: Number(r.location_id),
      quantity: Number(r.quantity) || 0,
      direction: r.direction,
    }));
    if (items.some((i) => !i.product_id || !i.location_id)) {
      throw new Error('Pilih produk dan lokasi terlebih dahulu.');
    }
    const labelPrefix = types.find((t) => t.v === moveType)?.label ?? 'Stok';
    const label = items.length === 1
      ? `${labelPrefix}: ${items[0].quantity} × ${products.find((p) => p.id === items[0].product_id)?.name}`
      : `${labelPrefix}: ${items.length} item`;
    const res = await submit('movement', '/stock/movements', { type: moveType, notes, items }, label);
    rows = [newRow()];
    notes = '';
    return res;
  }, 'Stok berhasil diperbarui.');
  const opname = () => run(async () => {
    if (!opLoc) throw new Error('Pilih lokasi hitung terlebih dahulu.');
    const list = Object.entries(counts).filter(([, v]) => v != null && v !== '').map(([id, v]) => ({ product_id: +id, counted: +v }));
    if (!list.length) throw new Error('Isi jumlah fisik minimal satu produk.');
    const r = await submit('opname', '/stock/opname', { location_id: opLoc, counts: list },
      `Hitung stok: ${list.length} produk di ${locations.find((l) => l.id === opLoc)?.name}`);
    counts = {};
    return r;
  }, 'Hasil hitung stok tersimpan.');
  load().catch((e) => (error = msg(e)));
</script>

{#if info}<div class="note" class:ok={!queued}>{#if queued}<CloudOff size={18} />{:else}<CheckCircle2 size={18} />{/if} {info}</div>{/if}
{#if error}<p class="err" role="alert">{error}</p>{/if}

{#if products.length === 0 || locations.length === 0}
  <div class="card empty"><Inbox size={32} />Tambahkan minimal satu produk dan satu lokasi sebelum mencatat stok.</div>
{:else}
  <div class="card">
    <h3><ArrowLeftRight size={18} /> Catat pergerakan stok</h3>
    <div class="seg one-row">
      {#each types as t}<button type="button" class:on={moveType === t.v} on:click={() => (moveType = t.v)}><svelte:component this={t.icon} size={16} />{t.label}</button>{/each}
    </div>
    <form on:submit|preventDefault={post} style="display: grid; gap: 10px;">
      {#each rows as r, i (i)}
        <div style="display: flex; flex-wrap: wrap; gap: 10px; align-items: center;">
          <Select bind:value={r.product_id} label="Produk" placeholder="Pilih produk" searchable icon={Package} options={productOptions} />
          <Select bind:value={r.location_id} label="Lokasi" placeholder="Pilih lokasi" searchable icon={MapPin} options={locationOptions} />
          {#if moveType === 'adjustment'}
            <Select bind:value={r.direction} label="Arah penyesuaian" icon={SlidersHorizontal} options={dirOptions} />
          {/if}
          <label class="field" style="flex: 0 1 110px;"><input type="number" min="1" placeholder="Jumlah" aria-label="Jumlah" bind:value={r.quantity} required /></label>
          <label class="field" style="flex: 1 1 150px;" title={moveType === 'in' || (moveType === 'adjustment' && r.direction === 'in') ? 'Harga beli (cost_price)' : 'Harga jual (selling_price)'}>
            <Banknote size={18} />
            <input type="text" value={idr(rowPrice(r, moveType, products))} readonly aria-label="Harga produk" />
          </label>
          {#if rows.length > 1}
            <button type="button" class="btn danger sm icon" on:click={() => removeRow(i)} aria-label="Hapus baris"><Trash2 size={16} /></button>
          {/if}
        </div>
      {/each}
      <div style="display: flex; flex-wrap: wrap; gap: 10px; align-items: center;">
        <button type="button" class="btn ghost sm" on:click={addRow} disabled={rows.length >= 100}><Plus size={16} /> Tambah baris</button>
        <label class="field" style="flex: 1 1 220px;"><input placeholder="Catatan (opsional)" bind:value={notes} /></label>
        <button class="btn"><Save size={18} /> Simpan pergerakan ({rows.length} item)</button>
      </div>
    </form>
    <div class="row muted" style="margin-top: 12px; padding-top: 10px; border-top: 1px solid var(--line);">
      <span>
        <Banknote size={16} style="vertical-align: -3px; margin-right: 4px;" />
        {rows.length} baris · {totalQty} unit
      </span>
      <strong style="color: var(--ink); font-size: 16px;">Total nilai: {idr(grandTotal)}</strong>
    </div>
  </div>

  <div class="card wrap">
    <h3><ClipboardCheck size={18} /> Hitung stok di lokasi</h3>
    <form on:submit|preventDefault={opname}>
      <Select bind:value={opLoc} label="Lokasi hitung" placeholder="Pilih lokasi" searchable icon={MapPin} options={locationOptions} />
      <button class="btn"><Save size={18} /> Simpan hasil hitung</button>
    </form>
    <table>
      <thead><tr><th>Produk</th><th>Jumlah fisik (kosongkan untuk dilewati)</th></tr></thead>
      <tbody>
        {#each products as p}
          <tr><td>{p.name} ({p.sku})</td><td><label class="field"><input inputmode="numeric" pattern="[0-9]*" aria-label="Jumlah fisik {p.name}" bind:value={counts[p.id]} /></label></td></tr>
        {/each}
      </tbody>
    </table>
  </div>
{/if}

<div class="card wrap">
  <h3><Package size={18} /> Stok saat ini</h3>
  <table>
    <thead><tr><th>Produk</th><th>SKU</th><th>Lokasi</th><th>Jumlah</th></tr></thead>
    <tbody>
      {#each balances as b}
        <tr><td>{b.product}</td><td>{b.sku}</td><td>{b.location}</td><td><b>{b.quantity}</b></td></tr>
      {:else}
        <tr><td colspan="4"><div class="empty"><Inbox size={30} />Belum ada stok tercatat.</div></td></tr>
      {/each}
    </tbody>
  </table>
</div>
