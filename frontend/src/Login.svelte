<script lang="ts">
  import { createEventDispatcher } from 'svelte';
  import { api } from './api';

  const dispatch = createEventDispatcher();
  let mode: 'login' | 'register' = 'login';
  let name = '', email = '', password = '', error = '';

  async function submit() {
    error = '';
    try {
      await api(mode === 'login' ? '/auth/login' : '/auth/register', 'POST',
        mode === 'login' ? { email, password } : { name, email, password });
      dispatch('done', await api('/me'));
    } catch (e) {
      error = (e as Error).message;
    }
  }
</script>

<main>
  <div class="panel">
    <h3>{mode === 'login' ? 'Sign in' : 'Create your account'}</h3>
    <form on:submit|preventDefault={submit}>
      {#if mode === 'register'}<input placeholder="Your name" bind:value={name} required />{/if}
      <input type="email" placeholder="Email" bind:value={email} required />
      <input type="password" placeholder="Password (10+ characters)" bind:value={password} required minlength="10" />
      <button class="primary">{mode === 'login' ? 'Sign in' : 'Create account'}</button>
    </form>
    {#if error}<p class="err">{error}</p>{/if}
    <p><button type="button" on:click={() => (mode = mode === 'login' ? 'register' : 'login')}>
      {mode === 'login' ? 'I need an account' : 'I already have an account'}
    </button></p>
  </div>
</main>
