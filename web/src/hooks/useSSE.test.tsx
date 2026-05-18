import { describe, it, expect, vi, beforeEach, afterEach } from "vitest";
import { renderHook, act } from "@testing-library/react";
import { useSSE } from "./useSSE";

describe("useSSE", () => {
  const token = "test-token";
  const mockCallbacks = {
    onResult: vi.fn(),
    onAgentStep: vi.fn(),
    onSqlGenerated: vi.fn(),
    onError: vi.fn(),
  };

  beforeEach(() => {
    vi.clearAllMocks();
    // Mock fetch for the SSE token request
    global.fetch = vi.fn().mockResolvedValue({
      ok: true,
      json: () => Promise.resolve({ sse_token: "sse-token-123" }),
    } as Response);
  });

  afterEach(() => {
    vi.restoreAllMocks();
  });

  it("provides startSSE and stopSSE functions", () => {
    const { result } = renderHook(() => useSSE(token, mockCallbacks));
    expect(result.current.startSSE).toBeDefined();
    expect(result.current.stopSSE).toBeDefined();
  });

  it("fetches SSE token on startSSE", async () => {
    const { result } = renderHook(() => useSSE(token, mockCallbacks));
    await act(async () => {
      result.current.startSSE("session-1");
    });
    expect(global.fetch).toHaveBeenCalledWith(
      "/v1/sessions/session-1/sse-token",
      expect.objectContaining({
        method: "POST",
      }),
    );
  });

  it("handles fetch failure gracefully", async () => {
    global.fetch = vi.fn().mockRejectedValue(new Error("Network error"));
    const { result } = renderHook(() => useSSE(token, mockCallbacks));
    await act(async () => {
      result.current.startSSE("session-1");
    });
    // Should not throw; the hook handles errors internally
    expect(result.current.startSSE).toBeDefined();
  });

  it("provides stopSSE that is a function", () => {
    const { result } = renderHook(() => useSSE(token, mockCallbacks));
    expect(typeof result.current.stopSSE).toBe("function");
  });
});
