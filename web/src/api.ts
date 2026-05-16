const base = import.meta.env.VITE_API_BASE_URL || "";

export function apiUrl(path: string): string {
  if (base) return `${base.replace(/\/$/, "")}${path}`;
  return path;
}

export async function apiFetch(
  path: string,
  opts: RequestInit & { token?: string | null } = {}
): Promise<Response> {
  const { token, headers, ...rest } = opts;
  const h = new Headers(headers);
  if (token) h.set("Authorization", `Bearer ${token}`);
  h.set("Content-Type", "application/json");
  return fetch(apiUrl(path), { ...rest, headers: h });
}

export function getSSEUrl(sessionId: string, sseToken: string): string {
  return apiUrl(`/v1/sessions/${sessionId}/stream?token=${encodeURIComponent(sseToken)}`);
}
