// Empty = same origin (Vite proxy). Set VITE_API_URL only to call the backend directly (needs CORS).
const BASE: string = import.meta.env.VITE_API_URL ?? '';

export class ApiError extends Error {
  constructor(message: string, public status: number) {
    super(message);
  }
}

const ID: Record<string, string> = {
  'authentication required': 'Silakan masuk terlebih dahulu.',
  'invalid email or password': 'Email atau kata sandi salah.',
  'account suspended': 'Akun Anda sedang ditangguhkan.',
  'registration is closed': 'Pendaftaran sedang ditutup.',
  'too many attempts, try again later': 'Terlalu banyak percobaan. Coba lagi sebentar lagi.',
  'already exists': 'Data tersebut sudah ada.',
  'not found': 'Data tidak ditemukan.',
  'referenced record does not exist or is still in use': 'Data masih dipakai atau tidak ditemukan.',
  'insufficient stock': 'Stok tidak mencukupi.',
  'stock changed since the count; please recount': 'Stok di server berubah sejak penghitungan. Hitung ulang.',
  'name, a valid email and a 10-72 character password are required': 'Isi nama, email yang valid, dan kata sandi 10–72 karakter.',
  'name is required (max 200 characters)': 'Nama wajib diisi (maksimal 200 karakter).',
  'name (max 200) and sku (max 64) are required; description max 2000': 'Nama dan SKU wajib diisi.',
  'quantity must be a positive integer': 'Jumlah harus bilangan bulat lebih dari 0.',
  'invalid JSON body': 'Data yang dikirim tidak valid.',
  'internal error': 'Terjadi gangguan di server. Coba lagi.',
};

export const msg = (e: unknown): string =>
  e instanceof ApiError ? e.message : e instanceof TypeError ? 'Tidak ada koneksi internet.' : (e as Error).message;

export async function api<T = any>(path: string, method = 'GET', body?: unknown): Promise<T> {
  const res = await fetch(BASE + '/api' + path, {
    method,
    credentials: BASE ? 'include' : 'same-origin',
    headers: body ? { 'Content-Type': 'application/json' } : {},
    body: body ? JSON.stringify(body) : undefined,
  });
  if (res.status === 401 && path !== '/me' && !path.startsWith('/auth/')) {
    location.reload(); // sesi habis: muat ulang menampilkan layar masuk
  }
  if (res.status === 204) return undefined as T;
  const data = await res.json().catch(() => ({}));
  if (!res.ok) throw new ApiError(ID[data.error] ?? data.error ?? res.statusText, res.status);
  return data as T;
}
