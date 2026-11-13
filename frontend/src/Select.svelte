<script lang="ts">
  import { ChevronDown, Check } from 'lucide-svelte';

  export let value: any = null;
  export let options: { value: any; label: string }[] = [];
  export let label = '';
  export let icon: any = null;

  let open = false;
  let btn: HTMLButtonElement;
  $: current = options.find((o) => o.value === value);

  function pick(v: any) {
    value = v;
    open = false;
    btn?.focus();
  }
</script>

<svelte:window on:keydown={(e) => open && e.key === 'Escape' && (open = false)} />

<div class="dd">
  <button type="button" class="field dd-btn" bind:this={btn} aria-haspopup="listbox" aria-expanded={open} aria-label={label} on:click={() => (open = !open)}>
    {#if icon}<svelte:component this={icon} size={18} />{/if}
    <span class="dd-val">{current?.label ?? 'Pilih…'}</span>
    <ChevronDown size={16} />
  </button>
  {#if open}
    <button type="button" class="dd-backdrop" aria-label="Tutup pilihan" tabindex="-1" on:click={() => (open = false)}></button>
    <div class="dd-list" role="listbox" aria-label={label}>
      {#each options as o}
        <button type="button" role="option" aria-selected={o.value === value} class:on={o.value === value} on:click={() => pick(o.value)}>
          <span>{o.label}</span>{#if o.value === value}<Check size={16} />{/if}
        </button>
      {:else}
        <div class="muted dd-empty">Belum ada pilihan.</div>
      {/each}
    </div>
  {/if}
</div>
