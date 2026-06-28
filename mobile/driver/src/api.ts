import { API_BASE } from "./config";

export async function postJSON<T = any>(path: string, body?: unknown): Promise<T> {
  const res = await fetch(API_BASE + path, {
    method: "POST",
    headers: body ? { "Content-Type": "application/json" } : undefined,
    body: body ? JSON.stringify(body) : undefined,
  });
  if (!res.ok) throw new Error(`${res.status} ${await res.text()}`);
  const text = await res.text();
  return (text ? JSON.parse(text) : {}) as T;
}

export async function getJSON<T = any>(path: string): Promise<T> {
  const res = await fetch(API_BASE + path);
  if (!res.ok) throw new Error(`${res.status} ${await res.text()}`);
  return (await res.json()) as T;
}

// randomId returns a short readable id like "rider-7f3a".
export function randomId(prefix: string): string {
  return `${prefix}-${Math.random().toString(16).slice(2, 6)}`;
}
