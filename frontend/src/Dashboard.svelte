<script lang="ts">
  import { api } from './api';

  let stats: { products: number; locations: number; total_units: number } | null = null;
  let error = '';
  api('/dashboard').then((s) => (stats = s)).catch((e) => (error = e.message));
</script>

{#if stats}
  <div class="stats">
    <div><b>{stats.products}</b>products</div>
    <div><b>{stats.locations}</b>locations</div>
    <div><b>{stats.total_units}</b>units in stock</div>
  </div>
  {#if stats.products === 0}<p class="muted">Start in Categories &amp; locations, then add your first product.</p>{/if}
{/if}
{#if error}<p class="err">{error}</p>{/if}
