<script lang="ts">
  import { api } from './api';

  type Opt = { id: number; name: string; sku?: string };
  type Bal = { product_id: number; sku: string; product: string; location: string; quantity: number };
  let products: Opt[] = [];
  let locations: Opt[] = [];
  let balances: Bal[] = [];
  let error = '', info = '';

  // A key is reused when a request fails, so retrying can never apply the same movement twice.
  let m = { type: 'in', product_id: 0, location_id: 0, quantity: 1, direction: 'in', notes: '' };
  let key = crypto.randomUUID();
  let opLoc = 0;
  let counts: Record<number, string> = {};
  let opKey = crypto.randomUUID();

  async function load() {
    [products, locations, balances] = await Promise.all([
      api('/products?limit=100'), api('/locations?limit=100'), api('/stock/balances?limit=100'),
    ]);
    m.product_id ||= products[0]?.id ?? 0;
    m.location_id ||= locations[0]?.id ?? 0;
    opLoc ||= locations[0]?.id ?? 0;
  }
  async function run(fn: () => Promise<unknown>) {
    error = info = '';
    try { await fn(); await load(); } catch (e) { error = (e as Error).message; }
  }
  const post = () => run(async () => {
    await api('/stock/movements', 'POST', {
      type: m.type, notes: m.notes, idempotency_key: key,
      items: [{ product_id: m.product_id, location_id: m.location_id, quantity: m.quantity, direction: m.direction }],
    });
    key = crypto.randomUUID();
    info = 'Stock updated.';
  });
  const opname = () => run(async () => {
    const list = Object.entries(counts).filter(([, v]) => v !== '' && v != null).map(([id, v]) => ({ product_id: +id, counted: +v }));
    const r = await api('/stock/opname', 'POST', { location_id: opLoc, idempotency_key: opKey, counts: list });
    opKey = crypto.randomUUID();
    counts = {};
    info = `Stock count saved: ${r.changes} adjustment${r.changes === 1 ? '' : 's'}.`;
  });
  load().catch((e) => (error = e.message));
</script>

{#if products.length === 0 || locations.length === 0}
  <p class="muted">Add at least one product and one location before recording stock.</p>
{:else}
  <div class="panel">
    <h3>Record stock</h3>
    <form on:submit|preventDefault={post}>
      <select bind:value={m.type}>
        <option value="in">Stock in</option><option value="out">Stock out</option><option value="adjustment">Adjustment</option>
      </select>
      <select bind:value={m.product_id}>{#each products as p}<option value={p.id}>{p.sku} · {p.name}</option>{/each}</select>
      <select bind:value={m.location_id}>{#each locations as l}<option value={l.id}>{l.name}</option>{/each}</select>
      {#if m.type === 'adjustment'}
        <select bind:value={m.direction}><option value="in">Add</option><option value="out">Remove</option></select>
      {/if}
      <input type="number" min="1" bind:value={m.quantity} required />
      <input placeholder="Notes" bind:value={m.notes} />
      <button class="primary">Save movement</button>
    </form>
  </div>

  <div class="panel wrap">
    <h3>Stock count at a location</h3>
    <form on:submit|preventDefault={opname}>
      <select bind:value={opLoc}>{#each locations as l}<option value={l.id}>{l.name}</option>{/each}</select>
      <button class="primary">Save count</button>
    </form>
    <table>
      <thead><tr><th>Product</th><th>Counted (leave blank to skip)</th></tr></thead>
      <tbody>
        {#each products as p}
          <tr><td>{p.sku} · {p.name}</td>
            <td><input inputmode="numeric" pattern="[0-9]*" bind:value={counts[p.id]} /></td></tr>
        {/each}
      </tbody>
    </table>
  </div>
{/if}

{#if info}<p>{info}</p>{/if}
{#if error}<p class="err">{error}</p>{/if}

<div class="panel wrap">
  <h3>Current stock</h3>
  <table>
    <thead><tr><th>SKU</th><th>Product</th><th>Location</th><th>Quantity</th></tr></thead>
    <tbody>
      {#each balances as b}
        <tr><td>{b.sku}</td><td>{b.product}</td><td>{b.location}</td><td>{b.quantity}</td></tr>
      {:else}
        <tr><td colspan="4" class="muted">No stock recorded yet.</td></tr>
      {/each}
    </tbody>
  </table>
</div>
