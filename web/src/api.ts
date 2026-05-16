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
  if (!(rest.body instanceof FormData)) {
    h.set("Content-Type", "application/json");
  }
  const r = await fetch(apiUrl(path), { ...rest, headers: h });
  if (r.status === 401) {
    handle401();
  }
  return r;
}

function handle401() {
  localStorage.removeItem("token");
  window.location.href = "/login";
}

/** Typed fetch: parse JSON response with type T. Throws on network error or non-2xx. */
export async function apiJson<T>(
  path: string,
  opts: RequestInit & { token?: string | null } = {}
): Promise<T> {
  let r: Response;
  try {
    r = await apiFetch(path, opts);
  } catch {
    throw new ApiError(0, "网络请求失败，请检查连接");
  }
  if (!r.ok) {
    const body = await r.text().catch(() => "");
    throw new ApiError(r.status, body || "服务异常，请稍后重试");
  }
  return r.json() as Promise<T>;
}

export class ApiError extends Error {
  constructor(public status: number, body: string) {
    super(`HTTP ${status}: ${body}`);
    this.name = "ApiError";
  }
}

export function getSSEUrl(sessionId: string, sseToken: string): string {
  return apiUrl(`/v1/sessions/${sessionId}/stream?token=${encodeURIComponent(sseToken)}`);
}
