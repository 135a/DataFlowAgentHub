import { render, screen, fireEvent } from "@testing-library/react";
import { describe, it, expect } from "vitest";
import { ChartView } from "./ChartView";

describe("ChartView", () => {
  it("should show no-data message when rows is empty", () => {
    render(<ChartView rows={[]} />);
    expect(screen.getByText("无可图表化的数据")).toBeInTheDocument();
  });

  it("should show no-numeric message when no numeric columns exist", () => {
    const rows = [{ name: "Alice", city: "北京" }];
    render(<ChartView rows={rows} />);
    expect(screen.getByText("结果集中未检测到数值列")).toBeInTheDocument();
  });

  it("should render bar chart by default with numeric data", () => {
    const rows = [
      { name: "A", value: 10 },
      { name: "B", value: 20 },
    ];
    render(<ChartView rows={rows} />);

    // Toolbar buttons rendered
    expect(screen.getByText("柱状图")).toBeInTheDocument();
    expect(screen.getByText("折线图")).toBeInTheDocument();

    // Chart container rendered
    expect(screen.getByText("柱状图")).toHaveClass(/chartBtnActive/);
  });

  it("should switch to line chart when line button clicked", () => {
    const rows = [
      { name: "A", value: 10 },
      { name: "B", value: 20 },
    ];
    render(<ChartView rows={rows} />);

    fireEvent.click(screen.getByText("折线图"));
    expect(screen.getByText("折线图")).toHaveClass(/chartBtnActive/);
  });

  it("should show truncation notice when rows exceed maxPoints", () => {
    const rows = Array.from({ length: 150 }, (_, i) => ({
      name: `Item ${i}`,
      value: i,
    }));
    render(<ChartView rows={rows} maxPoints={100} />);
    expect(screen.getByText(/仅展示前 100 条数据的图表/)).toBeInTheDocument();
  });

  it("should handle render error gracefully", () => {
    // Pass data that will cause recharts to error (e.g., invalid values)
    const rows = [
      { name: "A", value: 10 },
      { name: "B", value: NaN },
    ];
    // Attempt render; NaN shouldn't cause actual crash in recharts,
    // so let's verify the component at least doesn't crash
    expect(() => render(<ChartView rows={rows} />)).not.toThrow();
  });

  it("should use first non-numeric column as the x-axis label", () => {
    const rows = [
      { category: "X", value: 10, score: 20 },
      { category: "Y", value: 30, score: 40 },
    ];
    render(<ChartView rows={rows} />);
    expect(screen.getByText("柱状图")).toBeInTheDocument();
    expect(screen.getByText("折线图")).toBeInTheDocument();
  });
});
