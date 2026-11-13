<script lang="ts">
  import { ArrowDownToLine, ArrowUpFromLine, SlidersHorizontal, ClipboardCheck, ArrowLeftRight, Package, MapPin, Save, Inbox, CloudOff, CheckCircle2 } from 'lucide-svelte';
  import { msg } from './api';
  import { cached, submit } from './offline';

  type Opt = { id: number; name: string; sku?: string };
  type Bal = { sku: string; product: string; location: string; quantity: number };
  const types = [
    { v: 'in', label: 'Stok masuk', icon: ArrowDownToLine },
    { v: 'out', label: 'Stok keluar', icon: ArrowUpFromLine },
    { v: 'adjustment', label: 'Penyesuaian', icon: SlidersHorizontal },
  ];
  let products: Opt[] = [];
  let locations: Opt[] = [];
  let balances: Bal[] = [];
  let m = { type: 'in', product_id: 0, location_id: 0, quantity: 1, direction: 'in', notes: '' };
  let opLoc = 0;
  let counts: Record<number, string> = {};
  let info = '', error = '', queued = false;

  async function load() {
    [products, locations, balances] = await Promise.all([cached('/products?limit=100'), cached('/locations?limit=100'), cached('/stock/balances?limit=100')]);
    m.product_id ||= products[0]?.id ?? 0;
    m.location_id ||= locations[0]?.id ?? 0;
    opLoc ||= locations[0]?.id ?? 0;
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
  const post = () => run(() => submit('movement', '/stock/movements',
    { type: m.type, notes: m.notes, items: [{ product_id: m.product_id, location_id: m.location_id, quantity: m.quantity, direction: m.direction }] },
    `${types.find((t) => t.v === m.type)?.label}: ${m.quantity} × ${products.find((p) => p.id === m.product_id)?.name}`), 'Stok berhasil diperbarui.');
  const opname = () => run(async () => {
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
    <div class="seg">
      {#each types as t}<button type="button" class:on={m.type === t.v} on:click={() => (m.type = t.v)}><svelte:component this={t.icon} size={16} />{t.label}</button>{/each}
    </div>
    <form on:submit|preventDefault={post}>
      <label class="field"><Package size={18} /><select bind:value={m.product_id} aria-label="Produk">{#each products as p}<option value={p.id}>{p.name} ({p.sku})</option>{/each}</select></label>
      <label class="field"><MapPin size={18} /><select bind:value={m.location_id} aria-label="Lokasi">{#each locations as l}<option value={l.id}>{l.name}</option>{/each}</select></label>
      {#if m.type === 'adjustment'}
        <label class="field"><SlidersHorizontal size={18} /><select bind:value={m.direction} aria-label="Arah penyesuaian"><option value="in">Tambah</option><option value="out">Kurangi</option></select></label>
      {/if}
      <label class="field"><input type="number" min="1" placeholder="Jumlah" aria-label="Jumlah" bind:value={m.quantity} required /></label>
      <label class="field"><input placeholder="Catatan (opsional)" bind:value={m.notes} /></label>
      <button class="btn"><Save size={18} /> Simpan pergerakan</button>
    </form>
  </div>

  <div class="card wrap">
    <h3><ClipboardCheck size={18} /> Hitung stok di lokasi</h3>
    <form on:submit|preventDefault={opname}>
      <label class="field"><MapPin size={18} /><select bind:value={opLoc} aria-label="Lokasi hitung">{#each locations as l}<option value={l.id}>{l.name}</option>{/each}</select></label>
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
