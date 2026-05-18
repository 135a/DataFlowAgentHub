import { test, expect } from "./fixtures";
import {
  mockDataSources,
  mockUsers,
  mockTables,
  mockSessions,
  mockMessages,
} from "./mocks";

test.describe("Navigation", () => {
  test.beforeEach(async ({ page }) => {
    // Seed admin token for full access
    const adminToken =
      "eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9.eyJzdWIiOiJhZG1pbi1pZCIsInJvbGUiOiJhZG1pbiIsIndvcmtzcGFjZV9pZCI6ImRlZmF1bHQiLCJleHAiOjk5OTk5OTk5OTl9.dummy_admin_token";
    await page.goto("/");
    await page.evaluate((t) => localStorage.setItem("token", t), adminToken);

    // Apply common mocks
    mockSessions(page, [{ id: "s1", title: "测试会话" }]);
    mockMessages(page, []);
    mockDataSources(page, [
      { id: "ds1", name: "测试数据库", kind: "postgres", host: "localhost", port: 5432, database: "testdb", has_password: true },
    ]);
    mockUsers(page, [
      { id: "u1", name: "管理员", phone: "13800138000", role: "admin", created_at: "2026-01-01T00:00:00Z" },
      { id: "u2", name: "操作员", phone: "", role: "operator", created_at: "2026-01-02T00:00:00Z" },
    ]);
    mockTables(page, [
      {
        name: "demo_sales",
        columns: [
          { name: "id", type: "integer", nullable: false },
          { name: "amount", type: "numeric", nullable: true },
        ],
        row_estimate: 1000,
      },
    ]);
  });

  test("should navigate to data-sources page from main app [admin]", async ({ page }) => {
    await page.goto("/");

    // Click the "数据源" navigation link
    await page.locator('a[href="/data-sources"]').click();
    await page.waitForURL("/data-sources");

    // Should see the data sources page
    await expect(page.locator("h1", { hasText: "数据源管理" })).toBeVisible();
    await expect(page.locator("text=测试数据库")).toBeVisible();
  });

  test("should navigate to tables page [admin]", async ({ page }) => {
    await page.goto("/");

    // Click the "数据库表" navigation link
    await page.locator('a[href="/tables"]').click();
    await page.waitForURL("/tables");

    // Should see the tables page
    await expect(page.locator("h1", { hasText: "数据库表结构" })).toBeVisible();
    await expect(page.locator("text=demo_sales")).toBeVisible();
  });

  test("should navigate to admin users page [admin]", async ({ page }) => {
    await page.goto("/");

    // Click the "用户管理" navigation link
    await page.locator('a[href="/admin/users"]').click();
    await page.waitForURL("/admin/users");

    // Should see the user management page
    await expect(page.locator("h1", { hasText: "用户管理" })).toBeVisible();
    await expect(page.locator("text=管理员")).toBeVisible();
    await expect(page.locator("text=操作员")).toBeVisible();
  });

  test("should navigate back to main page from sub-pages [admin]", async ({ page }) => {
    await page.goto("/data-sources");

    // Click the "返回" link to go back to home
    await page.locator('a[href="/"]').click();
    await page.waitForURL("/");
  });

  test("should navigate to knowledge page [admin]", async ({ page }) => {
    await page.goto("/");

    // Click the "知识库" navigation link
    await page.locator('a[href="/knowledge"]').click();
    await page.waitForURL("/knowledge");

    // Should see the knowledge page
    await expect(page.locator("h1", { hasText: "知识库管理" })).toBeVisible();
  });

  test("should logout and redirect to login [admin]", async ({ page }) => {
    await page.goto("/");

    // Click the "退出" link
    await page.locator('a[href="/login"]').click();
    await page.waitForURL("/login");

    // Token should be cleared
    const token = await page.evaluate(() => localStorage.getItem("token"));
    expect(token).toBeNull();
  });

  test("should not show admin link for viewer [viewer]", async ({ page }) => {
    const viewerToken =
      "eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9.eyJzdWIiOiJ2aWV3ZXItaWQiLCJyb2xlIjoidmlld2VyIiwid29ya3NwYWNlX2lkIjoiZGVmYXVsdCIsImV4cCI6OTk5OTk5OTk5OX0.dummy_viewer_token";
    await page.goto("/");
    await page.evaluate((t) => localStorage.setItem("token", t), viewerToken);

    await page.goto("/");

    // Admin link should not be visible
    await expect(page.locator('a[href="/admin/users"]')).not.toBeVisible();
  });
});
