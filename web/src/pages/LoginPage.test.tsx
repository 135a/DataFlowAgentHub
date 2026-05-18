import { describe, it, expect, vi } from "vitest";
import { render, screen, fireEvent } from "@testing-library/react";
import { LoginPage } from "../pages/LoginPage";

describe("LoginPage", () => {
  beforeEach(() => {
    localStorage.clear();
    global.fetch = vi.fn();
  });

  it("renders login form with pre-filled demo credentials", () => {
    render(<LoginPage />);
    const emailInput = screen.getByDisplayValue("admin@demo.local") as HTMLInputElement;
    expect(emailInput).toBeInTheDocument();
  });

  it("shows error on failed login", async () => {
    global.fetch = vi.fn().mockResolvedValue({
      ok: false,
      status: 401,
    } as Response);

    render(<LoginPage />);
    fireEvent.click(screen.getByText("进入"));

    // Wait for async login to fail
    await vi.waitFor(() => {
      expect(screen.getByText("登录失败")).toBeInTheDocument();
    });
  });

  it("stores token on successful login", async () => {
    global.fetch = vi.fn().mockResolvedValue({
      ok: true,
      json: () => Promise.resolve({ access_token: "jwt-token" }),
    } as Response);

    render(<LoginPage />);
    fireEvent.click(screen.getByText("进入"));

    await vi.waitFor(() => {
      expect(localStorage.getItem("token")).toBe("jwt-token");
    });
  });
});
