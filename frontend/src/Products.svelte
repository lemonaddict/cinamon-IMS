<script lang="ts">
  import { api } from './api';

  type Cat = { id: number; name: string };
  type Product = { id: number; name: string; sku: string; category_id: number | null };
  const blank = () => ({ name: '', sku: '', description: '', category_id: null as number | null });
  let products: Product[] = [];
  let cats: Cat[] = [];
  let f = blank();
  let error = '';

  async function load() {
    [products, cats] = await Promise.all([api('/products?limit=100'), api('/categories?limit=100')]);
  }
  async function run(fn: () => Promise<unknown>) {
    error = '';
    try { await fn(); await load(); } catch (e) { error = (e as Error).message; }
  }
  const add = () => run(async () => { await api('/products', 'POST', f); f = blank(); });
  const del = (id: number) => run(() => api(`/products/${id}`, 'DELETE'));
  const catName = (id: number | null) => cats.find((c) => c.id === id)?.name ?? '';
  load().catch((e) => (error = e.message));
</script>

<div class="panel">
  <h3>New product</h3>
  <form on:submit|preventDefault={add}>
    <input placeholder="Name" bind:value={f.name} required />
    <input placeholder="SKU" bind:value={f.sku} required />
    <select bind:value={f.category_id}>
      <option value={null}>No category</option>
      {#each cats as c}<option value={c.id}>{c.name}</option>{/each}
    </select>
    <input placeholder="Description" bind:value={f.description} />
    <button class="primary">Add product</button>
  </form>
  {#if error}<p class="err">{error}</p>{/if}
</div>

<div class="panel wrap">
  <table>
    <thead><tr><th>SKU</th><th>Name</th><th>Category</th><th></th></tr></thead>
    <tbody>
      {#each products as p}
        <tr><td>{p.sku}</td><td>{p.name}</td><td>{catName(p.category_id)}</td>
          <td><button on:click={() => del(p.id)}>Delete</button></td></tr>
      {:else}
        <tr><td colspan="4" class="muted">No products yet. Add one above.</td></tr>
      {/each}
    </tbody>
  </table>
</div>
