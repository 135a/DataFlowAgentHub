import { describe, it, expect, vi, beforeEach } from "vitest";
import { apiFetch, apiJson, ApiError } from "./api";

import { describe, it, expect, vi, beforeEach } from "vitest";
import { apiFetch, apiJson, ApiError } from "./api";

const mockFetch = vi.fn();
vi.stubGlobal("fetch", mockFetch);

beforeEach(() => {
  vi.clearAllMocks();
  localStorage.clear();
});

describe("apiFetch", () => {
  it("should add auth header when token exists", async () => {
    mockFetch.mockResolvedValueOnce(new Response(JSON.stringify({ ok: true }), { status: 200 }));

    const res = await apiFetch("/v1/test", { token: "test-token" });
    expect(res.status).toBe(200);

    const callArgs = mockFetch.mock.calls[0];
    const headers = new Headers(callArgs[1].headers as Record<string, string>);
    expect(headers.get("Authorization")).toBe("Bearer test-token");
  });

  it("should not add auth header when token is empty", async () => {
    mockFetch.mockResolvedValueOnce(new Response(JSON.stringify({ ok: true }), { status: 200 }));

    await apiFetch("/v1/test");
    const callArgs = mockFetch.mock.calls[0];
    // headers should not contain Authorization since no token passed
    const headers = callArgs[1].headers as Record<string, string>;
    expect(headers).not.toHaveProperty("Authorization");
  });

  it("should set Content-Type to application/json for non-FormData", async () => {
    mockFetch.mockResolvedValueOnce(new Response(JSON.stringify({ ok: true }), { status: 200 }));

    await apiFetch("/v1/test", { method: "POST", body: JSON.stringify({ a: 1 }) });
    const callArgs = mockFetch.mock.calls[0];
    const headers = new Headers(callArgs[1].headers as Record<string, string>);
    expect(headers.get("Content-Type")).toBe("application/json");
  });

  it("should not set Content-Type for FormData", async () => {
    mockFetch.mockResolvedValueOnce(new Response(JSON.stringify({ ok: true }), { status: 200 }));
    const fd = new FormData();
    fd.append("key", "value");

    await apiFetch("/v1/test", { method: "POST", body: fd });
    const callArgs = mockFetch.mock.calls[0];
    const headers = new Headers(callArgs[1].headers as Record<string, string>);
    expect(headers.get("Content-Type")).toBeNull();
  });

  it("should handle 401 and call handle401 (remove token, redirect)", async () => {
    const originalLocation = window.location;
    // @ts-expect-error - mocking location href
    delete window.location;
    // @ts-expect-error - partial location mock
    window.location = { href: "" } as unknown as Location;

    localStorage.setItem("token", "expired-token");
    mockFetch.mockResolvedValueOnce(new Response("Unauthorized", { status: 401 }));

    await apiFetch("/v1/test");
    expect(localStorage.getItem("token")).toBeNull();
    expect(window.location.href).toBe("/login");

    window.location = originalLocation;
  });
});

describe("apiJson", () => {
  it("should parse JSON response on success", async () => {
    mockFetch.mockResolvedValueOnce(new Response(JSON.stringify({ data: "hello" }), { status: 200 }));

    interface TestResp { data: string }
    const result = await apiJson<TestResp>("/v1/test");
    expect(result.data).toBe("hello");
  });

  it("should throw ApiError on non-2xx response", async () => {
    mockFetch.mockResolvedValue(new Response("Not Found", { status: 404 }));

    const err = await apiJson("/v1/not-found").catch((e) => e);
    expect(err).toBeInstanceOf(ApiError);
    expect((err as ApiError).status).toBe(404);
    expect((err as ApiError).message).toContain("HTTP 404");
  });

  it("should throw ApiError with network error message on fetch failure", async () => {
    mockFetch.mockRejectedValueOnce(new TypeError("Failed to fetch"));

    const err = await apiJson("/v1/network-error").catch((e) => e);
    expect(err).toBeInstanceOf(ApiError);
    expect((err as ApiError).status).toBe(0);
    expect((err as ApiError).message).toContain("网络请求失败");
  });
});
