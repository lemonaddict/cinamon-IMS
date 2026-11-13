<script lang="ts">
  import { createEventDispatcher } from 'svelte';
  import { Boxes, Mail, Lock, User, WifiOff, ShieldCheck, TrendingUp } from 'lucide-svelte';
  import { api, msg } from './api';

  const dispatch = createEventDispatcher();
  let mode: 'login' | 'register' = 'login';
  let name = '', email = '', password = '', error = '', busy = false;

  async function submit() {
    error = '';
    busy = true;
    try {
      await api(mode === 'login' ? '/auth/login' : '/auth/register', 'POST',
        mode === 'login' ? { email, password } : { name, email, password });
      dispatch('done', await api('/me'));
    } catch (e) {
      error = msg(e);
    }
    busy = false;
  }
</script>

<div class="auth">
  <section class="hero">
    <div class="brand"><Boxes size={28} /> Inventori</div>
    <h1>Stok usaha Anda, rapi di mana saja.</h1>
    <ul>
      <li><TrendingUp size={20} /> Pantau stok per lokasi secara langsung</li>
      <li><WifiOff size={20} /> Tetap bisa dipakai saat internet putus</li>
      <li><ShieldCheck size={20} /> Data setiap akun terpisah dan aman</li>
    </ul>
  </section>
  <section class="formside">
    <form class="card" on:submit|preventDefault={submit}>
      <h2>{mode === 'login' ? 'Masuk ke akun' : 'Buat akun baru'}</h2>
      {#if mode === 'register'}
        <label class="field"><User size={18} /><input placeholder="Nama lengkap" autocomplete="name" bind:value={name} required /></label>
      {/if}
      <label class="field"><Mail size={18} /><input type="email" placeholder="Email" autocomplete="email" bind:value={email} required /></label>
      <label class="field"><Lock size={18} /><input type="password" placeholder="Kata sandi (minimal 10 karakter)" autocomplete={mode === 'login' ? 'current-password' : 'new-password'} bind:value={password} required minlength="10" /></label>
      {#if error}<p class="err" role="alert">{error}</p>{/if}
      <button class="btn" disabled={busy}>{mode === 'login' ? 'Masuk' : 'Buat akun'}</button>
      <button type="button" class="btn ghost" on:click={() => { mode = mode === 'login' ? 'register' : 'login'; error = ''; }}>
        {mode === 'login' ? 'Belum punya akun? Daftar' : 'Sudah punya akun? Masuk'}
      </button>
    </form>
  </section>
</div>
