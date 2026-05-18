import { test, expect, TEST_TOKENS } from "./fixtures";
import { mockLoginSuccess, mockLoginFailure, mockSessions } from "./mocks";

test.describe("Login Flow", () => {
  test.beforeEach(async ({ page }) => {
    // Mock sessions endpoint so the main page doesn't fail after login
    mockSessions(page, [
      { id: "session-1", title: "测试会话" },
    ]);
  });

  test("should login successfully as admin [admin]", async ({ page }) => {
    mockLoginSuccess(page, "admin");

    await page.goto("/login");
    await page.getByRole("button", { name: "进入" }).click();

    // After successful login, should redirect to home (App)
    await expect(page).toHaveURL("/");
    // Token should be stored in localStorage
    const token = await page.evaluate(() => localStorage.getItem("token"));
    expect(token).toBeTruthy();
  });

  test("should login successfully as operator [operator]", async ({ page }) => {
    mockLoginSuccess(page, "operator");

    await page.goto("/login");
    await page.getByRole("button", { name: "进入" }).click();

    await expect(page).toHaveURL("/");
    const token = await page.evaluate(() => localStorage.getItem("token"));
    expect(token).toBe(TEST_TOKENS.operator);
  });

  test("should login successfully as viewer [viewer]", async ({ page }) => {
    mockLoginSuccess(page, "viewer");

    await page.goto("/login");
    await page.getByRole("button", { name: "进入" }).click();

    await expect(page).toHaveURL("/");
    const token = await page.evaluate(() => localStorage.getItem("token"));
    expect(token).toBe(TEST_TOKENS.viewer);
  });

  test("should show error on invalid credentials", async ({ page }) => {
    mockLoginFailure(page);

    await page.goto("/login");
    await page.getByRole("button", { name: "进入" }).click();

    // Should stay on login page
    await expect(page).toHaveURL("/login");
    // Error message should be visible
    await expect(page.locator("text=登录失败")).toBeVisible();
  });

  test("should redirect to login when accessing protected page without token", async ({ page }) => {
    // Clear any existing token
    await page.goto("/");
    await page.evaluate(() => localStorage.removeItem("token"));

    // Navigate to a protected route without token
    await page.goto("/data-sources");

    // Should be redirected to login
    await expect(page).toHaveURL("/login");
  });
});
