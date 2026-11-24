// API client. Tokens live in localStorage; access tokens are refreshed transparently.

export const API_URL = process.env.NEXT_PUBLIC_API_URL ?? "http://localhost:8080";
const BASE = `${API_URL}/api/v1`;

export class ApiError extends Error {
  constructor(
    public status: number,
    public code: string,
    message: string,
    public fields?: Record<string, string>,
  ) {
    super(message);
  }
}

interface Session {
  access: string;
  refresh: string;
  expiresAt: number;
}

const KEY = "cosmetics.session";

export function getSession(): Session | null {
  if (typeof window === "undefined") return null;
  try {
    return JSON.parse(localStorage.getItem(KEY) ?? "null");
  } catch {
    return null;
  }
}

export function setSession(t: { accessToken: string; refreshToken: string; expiresIn: number }) {
  localStorage.setItem(
    KEY,
    JSON.stringify({ access: t.accessToken, refresh: t.refreshToken, expiresAt: Date.now() + t.expiresIn * 1000 }),
  );
  window.dispatchEvent(new Event("auth:change"));
}

export function clearSession() {
  localStorage.removeItem(KEY);
  window.dispatchEvent(new Event("auth:change"));
}

let refreshing: Promise<Session | null> | null = null;

function refreshSession(): Promise<Session | null> {
  if (refreshing) return refreshing;
  const s = getSession();
  if (!s) return Promise.resolve(null);
  refreshing = (async () => {
    try {
      const res = await fetch(`${BASE}/auth/refresh`, {
        method: "POST",
        headers: { "Content-Type": "application/json" },
        body: JSON.stringify({ refreshToken: s.refresh }),
      });
      if (!res.ok) {
        clearSession();
        return null;
      }
      setSession(await res.json());
      return getSession();
    } catch {
      return null; // offline: keep the session, try again later
    } finally {
      refreshing = null;
    }
  })();
  return refreshing;
}

interface Opts {
  method?: string;
  body?: unknown;
  form?: FormData;
  signal?: AbortSignal;
  anonymous?: boolean;
}

async function request(path: string, o: Opts): Promise<Response> {
  const send = (token?: string) => {
    const headers: Record<string, string> = {};
    if (token) headers.Authorization = `Bearer ${token}`;
    let body: BodyInit | undefined;
    if (o.form) body = o.form;
    else if (o.body !== undefined) {
      headers["Content-Type"] = "application/json";
      body = JSON.stringify(o.body);
    }
    return fetch(`${BASE}${path}`, { method: o.method ?? (body ? "POST" : "GET"), headers, body, signal: o.signal, cache: "no-store" });
  };
  let s = o.anonymous ? null : getSession();
  if (s && s.expiresAt - Date.now() < 30_000) s = (await refreshSession()) ?? s;
  let res = await send(s?.access);
  if (res.status === 401 && s) {
    const n = await refreshSession();
    if (n) res = await send(n.access);
  }
  if (!res.ok) {
    let j: { error?: { code?: string; message?: string; fields?: Record<string, string> } } = {};
    try {
      j = await res.json();
    } catch {}
    throw new ApiError(res.status, j.error?.code ?? "error", j.error?.message ?? "Xatolik yuz berdi", j.error?.fields);
  }
  return res;
}

export async function api<T = unknown>(path: string, o: Opts = {}): Promise<T> {
  const res = await request(path, o);
  if (res.status === 204) return undefined as T;
  return res.json();
}

/** Fetch a protected binary (e.g. a payment receipt) with the Bearer token. */
export async function apiBlob(path: string): Promise<Blob> {
  return (await request(path, {})).blob();
}

/** Server-side fetch of public data (used by server components). */
export async function publicGet<T>(path: string, revalidate = 30): Promise<T> {
  const base = process.env.API_URL ?? API_URL;
  const res = await fetch(`${base}/api/v1${path}`, { next: { revalidate } });
  if (!res.ok) throw new ApiError(res.status, "error", `API ${res.status}`);
  return res.json();
}

/** Turn an ApiError into a short message plus per-field messages for forms. */
export function errorInfo(e: unknown): { message: string; fields: Record<string, string> } {
  if (e instanceof ApiError) return { message: e.message, fields: e.fields ?? {} };
  return { message: "Tarmoq xatosi. Internetni tekshirib, qayta urinib ko'ring.", fields: {} };
}
