import { render, screen, fireEvent } from "@testing-library/react";
import { describe, it, expect, vi, beforeEach, afterEach } from "vitest";
import type { ReactNode } from "react";
import { ErrorBoundary } from "./ErrorBoundary";

// Suppress console.error during error boundary tests
beforeEach(() => {
  vi.spyOn(console, "error").mockImplementation(() => {});
});

afterEach(() => {
  vi.restoreAllMocks();
});

describe("ErrorBoundary", () => {
  it("should render children when there is no error", () => {
    render(
      <ErrorBoundary>
        <div>正常内容</div>
      </ErrorBoundary>
    );
    expect(screen.getByText("正常内容")).toBeInTheDocument();
  });

  it("should render fallback UI when a child throws", () => {
    const Bomb = (): ReactNode => {
      throw new Error("测试错误");
    };

    render(
      <ErrorBoundary>
        <Bomb />
      </ErrorBoundary>
    );

    expect(screen.getByText("页面出错了")).toBeInTheDocument();
    expect(screen.getByText("测试错误")).toBeInTheDocument();
    expect(screen.getByText("重试")).toBeInTheDocument();
  });

  it("should recover after clicking retry button", () => {
    let shouldThrow = true;

    const FlakyComponent = (): ReactNode => {
      if (shouldThrow) {
        throw new Error("暂时错误");
      }
      return <div>恢复成功</div>;
    };

    render(
      <ErrorBoundary>
        <FlakyComponent />
      </ErrorBoundary>
    );

    // Error state shown
    expect(screen.getByText("页面出错了")).toBeInTheDocument();

    // Fix the flaky component and retry
    shouldThrow = false;
    fireEvent.click(screen.getByText("重试"));

    expect(screen.getByText("恢复成功")).toBeInTheDocument();
  });
});
