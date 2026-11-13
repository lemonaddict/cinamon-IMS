<script lang="ts">
  import { RefreshCw, Trash2, RotateCcw, CheckCircle2, AlertTriangle, Clock } from 'lucide-svelte';
  import { listOps, sync, retry, discard, online, pending } from './offline';
  import type { Op } from './offline';

  let ops: Op[] = [];
  let busy = false;
  const load = async () => (ops = await listOps());
  async function syncNow() { busy = true; await sync(); await load(); busy = false; }
  async function doRetry(id: string) { await retry(id); await load(); }
  $: $pending, load();
</script>

<div class="card">
  <h3><RefreshCw size={18} /> Sinkronisasi</h3>
  <p class="muted">Perubahan yang dibuat saat offline disimpan di perangkat ini dan dikirim otomatis saat internet kembali. Server memeriksa ulang setiap perubahan sebelum menerapkannya.</p>
  <button class="btn" disabled={busy || !$online} on:click={syncNow}><RefreshCw size={18} /> {busy ? 'Menyinkronkan…' : 'Sinkronkan sekarang'}</button>
</div>

<div class="card ops">
  {#each ops as op (op.id)}
    <div class="row">
      <div>
        <b>{op.label}</b>
        <div class="muted">{new Date(op.createdAt).toLocaleString('id-ID')}</div>
        {#if op.error}<div class="err">{op.error}</div>{/if}
      </div>
      <div class="cell">
        <span class="chip {op.status === 'pending' ? 'wait' : 'out'}">
          {#if op.status === 'pending'}<Clock size={14} /> Menunggu{:else}<AlertTriangle size={14} /> Perlu dicek{/if}
        </span>
        {#if op.status === 'conflict'}<button class="btn ghost sm" on:click={() => doRetry(op.id)}><RotateCcw size={14} /> Coba lagi</button>{/if}
        <button class="btn danger sm" on:click={() => discard(op.id)}><Trash2 size={14} /> Hapus</button>
      </div>
    </div>
  {:else}
    <div class="empty"><CheckCircle2 size={32} />Semua perubahan sudah tersinkron.</div>
  {/each}
</div>
