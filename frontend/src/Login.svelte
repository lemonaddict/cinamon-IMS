<script lang="ts">
  import { createEventDispatcher } from 'svelte';
  import { Boxes, Mail, Lock, WifiOff, ShieldCheck, TrendingUp } from 'lucide-svelte';
  import { api, msg } from './api';

  const dispatch = createEventDispatcher();
  let email = '', password = '', error = '', busy = false;

  async function submit() {
    error = '';
    busy = true;
    try {
      await api('/auth/login', 'POST', { email, password });
      dispatch('done', await api('/me'));
    } catch (e) {
      error = msg(e);
    }
    busy = false;
  }
</script>

<div class="auth">
  <section class="hero">
    <div class="brand"><Boxes size={28} /> CIMOL - Cellix Inventory Management Online</div>
    <h1>Stok usaha Anda, rapi di mana saja.</h1>
    <ul>
      <li><TrendingUp size={20} /> Pantau stok per lokasi secara langsung</li>
      <li><WifiOff size={20} /> Tetap bisa dipakai saat internet putus</li>
      <li><ShieldCheck size={20} /> Data setiap akun terpisah dan aman</li>
    </ul>
  </section>
  <section class="formside">
    <form class="card" on:submit|preventDefault={submit}>
      <h2>Masuk ke akun</h2>
      <label class="field"><Mail size={18} /><input type="email" placeholder="Email" autocomplete="email" bind:value={email} required /></label>
      <label class="field"><Lock size={18} /><input type="password" placeholder="Kata sandi" autocomplete="current-password" bind:value={password} required minlength="10" /></label>
      {#if error}<p class="err" role="alert">{error}</p>{/if}
      <button class="btn" disabled={busy}>Masuk</button>
      <p class="muted" style="margin-top:0.75rem;font-size:0.85rem;text-align:center">Akun dibuat oleh administrator. Hubungi admin jika belum punya akses.</p>
    </form>
  </section>
</div>
