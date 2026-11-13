<script lang="ts">
  import { Plus, Pencil, Save, X, Trash2, Search, Package, Tag, Banknote, Inbox } from 'lucide-svelte';
  import { api, msg, idr } from './api';
  import { cached } from './offline';
  import Select from './Select.svelte';

  type Cat = { id: number; name: string };
  type Product = { id: number; name: string; sku: string; cost_price: number; selling_price: number; description: string; category_id: number | null };
  const blank = () => ({ name: '', sku: '', cost_price: 0, selling_price: 0, description: '', category_id: null as number | null });
  let products: Product[] = [];
  let cats: Cat[] = [];
  let f = blank();
  let editingId: number | null = null;
  let q = '';
  let error = '';

  async function load() {
    [products, cats] = await Promise.all([cached('/products?limit=100'), cached('/categories?limit=100')]);
  }
  async function run(fn: () => Promise<unknown>) {
    error = '';
    try { await fn(); await load(); } catch (e) { error = msg(e); }
  }
  function startEdit(p: Product) {
    editingId = p.id;
    f = {
      name: p.name,
      sku: p.sku,
      cost_price: p.cost_price ?? 0,
      selling_price: p.selling_price ?? 0,
      description: p.description ?? '',
      category_id: p.category_id ?? null,
    };
    error = '';
    window.scrollTo({ top: 0, behavior: 'smooth' });
  }
  function cancelEdit() {
    editingId = null;
    f = blank();
    error = '';
  }
  const save = () => run(async () => {
    const payload = {
      ...f,
      cost_price: Number(f.cost_price) || 0,
      selling_price: Number(f.selling_price) || 0,
    };
    if (editingId !== null) {
      await api(`/products/${editingId}`, 'PUT', payload);
      editingId = null;
    } else {
      await api('/products', 'POST', payload);
    }
    f = blank();
  });
  const del = (p: Product) => confirm(`Hapus produk "${p.name}"?`) && run(async () => {
    await api(`/products/${p.id}`, 'DELETE');
    if (editingId === p.id) cancelEdit();
  });
  const catName = (id: number | null) => cats.find((c) => c.id === id)?.name ?? '—';
  $: catOptions = [{ value: null, label: 'Pilih kategori' }, ...cats.map((c) => ({ value: c.id, label: c.name }))];
  $: shown = products.filter((p) => (p.name + p.sku).toLowerCase().includes(q.toLowerCase()));
  load().catch((e) => (error = msg(e)));
</script>

<div class="card">
  <h3>{#if editingId !== null}<Pencil size={18} /> Edit produk{:else}<Plus size={18} /> Tambah produk{/if}</h3>
  <form on:submit|preventDefault={save}>
    <label class="field"><Package size={18} /><input placeholder="Nama produk" bind:value={f.name} required /></label>
    <label class="field"><Tag size={18} /><input placeholder="SKU" bind:value={f.sku} required /></label>
    <label class="field"><Banknote size={18} /><input type="number" min="0" placeholder="Harga beli" title="Harga beli (HPP)" bind:value={f.cost_price} /></label>
    <label class="field"><Banknote size={18} /><input type="number" min="0" placeholder="Harga jual" title="Harga jual" bind:value={f.selling_price} /></label>
    <Select bind:value={f.category_id} label="Kategori" placeholder="Pilih kategori" searchable options={catOptions} />
    <label class="field"><input placeholder="Deskripsi (opsional)" bind:value={f.description} /></label>
    <button class="btn">{#if editingId !== null}<Save size={18} /> Simpan perubahan{:else}<Plus size={18} /> Simpan produk{/if}</button>
    {#if editingId !== null}
      <button type="button" class="btn ghost" on:click={cancelEdit}><X size={18} /> Batal</button>
    {/if}
  </form>
  {#if error}<p class="err" role="alert">{error}</p>{/if}
</div>

<div class="card">
  <label class="field" style="margin-bottom: 12px;"><Search size={18} /><input placeholder="Cari nama atau SKU" bind:value={q} /></label>
  <div class="wrap">
    <table>
      <thead><tr><th>Produk</th><th>SKU</th><th>Harga Beli</th><th>Harga Jual</th><th>Kategori</th><th></th></tr></thead>
      <tbody>
        {#each shown as p (p.id)}
          <tr>
            <td><span class="cell"><span class="avatar">{p.name.charAt(0).toUpperCase()}</span>{p.name}</span></td>
            <td>{p.sku}</td>
            <td>{idr(p.cost_price)}</td>
            <td>{idr(p.selling_price)}</td>
            <td>{catName(p.category_id)}</td>
            <td style="width: 1%; white-space: nowrap;">
              <span class="cell">
                <button class="btn ghost sm" on:click={() => startEdit(p)} aria-label="Edit {p.name}"><Pencil size={14} /> Edit</button>
                <button class="btn danger sm" on:click={() => del(p)} aria-label="Hapus {p.name}"><Trash2 size={14} /> Hapus</button>
              </span>
            </td>
          </tr>
        {:else}
          <tr><td colspan="6"><div class="empty"><Inbox size={30} />{products.length ? 'Tidak ada produk yang cocok.' : 'Belum ada produk. Tambahkan produk pertama Anda di atas.'}</div></td></tr>
        {/each}
      </tbody>
    </table>
  </div>
</div>
