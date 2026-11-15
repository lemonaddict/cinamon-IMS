<script lang="ts">
  import { Users, UserPlus, Shield, Ban, CheckCircle, KeyRound } from 'lucide-svelte';
  import { api, msg } from './api';
  import Pagination from './Pagination.svelte';

  type User = {
    id: number;
    name: string;
    email: string;
    role: 'user' | 'admin';
    status: 'active' | 'suspended';
    created_at: string;
  };

  let users: User[] = [];
  let error = '';
  let busy = false;
  let offset = 0;
  const limit = 50;

  let form = { name: '', email: '', password: '', role: 'user' as 'user' | 'admin' };
  let showForm = false;

  async function load() {
    error = '';
    try {
      users = await api(`/admin/users?limit=${limit}&offset=${offset}`);
    } catch (e) {
      error = msg(e);
    }
  }

  async function create() {
    error = '';
    busy = true;
    try {
      await api('/admin/users', 'POST', form);
      form = { name: '', email: '', password: '', role: 'user' };
      showForm = false;
      await load();
    } catch (e) {
      error = msg(e);
    }
    busy = false;
  }

  async function setStatus(u: User, status: 'active' | 'suspended') {
    if (!confirm(status === 'suspended' ? `Tangguhkan "${u.name}"? Sesi mereka akan dihapus.` : `Aktifkan kembali "${u.name}"?`)) return;
    error = '';
    try {
      await api(`/admin/users/${u.id}/status`, 'PUT', { status });
      await load();
    } catch (e) {
      error = msg(e);
    }
  }

  async function setRole(u: User, role: 'user' | 'admin') {
    if (!confirm(`Ubah peran "${u.name}" menjadi ${role}?`)) return;
    error = '';
    try {
      await api(`/admin/users/${u.id}/role`, 'PUT', { role });
      await load();
    } catch (e) {
      error = msg(e);
    }
  }

  async function resetPassword(u: User) {
    const password = prompt(`Kata sandi baru untuk ${u.email} (min. 10 karakter):`);
    if (!password) return;
    if (password.length < 10 || password.length > 72) {
      error = 'Kata sandi harus 10–72 karakter.';
      return;
    }
    error = '';
    try {
      await api(`/admin/users/${u.id}/password`, 'PUT', { password });
      alert('Kata sandi diganti. Pengguna harus login ulang.');
    } catch (e) {
      error = msg(e);
    }
  }

  load();
</script>

<section class="page">
  <header class="row" style="justify-content:space-between;align-items:center;margin-bottom:1rem">
    <div>
      <h2 style="margin:0;display:flex;align-items:center;gap:0.5rem"><Users size={22} /> Manajemen pengguna</h2>
      <p class="muted" style="margin:0.25rem 0 0">Hanya admin yang dapat menambah dan mengelola akun. Tidak ada pendaftaran mandiri.</p>
    </div>
    <button class="btn" on:click={() => (showForm = !showForm)}>
      <UserPlus size={18} /> {showForm ? 'Tutup' : 'Tambah pengguna'}
    </button>
  </header>

  {#if error}<p class="err" role="alert">{error}</p>{/if}

  {#if showForm}
    <form class="card" style="margin-bottom:1rem" on:submit|preventDefault={create}>
      <h3 style="margin-top:0">Pengguna baru</h3>
      <label class="field"><span>Nama</span><input bind:value={form.name} required maxlength="200" /></label>
      <label class="field"><span>Email</span><input type="email" bind:value={form.email} required /></label>
      <label class="field"><span>Kata sandi (10–72)</span><input type="password" bind:value={form.password} required minlength="10" maxlength="72" /></label>
      <label class="field"><span>Peran</span>
        <select bind:value={form.role}>
          <option value="user">User (IMS)</option>
          <option value="admin">Admin</option>
        </select>
      </label>
      <button class="btn" disabled={busy}>Simpan</button>
    </form>
  {/if}

  <div class="card" style="padding:0;overflow:auto">
    <table class="table">
      <thead>
        <tr>
          <th>Nama</th>
          <th>Email</th>
          <th>Peran</th>
          <th>Status</th>
          <th>Dibuat</th>
          <th></th>
        </tr>
      </thead>
      <tbody>
        {#each users as u}
          <tr>
            <td>{u.name}</td>
            <td>{u.email}</td>
            <td><span class="pill">{u.role === 'admin' ? 'Admin' : 'User'}</span></td>
            <td>
              {#if u.status === 'active'}
                <span class="pill" style="color:var(--ok,#16a34a)">Aktif</span>
              {:else}
                <span class="pill" style="color:var(--err,#dc2626)">Ditangguhkan</span>
              {/if}
            </td>
            <td class="muted">{u.created_at?.slice(0, 10) ?? ''}</td>
            <td class="row" style="gap:0.35rem;flex-wrap:wrap">
              {#if u.status === 'active'}
                <button class="btn ghost sm" title="Tangguhkan" on:click={() => setStatus(u, 'suspended')}><Ban size={16} /></button>
              {:else}
                <button class="btn ghost sm" title="Aktifkan" on:click={() => setStatus(u, 'active')}><CheckCircle size={16} /></button>
              {/if}
              {#if u.role === 'user'}
                <button class="btn ghost sm" title="Jadikan admin" on:click={() => setRole(u, 'admin')}><Shield size={16} /></button>
              {:else}
                <button class="btn ghost sm" title="Jadikan user" on:click={() => setRole(u, 'user')}><Users size={16} /></button>
              {/if}
              <button class="btn ghost sm" title="Ganti kata sandi" on:click={() => resetPassword(u)}><KeyRound size={16} /></button>
            </td>
          </tr>
        {:else}
          <tr><td colspan="6" class="muted" style="text-align:center;padding:1.5rem">Belum ada pengguna.</td></tr>
        {/each}
      </tbody>
    </table>
  </div>
  <Pagination {offset} {limit} count={users.length} on:page={(e) => { offset = e.detail; load(); }} />
</section>
