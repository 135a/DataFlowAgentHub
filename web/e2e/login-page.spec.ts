import { test, expect } from "./fixtures";

test.describe("Login Page Rendering", () => {
  test("should display login form with default values", async ({ page }) => {
    await page.goto("/login");

    // Verify the page title
    await expect(page.locator("h1")).toHaveText("登录");

    // Verify email input has the default demo value
    const emailInput = page.locator('input[type="text"]').first();
    await expect(emailInput).toHaveValue("admin@demo.local");

    // Verify password input
    const passwordInput = page.locator('input[type="password"]');
    await expect(passwordInput).toHaveValue("changeme");

    // Verify submit button is present
    const submitButton = page.getByRole("button", { name: "进入" });
    await expect(submitButton).toBeVisible();
  });

  test("should show error message on failed login attempt", async ({ page }) => {
    // Mock the login endpoint to return a failure
    await page.route("**/v1/auth/login", (route) => {
      route.fulfill({
        status: 401,
        contentType: "application/json",
        body: JSON.stringify({ error: "invalid credentials" }),
      });
    });

    await page.goto("/login");

    // Click submit without changing defaults
    await page.getByRole("button", { name: "进入" }).click();

    // Verify error message is shown
    await expect(page.locator("text=登录失败")).toBeVisible();
  });

  test("should render with correct structure", async ({ page }) => {
    await page.goto("/login");

    // Check the hint text is present
    await expect(
      page.locator("text=开发环境默认走 Vite 代理到后端")
    ).toBeVisible();

    // Verify form has all expected elements
    const form = page.locator("form");
    await expect(form).toBeVisible();

    // Check labels
    await expect(page.locator("label", { hasText: "Email" })).toBeVisible();
    await expect(page.locator("label", { hasText: "密码" })).toBeVisible();
  });
});
