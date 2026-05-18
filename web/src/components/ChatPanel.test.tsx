import { describe, it, expect } from "vitest";
import { render, screen } from "@testing-library/react";
import { MessageBlock, MessageBody } from "./ChatPanel";
import type { ApiMessage } from "../types/api";

describe("MessageBlock", () => {
  it("renders user message bubble with role and timestamp", () => {
    const msg: ApiMessage = {
      id: "1",
      role: "user",
      content: { text: "Hello" },
      created_at: "2026-01-01T00:00:00Z",
    };
    render(<MessageBlock msg={msg} />);
    expect(screen.getByText("Hello")).toBeInTheDocument();
    expect(screen.getByText(/user/)).toBeInTheDocument();
  });

  it("renders assistant message bubble", () => {
    const msg: ApiMessage = {
      id: "2",
      role: "assistant",
      content: { text: "Here are the results" },
      created_at: "2026-01-01T00:00:01Z",
    };
    render(<MessageBlock msg={msg} />);
    expect(screen.getByText("Here are the results")).toBeInTheDocument();
  });

  it("renders empty state for null content", () => {
    const msg: ApiMessage = {
      id: "3",
      role: "assistant",
      content: null as unknown as ApiMessage["content"],
      created_at: "2026-01-01T00:00:02Z",
    };
    render(<MessageBlock msg={msg} />);
    expect(screen.getByText("（空）")).toBeInTheDocument();
  });
});

describe("MessageBody", () => {
  it("renders text content", () => {
    render(<MessageBody content={{ text: "Hello world" }} />);
    expect(screen.getByText("Hello world")).toBeInTheDocument();
  });

  it("renders error content with error style", () => {
    render(<MessageBody content={{ error: "Something went wrong", code: "ERR001" }} />);
    expect(screen.getByText("错误")).toBeInTheDocument();
    expect(screen.getByText("Something went wrong")).toBeInTheDocument();
    expect(screen.getByText("ERR001")).toBeInTheDocument();
  });

  it("renders SQL result block with table and chart toggle", () => {
    const content = {
      sql: "SELECT * FROM users",
      rows: [
        { id: 1, name: "Alice" },
        { id: 2, name: "Bob" },
      ],
    };
    render(<MessageBody content={content} />);
    expect(screen.getByText(/SELECT/)).toBeInTheDocument();
    expect(screen.getByText("Alice")).toBeInTheDocument();
    expect(screen.getByText("Bob")).toBeInTheDocument();
  });

  it("renders final report with download link", () => {
    const content = {
      final_report: { final_report: "Analysis complete" },
      run_id: "run-123",
    };
    render(<MessageBody content={content} />);
    expect(screen.getByText("生成报告")).toBeInTheDocument();
    expect(screen.getByText("Analysis complete")).toBeInTheDocument();
    const link = screen.getByText("下载 Excel 报告");
    expect(link).toBeInTheDocument();
    expect(link).toHaveAttribute("href", "/api/v1/runs/run-123/report");
  });
});
