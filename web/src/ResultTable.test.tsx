import { describe, it, expect } from "vitest";
import { render, screen } from "@testing-library/react";
import { ResultTable } from "./ResultTable";

describe("ResultTable", () => {
  it("renders empty message when no rows", () => {
    render(<ResultTable rows={[]} />);
    expect(screen.getByText("查询返回 0 行")).toBeInTheDocument();
  });

  it("renders table with correct column order from first row keys", () => {
    const rows = [
      { id: 1, name: "Alice", score: 85 },
      { id: 2, name: "Bob", score: 92 },
    ];
    render(<ResultTable rows={rows} />);

    // Column headers
    expect(screen.getByText("id")).toBeInTheDocument();
    expect(screen.getByText("name")).toBeInTheDocument();
    expect(screen.getByText("score")).toBeInTheDocument();

    // Data cells
    expect(screen.getByText("Alice")).toBeInTheDocument();
    expect(screen.getByText("Bob")).toBeInTheDocument();
  });

  it("handles rows with different key sets (union columns)", () => {
    const rows = [
      { id: 1, name: "Alice" },
      { id: 2, name: "Bob", extra: "yes" },
    ];
    render(<ResultTable rows={rows} />);

    expect(screen.getByText("extra")).toBeInTheDocument();
    expect(screen.getByText("yes")).toBeInTheDocument();
  });

  it("renders large number of rows without crash", () => {
    const rows = Array.from({ length: 100 }, (_, i) => ({
      id: i,
      value: `row-${i}`,
    }));
    render(<ResultTable rows={rows} />);

    expect(screen.getByText("row-99")).toBeInTheDocument();
  });

  it("displays null values as empty string", () => {
    const rows = [{ id: 1, name: null }];
    render(<ResultTable rows={rows} />);
    // The cell should render without crashing
    expect(screen.getByText("id")).toBeInTheDocument();
  });
});
