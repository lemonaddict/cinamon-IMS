<script lang="ts">
  import { tick } from 'svelte';
  import { ChevronDown, Check, Search } from 'lucide-svelte';

  export let value: any = null;
  export let options: { value: any; label: string }[] = [];
  export let label = '';
  export let icon: any = null;
  export let placeholder = 'Pilih…';
  export let searchable = false;

  let open = false;
  let query = '';
  let btn: HTMLButtonElement;
  let searchEl: HTMLInputElement;
  $: current = options.find((o) => o.value === value);
  $: filtered = searchable && query.trim()
    ? options.filter((o) => o.label.toLowerCase().includes(query.trim().toLowerCase()))
    : options;

  async function toggle() {
    open = !open;
    if (open) {
      query = '';
      if (searchable) {
        await tick();
        searchEl?.focus();
      }
    }
  }

  function pick(v: any) {
    value = v;
    open = false;
    query = '';
    btn?.focus();
  }
</script>

<svelte:window on:keydown={(e) => open && e.key === 'Escape' && (open = false)} />

<div class="dd">
  <button type="button" class="field dd-btn" bind:this={btn} aria-haspopup="listbox" aria-expanded={open} aria-label={label} on:click={toggle}>
    {#if icon}<svelte:component this={icon} size={18} />{/if}
    <span class="dd-val">{current?.label ?? placeholder}</span>
    <ChevronDown size={16} />
  </button>
  {#if open}
    <button type="button" class="dd-backdrop" aria-label="Tutup pilihan" tabindex="-1" on:click={() => (open = false)}></button>
    <div class="dd-list" role="listbox" aria-label={label}>
      {#if searchable}
        <label class="field dd-search">
          <Search size={15} />
          <input bind:this={searchEl} bind:value={query} placeholder="Cari…" aria-label="Cari {label}" />
        </label>
      {/if}
      {#each filtered as o}
        <button type="button" role="option" aria-selected={o.value === value} class:on={o.value === value} on:click={() => pick(o.value)}>
          <span>{o.label}</span>{#if o.value === value}<Check size={16} />{/if}
        </button>
      {:else}
        <div class="muted dd-empty">{query.trim() ? 'Tidak ditemukan.' : 'Belum ada pilihan.'}</div>
      {/each}
    </div>
  {/if}
</div>
