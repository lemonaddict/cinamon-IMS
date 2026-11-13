<script lang="ts">
  import { api } from './api';

  type Item = { product_id: number; location_id: number; quantity: number; direction: string };
  type Move = { id: number; type: string; reference_number: string; notes: string; created_at: string; items: Item[] };
  const label: Record<string, string> = { in: 'Stock in', out: 'Stock out', adjustment: 'Adjustment' };
  let moves: Move[] = [];
  let names: Record<string, string> = {};
  let error = '';

  Promise.all([api('/stock/movements?limit=50'), api('/products?limit=100'), api('/locations?limit=100')])
    .then(([m, p, l]) => {
      moves = m;
      for (const x of p) names['p' + x.id] = x.name;
      for (const x of l) names['l' + x.id] = x.name;
      names = names;
    })
    .catch((e) => (error = e.message));
</script>

<div class="panel wrap">
  <h3>Stock history</h3>
  <table>
    <thead><tr><th>When</th><th>Type</th><th>Items</th><th>Notes</th></tr></thead>
    <tbody>
      {#each moves as m}
        <tr>
          <td>{new Date(m.created_at).toLocaleString()}</td>
          <td>{label[m.type]}{m.reference_number === 'opname' ? ' (count)' : ''}</td>
          <td>{#each m.items as i}<div>{i.direction === 'in' ? '+' : '−'}{i.quantity} {names['p' + i.product_id]} at {names['l' + i.location_id]}</div>{/each}</td>
          <td>{m.notes}</td>
        </tr>
      {:else}
        <tr><td colspan="4" class="muted">No movements yet.</td></tr>
      {/each}
    </tbody>
  </table>
</div>
{#if error}<p class="err">{error}</p>{/if}
