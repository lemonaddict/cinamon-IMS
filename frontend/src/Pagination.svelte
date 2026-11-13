<script lang="ts">
  import { ChevronLeft, ChevronRight } from 'lucide-svelte';

  export let page = 1;
  export let total = 0;
  export let perPage = 10;

  $: totalPages = Math.max(1, Math.ceil(total / perPage));
  $: if (page > totalPages) page = totalPages;
  $: if (page < 1) page = 1;
  $: start = total === 0 ? 0 : (page - 1) * perPage + 1;
  $: end = Math.min(total, page * perPage);

  function buildPages(cur: number, max: number): (number | '...')[] {
    if (max <= 7) return Array.from({ length: max }, (_, i) => i + 1);
    if (cur <= 4) return [1, 2, 3, 4, 5, '...', max];
    if (cur >= max - 3) return [1, '...', max - 4, max - 3, max - 2, max - 1, max];
    return [1, '...', cur - 1, cur, cur + 1, '...', max];
  }
  $: items = buildPages(page, totalPages);
</script>

{#if total > 0}
  <div class="pg">
    <span class="muted pg-info">Menampilkan {start}–{end} dari {total} data</span>
    <div class="pg-nav" role="navigation" aria-label="Navigasi halaman">
      <button type="button" class="pg-btn" disabled={page <= 1} on:click={() => (page -= 1)}>
        <ChevronLeft size={15} /> Sebelumnya
      </button>
      {#each items as item}
        {#if item === '...'}
          <span class="pg-dots">…</span>
        {:else}
          <button
            type="button"
            class="pg-btn num"
            class:on={item === page}
            aria-current={item === page ? 'page' : undefined}
            on:click={() => (page = item)}
          >
            {item}
          </button>
        {/if}
      {/each}
      <button type="button" class="pg-btn" disabled={page >= totalPages} on:click={() => (page += 1)}>
        Berikutnya <ChevronRight size={15} />
      </button>
    </div>
  </div>
{/if}
