import type { Page } from "@playwright/test";
import { TEST_TOKENS } from "./fixtures";

/**
 * Mock API handlers for E2E tests.
 * Uses Playwright's route interception to mock backend responses.
 */

export interface MockOptions {
  /** Page instance to attach mocks to */
  page: Page;
}

/** ----- Auth ----- */

export function mockLoginSuccess(page: Page, role: "admin" | "operator" | "viewer" = "admin") {
  return page.route("**/v1/auth/login", (route) => {
    route.fulfill({
      status: 200,
      contentType: "application/json",
      body: JSON.stringify({ access_token: TEST_TOKENS[role] }),
    });
  });
}

export function mockLoginFailure(page: Page) {
  return page.route("**/v1/auth/login", (route) => {
    route.fulfill({
      status: 401,
      contentType: "application/json",
      body: JSON.stringify({ error: "invalid credentials" }),
    });
  });
}

/** ----- Sessions ----- */

export function mockSessions(page: Page, sessions: { id: string; title: string }[] = []) {
  return page.route("**/v1/sessions", (route, request) => {
    if (request.method() === "GET") {
      route.fulfill({
        status: 200,
        contentType: "application/json",
        body: JSON.stringify({ sessions }),
      });
    } else if (request.method() === "POST") {
      route.fulfill({
        status: 200,
        contentType: "application/json",
        body: JSON.stringify({ id: "new-session-id", title: "新会话" }),
      });
    } else {
      route.fallback();
    }
  });
}

/** ----- Messages ----- */

export function mockMessages(page: Page, messages: Record<string, unknown>[] = []) {
  return page.route(/\/v1\/sessions\/[^/]+\/messages/, (route, request) => {
    if (request.method() === "GET") {
      route.fulfill({
        status: 200,
        contentType: "application/json",
        body: JSON.stringify({ messages, run_steps: [] }),
      });
    } else if (request.method() === "POST") {
      route.fulfill({
        status: 202,
        contentType: "application/json",
        body: JSON.stringify({ task_id: "test-task-id" }),
      });
    } else {
      route.fallback();
    }
  });
}

export function mockMessagesSync(page: Page, messages: Record<string, unknown>[] = []) {
  return page.route(/\/v1\/sessions\/[^/]+\/messages/, (route, request) => {
    if (request.method() === "POST") {
      route.fulfill({
        status: 200,
        contentType: "application/json",
        body: JSON.stringify({ messages }),
      });
    } else {
      route.fallback();
    }
  });
}

/** ----- SSE Token ----- */

export function mockSSEToken(page: Page) {
  return page.route(/\/v1\/sessions\/[^/]+\/sse-token/, (route) => {
    route.fulfill({
      status: 200,
      contentType: "application/json",
      body: JSON.stringify({ sse_token: "test-sse-token" }),
    });
  });
}

/** ----- SSE Stream ----- */

/**
 * Mock the SSE stream endpoint.
 * After route interception, creates a readable stream that sends events.
 */
export function mockSSEStream(page: Page, events: { event: string; data: string }[] = []) {
  return page.route(/\/v1\/sessions\/[^/]+\/stream/, (route) => {
    const encoder = new TextEncoder();
    const stream = new ReadableStream({
      start(controller) {
        const pushEvent = (evt: { event: string; data: string }) => {
          controller.enqueue(encoder.encode(`event: ${evt.event}\ndata: ${evt.data}\n\n`));
        };

        events.forEach(pushEvent);

        // Send a final "result" event to close the stream
        pushEvent({ event: "result", data: "{}" });
        controller.close();
      },
    });

    route.fulfill({
      status: 200,
      headers: {
        "Content-Type": "text/event-stream",
        "Cache-Control": "no-cache",
        Connection: "keep-alive",
      },
      body: stream,
    });
  });
}

/** ----- Data Sources ----- */

export function mockDataSources(page: Page, items: Record<string, unknown>[] = []) {
  return page.route("**/v1/data-sources", (route) => {
    route.fulfill({
      status: 200,
      contentType: "application/json",
      body: JSON.stringify({ items }),
    });
  });
}

/** ----- Users (admin) ----- */

export function mockUsers(page: Page, users: Record<string, unknown>[] = []) {
  return page.route("**/v1/users", (route, request) => {
    if (request.method() === "GET") {
      route.fulfill({
        status: 200,
        contentType: "application/json",
        body: JSON.stringify({ users }),
      });
    } else {
      route.fallback();
    }
  });
}

/** ----- Schema / Tables ----- */

export function mockTables(page: Page, tables: Record<string, unknown>[] = []) {
  return page.route("**/v1/schema/tables", (route) => {
    route.fulfill({
      status: 200,
      contentType: "application/json",
      body: JSON.stringify({ tables }),
    });
  });
}

/** ----- Knowledge Docs ----- */

export function mockKnowledgeDocs(page: Page, docs: Record<string, unknown>[] = []) {
  return page.route(/\/v1\/workspaces\/[^/]+\/knowledge\/docs/, (route) => {
    route.fulfill({
      status: 200,
      contentType: "application/json",
      body: JSON.stringify({ docs }),
    });
  });
}

/**
 * Apply all common mocks at once for a standard authenticated session.
 */
export async function applyAllMocks(page: Page, options?: { sessions?: number }) {
  const sessionCount = options?.sessions ?? 2;
  const mockSessionsList = Array.from({ length: sessionCount }, (_, i) => ({
    id: `session-${i + 1}`,
    title: i === 0 ? "测试会话 1" : "测试会话 2",
  }));

  mockSessions(page, mockSessionsList);
  mockMessages(page, [
    {
      id: "msg-1",
      role: "user",
      content: { text: "show me sales data" },
      created_at: "2026-05-18T00:00:00Z",
    },
    {
      id: "msg-2",
      role: "assistant",
      content: {
        sql: "SELECT * FROM sales",
        rows: [
          { region: "North", amount: 100 },
          { region: "South", amount: 200 },
        ],
      },
      created_at: "2026-05-18T00:00:01Z",
    },
  ]);
  mockSSEToken(page);
  mockDataSources(page);
  mockUsers(page);
  mockTables(page);
}
