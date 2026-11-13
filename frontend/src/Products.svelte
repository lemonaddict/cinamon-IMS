<script lang="ts">
  import { Plus, Trash2, Search, Package, Tag, Inbox } from 'lucide-svelte';
  import { api, msg } from './api';
  import { cached } from './offline';
  import Select from './Select.svelte';

  type Cat = { id: number; name: string };
  type Product = { id: number; name: string; sku: string; category_id: number | null };
  const blank = () => ({ name: '', sku: '', description: '', category_id: null as number | null });
  let products: Product[] = [];
  let cats: Cat[] = [];
  let f = blank();
  let q = '';
  let error = '';

  async function load() {
    [products, cats] = await Promise.all([cached('/products?limit=100'), cached('/categories?limit=100')]);
  }
  async function run(fn: () => Promise<unknown>) {
    error = '';
    try { await fn(); await load(); } catch (e) { error = msg(e); }
  }
  const add = () => run(async () => { await api('/products', 'POST', f); f = blank(); });
  const del = (p: Product) => confirm(`Hapus produk "${p.name}"?`) && run(() => api(`/products/${p.id}`, 'DELETE'));
  const catName = (id: number | null) => cats.find((c) => c.id === id)?.name ?? '—';
  $: catOptions = [{ value: null, label: 'Tanpa kategori' }, ...cats.map((c) => ({ value: c.id, label: c.name }))];
  $: shown = products.filter((p) => (p.name + p.sku).toLowerCase().includes(q.toLowerCase()));
  load().catch((e) => (error = msg(e)));
</script>

<div class="card">
  <h3><Plus size={18} /> Tambah produk</h3>
  <form on:submit|preventDefault={add}>
    <label class="field"><Package size={18} /><input placeholder="Nama produk" bind:value={f.name} required /></label>
    <label class="field"><Tag size={18} /><input placeholder="SKU" bind:value={f.sku} required /></label>
    <Select bind:value={f.category_id} label="Kategori" options={catOptions} />
    <label class="field"><input placeholder="Deskripsi (opsional)" bind:value={f.description} /></label>
    <button class="btn"><Plus size={18} /> Simpan produk</button>
  </form>
  {#if error}<p class="err" role="alert">{error}</p>{/if}
</div>

<div class="card wrap">
  <label class="field"><Search size={18} /><input placeholder="Cari nama atau SKU" bind:value={q} /></label>
  <table>
    <thead><tr><th>Produk</th><th>SKU</th><th>Kategori</th><th></th></tr></thead>
    <tbody>
      {#each shown as p (p.id)}
        <tr>
          <td><span class="cell"><span class="avatar">{p.name.charAt(0).toUpperCase()}</span>{p.name}</span></td>
          <td>{p.sku}</td>
          <td>{catName(p.category_id)}</td>
          <td><button class="btn danger sm" on:click={() => del(p)} aria-label="Hapus {p.name}"><Trash2 size={14} /> Hapus</button></td>
        </tr>
      {:else}
        <tr><td colspan="4"><div class="empty"><Inbox size={30} />{products.length ? 'Tidak ada produk yang cocok.' : 'Belum ada produk. Tambahkan produk pertama Anda di atas.'}</div></td></tr>
      {/each}
    </tbody>
  </table>
</div>
