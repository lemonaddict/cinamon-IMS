<script lang="ts">
  import { ArrowDownToLine, ArrowUpFromLine, SlidersHorizontal, Inbox } from 'lucide-svelte';
  import { msg } from './api';
  import { cached } from './offline';

  type Item = { product_id: number; location_id: number; quantity: number; direction: string };
  type Move = { id: number; type: string; reference_number: string; notes: string; created_at: string; items: Item[] };
  const info: Record<string, { label: string; cls: string; icon: any }> = {
    in: { label: 'Stok masuk', cls: 'in', icon: ArrowDownToLine },
    out: { label: 'Stok keluar', cls: 'out', icon: ArrowUpFromLine },
    adjustment: { label: 'Penyesuaian', cls: 'adj', icon: SlidersHorizontal },
  };
  let moves: Move[] = [];
  let names: Record<string, string> = {};
  let error = '';

  Promise.all([cached('/stock/movements?limit=50'), cached('/products?limit=100'), cached('/locations?limit=100')])
    .then(([m, p, l]) => {
      moves = m;
      for (const x of p) names['p' + x.id] = x.name;
      for (const x of l) names['l' + x.id] = x.name;
      names = names;
    })
    .catch((e) => (error = msg(e)));
</script>

{#if error}<p class="err" role="alert">{error}</p>{/if}
<div class="card wrap">
  <table>
    <thead><tr><th>Waktu</th><th>Jenis</th><th>Rincian</th><th>Catatan</th></tr></thead>
    <tbody>
      {#each moves as m (m.id)}
        <tr>
          <td>{new Date(m.created_at).toLocaleString('id-ID')}</td>
          <td><span class="chip {info[m.type].cls}"><svelte:component this={info[m.type].icon} size={14} />{info[m.type].label}{m.reference_number === 'opname' ? ' (hitung stok)' : ''}</span></td>
          <td>{#each m.items as i}<div>{i.direction === 'in' ? '+' : '−'}{i.quantity} {names['p' + i.product_id]} di {names['l' + i.location_id]}</div>{/each}</td>
          <td>{m.notes}</td>
        </tr>
      {:else}
        <tr><td colspan="4"><div class="empty"><Inbox size={30} />Belum ada pergerakan stok.</div></td></tr>
      {/each}
    </tbody>
  </table>
</div>
