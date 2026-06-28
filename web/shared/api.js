// Thin fetch helpers around the backend REST API.
import { API } from "./config.js";

export async function postJSON(path, body) {
  const res = await fetch(API + path, {
    method: "POST",
    headers: body ? { "Content-Type": "application/json" } : {},
    body: body ? JSON.stringify(body) : undefined,
  });
  if (!res.ok) throw new Error(`${res.status} ${await res.text()}`);
  const text = await res.text();
  return text ? JSON.parse(text) : {};
}

export async function getJSON(path) {
  const res = await fetch(API + path);
  if (!res.ok) throw new Error(`${res.status} ${await res.text()}`);
  return res.json();
}

// randomId returns a short readable id like "rider-7f3a".
export function randomId(prefix) {
  return `${prefix}-${Math.random().toString(16).slice(2, 6)}`;
}
