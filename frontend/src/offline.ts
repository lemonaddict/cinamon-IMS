import { writable } from 'svelte/store';
import { api, ApiError, setReachHandler } from './api';

export type Op = {
  id: string; // also the idempotency key: the server never applies the same id twice
  uid: number;
  kind: 'movement' | 'opname';
  path: string;
  body: any;
  label: string;
  createdAt: string;
  status: 'pending' | 'conflict';
  error?: string;
};

export const online = writable(navigator.onLine);
export const pending = writable(0);
let uid = 0;

// ---- IndexedDB (no dependency): "cache" = last server data, "queue" = changes made offline ----
const dbp = new Promise<IDBDatabase>((res, rej) => {
  const r = indexedDB.open('inventori', 1);
  r.onupgradeneeded = () => {
    r.result.createObjectStore('cache');
    r.result.createObjectStore('queue', { keyPath: 'id' });
  };
  r.onsuccess = () => res(r.result);
  r.onerror = () => rej(r.error);
});
function run<T = unknown>(store: 'cache' | 'queue', mode: IDBTransactionMode, fn: (s: IDBObjectStore) => IDBRequest): Promise<T> {
  return dbp.then((db) => new Promise<T>((res, rej) => {
    const r = fn(db.transaction(store, mode).objectStore(store));
    r.onsuccess = () => res(r.result as T);
    r.onerror = () => rej(r.error);
  }));
}
const queueAll = () => run<Op[]>('queue', 'readonly', (s) => s.getAll());
const queuePut = (op: Op) => run('queue', 'readwrite', (s) => s.put(op));
const queueDel = (id: string) => run('queue', 'readwrite', (s) => s.delete(id));
export const clearCache = () => run('cache', 'readwrite', (s) => s.clear());

const key = (p: string) => (p === '/me' ? p : `${uid}:${p}`);
const isNetworkError = (e: unknown) => !(e instanceof ApiError);

export function setUser(id: number) {
  uid = id;
  refresh();
}

/** Network first; if the network is down, fall back to the last data seen on this device. */
export async function cached<T = any>(path: string): Promise<T> {
  try {
    const data = await api<T>(path);
    run('cache', 'readwrite', (s) => s.put(data, key(path))).catch(() => {});
    return data;
  } catch (e) {
    if (!isNetworkError(e)) throw e;
    const hit = await run<T | undefined>('cache', 'readonly', (s) => s.get(key(path)));
    if (hit === undefined) throw new Error('Belum ada data offline. Buka halaman ini sekali saat online.');
    return hit;
  }
}

/** Send a stock change; if offline, keep it on the device with its idempotency key. */
export async function submit(kind: Op['kind'], path: string, body: any, label: string) {
  const id = crypto.randomUUID();
  const payload = { ...body, idempotency_key: id, ...(kind === 'opname' ? { observed_at: new Date().toISOString() } : {}) };
  try {
    const res = await api(path, 'POST', payload);
    return { queued: false, res };
  } catch (e) {
    if (!isNetworkError(e)) throw e;
    await queuePut({ id, uid, kind, path, body: payload, label, createdAt: new Date().toISOString(), status: 'pending' });
    await refresh();
    return { queued: true, res: null };
  }
}

export async function listOps() {
  return (await queueAll()).filter((o) => o.uid === uid).sort((a, b) => a.createdAt.localeCompare(b.createdAt));
}
export async function refresh() {
  pending.set((await listOps()).length);
}
export async function discard(id: string) {
  await queueDel(id);
  await refresh();
}
export async function retry(id: string) {
  const op = (await listOps()).find((o) => o.id === id);
  if (op) await queuePut({ ...op, status: 'pending', error: undefined });
  await sync();
}

let syncing = false;
/** Replay queued changes in order. The server re-validates each one; rejected ones are kept for review. */
export async function sync() {
  if (syncing || !uid) return;
  syncing = true;
  try {
    for (const op of await listOps()) {
      if (op.status !== 'pending') continue;
      try {
        await api(op.path, 'POST', op.body); // a replayed idempotency key is answered without re-applying
        await queueDel(op.id);
      } catch (e) {
        if (isNetworkError(e)) break;
        await queuePut({ ...op, status: 'conflict', error: (e as Error).message });
      }
    }
  } finally {
    syncing = false;
    await refresh();
  }
}

// "Online" = the device has a network AND the server answered the latest request.
// api() reports every outcome here, so the badge can no longer flip back by accident.
setReachHandler((ok) => online.set(navigator.onLine && ok));

// Heartbeat: notice the server coming back (or going away) without waiting for a click.
async function probe() {
  if (document.hidden || !navigator.onLine) return;
  try { await api('/me'); } catch { /* api() already reported the result */ }
  await sync();
}
addEventListener('online', probe);
addEventListener('offline', () => online.set(false));
setInterval(probe, 15000);
