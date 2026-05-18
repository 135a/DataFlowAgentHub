import { test as base, type Page, type BrowserContext } from "@playwright/test";

/**
 * Role-based JWT tokens for testing.
 * These are pre-encoded JWTs with the following claims:
 * - admin:  { sub: "admin-id", role: "admin", workspace_id: "default", exp: 9999999999 }
 * - operator: { sub: "op-id", role: "operator", workspace_id: "default", exp: 9999999999 }
 * - viewer: { sub: "viewer-id", role: "viewer", workspace_id: "default", exp: 9999999999 }
 *
 * In a real scenario these would be obtained via /v1/auth/login.
 * For testing with mocked API, use these to simulate auth state.
 */
export const TEST_TOKENS = {
  admin:
    "eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9.eyJzdWIiOiJhZG1pbi1pZCIsInJvbGUiOiJhZG1pbiIsIndvcmtzcGFjZV9pZCI6ImRlZmF1bHQiLCJleHAiOjk5OTk5OTk5OTl9.dummy_admin_token",
  operator:
    "eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9.eyJzdWIiOiJvcC1pZCIsInJvbGUiOiJvcGVyYXRvciIsIndvcmtzcGFjZV9pZCI6ImRlZmF1bHQiLCJleHAiOjk5OTk5OTk5OTl9.dummy_operator_token",
  viewer:
    "eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9.eyJzdWIiOiJ2aWV3ZXItaWQiLCJyb2xlIjoidmlld2VyIiwid29ya3NwYWNlX2lkIjoiZGVmYXVsdCIsImV4cCI6OTk5OTk5OTk5OX0.dummy_viewer_token",
};

/** Default test user credentials matching the seeded demo user */
export const TEST_CREDENTIALS = {
  email: "admin@demo.local",
  password: "changeme",
};

// Extend the base test with custom fixtures
export const test = base.extend<{
  /** Page that is already logged in with the given role */
  authenticatedPage: Page;
  /** Logged-in browser context */
  authContext: BrowserContext;
}>({
  authContext: async ({ browser }, use) => {
    const context = await browser.newContext();
    await use(context);
    await context.close();
  },

  authenticatedPage: async ({ browser }, use, testInfo) => {
    // Determine role from test title tag or default to admin
    const role = testInfo.title.match(/\[(\w+)\]/)?.[1] || "admin";
    const token = TEST_TOKENS[role as keyof typeof TEST_TOKENS] || TEST_TOKENS.admin;

    const context = await browser.newContext();
    const page = await context.newPage();

    // Seed localStorage with token before navigating
    await page.goto("/");
    await page.evaluate((t) => {
      localStorage.setItem("token", t);
    }, token);

    await use(page);
    await context.close();
  },
});

export { expect } from "@playwright/test";
