import { test, expect } from "./fixtures";
import { mockSessions, mockMessages } from "./mocks";

test.describe("Chat Flow", () => {
  test.beforeEach(async ({ page }) => {
    // Seed token for authenticated access
    await page.goto("/");
    const viewerToken =
      "eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9.eyJzdWIiOiJ2aWV3ZXItaWQiLCJyb2xlIjoidmlld2VyIiwid29ya3NwYWNlX2lkIjoiZGVmYXVsdCIsImV4cCI6OTk5OTk5OTk5OX0.dummy_viewer_token";
    const adminToken =
      "eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9.eyJzdWIiOiJhZG1pbi1pZCIsInJvbGUiOiJhZG1pbiIsIndvcmtzcGFjZV9pZCI6ImRlZmF1bHQiLCJleHAiOjk5OTk5OTk5OTl9.dummy_admin_token";
    const token = test.info().title.includes("[viewer]")
      ? viewerToken
      : adminToken;
    await page.evaluate((t) => localStorage.setItem("token", t), token);
  });

  test("should display sessions list [admin]", async ({ page }) => {
    mockSessions(page, [
      { id: "s1", title: "销售分析" },
      { id: "s2", title: "用户查询" },
    ]);
    mockMessages(page, []);

    await page.goto("/");
    await page.waitForLoadState("networkidle");

    // Should see session titles
    await expect(page.locator("text=销售分析")).toBeVisible();
    await expect(page.locator("text=用户查询")).toBeVisible();
  });

  test("should select a session and load messages [admin]", async ({ page }) => {
    mockSessions(page, [{ id: "s1", title: "销售分析" }]);
    mockMessages(page, [
      {
        id: "m1",
        role: "user",
        content: { text: "显示销售数据" },
        created_at: "2026-05-18T00:00:00Z",
      },
      {
        id: "m2",
        role: "assistant",
        content: {
          sql: "SELECT * FROM sales",
          rows: [{ region: "North", amount: 100 }],
        },
        created_at: "2026-05-18T00:00:01Z",
      },
    ]);

    await page.goto("/");

    // Click on the session
    await page.locator("button", { hasText: "销售分析" }).click();
    await page.waitForTimeout(500);

    // Should see the user message
    await expect(page.locator("text=显示销售数据")).toBeVisible();

    // Should see the SQL result
    await expect(page.locator("text=SELECT * FROM sales")).toBeVisible();
    await expect(page.locator("text=North")).toBeVisible();
    await expect(page.locator("text=100")).toBeVisible();
  });

  test("should send a message and show progress [admin]", async ({ page }) => {
    mockSessions(page, [{ id: "s1", title: "测试" }]);
    mockMessages(page, []);

    await page.goto("/");

    // Select the session
    await page.locator("button", { hasText: "测试" }).click();
    await page.waitForTimeout(300);

    // Type and send a message
    const input = page.locator('input[name="t"]');
    await input.fill("show me the data");
    await page.locator('button[type="submit"]').click();

    // Since we mocked POST to return 202 (async), show progress panel
    await expect(page.locator("text=SQL 生成")).toBeVisible();

    // Confirm send button is disabled during processing
    const sendButton = page.locator('button[type="submit"]');
    await expect(sendButton).toBeDisabled();
  });

  test("should create a new session [admin]", async ({ page }) => {
    mockSessions(page, []);

    await page.goto("/");

    // Click the "新建" button
    await page.locator("button", { hasText: "新建" }).click();

    // Since we mocked POST /v1/sessions to return a new session,
    // the sessions list should reload
    await page.waitForTimeout(300);
  });

  test("should display empty state when no messages [admin]", async ({ page }) => {
    mockSessions(page, [{ id: "s1", title: "空会话" }]);
    mockMessages(page, []);

    await page.goto("/");

    // Select the empty session
    await page.locator("button", { hasText: "空会话" }).click();
    await page.waitForTimeout(300);

    // Should see the mode description
    await expect(page.locator("text=深度分析")).toBeVisible();
  });
});
