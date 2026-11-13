<script lang="ts">
  import { api } from './api';

  type Row = { id: number; name: string };
  const kinds = ['categories', 'locations'];
  let data: Record<string, Row[]> = { categories: [], locations: [] };
  let names: Record<string, string> = { categories: '', locations: '' };
  let error = '';

  async function load() {
    for (const k of kinds) data[k] = await api(`/${k}?limit=100`);
    data = data;
  }
  async function run(fn: () => Promise<unknown>) {
    error = '';
    try { await fn(); await load(); } catch (e) { error = (e as Error).message; }
  }
  const add = (k: string) => run(async () => { await api(`/${k}`, 'POST', { name: names[k] }); names[k] = ''; });
  const del = (k: string, id: number) => run(() => api(`/${k}/${id}`, 'DELETE'));
  load().catch((e) => (error = e.message));
</script>

{#each kinds as k}
  <div class="panel">
    <h3>{k === 'categories' ? 'Categories' : 'Storage locations'}</h3>
    <form on:submit|preventDefault={() => add(k)}>
      <input placeholder="Name" bind:value={names[k]} required />
      <button class="primary">Add {k === 'categories' ? 'category' : 'location'}</button>
    </form>
    {#each data[k] as row}
      <p>{row.name} <button on:click={() => del(k, row.id)}>Delete</button></p>
    {:else}
      <p class="muted">None yet.</p>
    {/each}
  </div>
{/each}
{#if error}<p class="err">{error}</p>{/if}
