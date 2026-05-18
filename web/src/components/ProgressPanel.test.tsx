import { describe, it, expect } from "vitest";
import { render, screen } from "@testing-library/react";
import { ProgressPanel } from "./ProgressPanel";
import type { StepDef, StepState } from "./ProgressPanel";

describe("ProgressPanel", () => {
  const steps: StepDef[] = [
    { name: "SQL 生成", weight: 0.7 },
    { name: "执行查询", weight: 0.3 },
  ];

  it("renders progress steps with correct names", () => {
    const states: StepState[] = [
      { status: "running", durationMs: 500 },
      { status: "waiting", durationMs: 0 },
    ];

    render(
      <ProgressPanel
        steps={steps}
        stepStates={states}
        elapsedMs={1500}
        estimatedRemainingMs={null}
      />,
    );

    // Use getByText with a function to find text containing the step name
    expect(screen.getByText((content) => content.includes("SQL 生成"))).toBeInTheDocument();
    expect(screen.getByText((content) => content.includes("执行查询"))).toBeInTheDocument();
  });

  it("displays elapsed time", () => {
    const states: StepState[] = [
      { status: "running", durationMs: 2000 },
      { status: "waiting", durationMs: 0 },
    ];

    render(
      <ProgressPanel
        steps={steps}
        stepStates={states}
        elapsedMs={2500}
        estimatedRemainingMs={null}
      />,
    );

    expect(screen.getByText((content) => content.includes("已用"))).toBeInTheDocument();
    expect(screen.getByText((content) => content.includes("2.5s"))).toBeInTheDocument();
  });

  it("displays estimated remaining time when provided", () => {
    const states: StepState[] = [
      { status: "running", durationMs: 1000 },
      { status: "waiting", durationMs: 0 },
    ];

    render(
      <ProgressPanel
        steps={steps}
        stepStates={states}
        elapsedMs={1000}
        estimatedRemainingMs={5000}
      />,
    );

    expect(screen.getByText((content) => content.includes("预计剩余"))).toBeInTheDocument();
  });

  it("shows completed state for all steps", () => {
    const states: StepState[] = [
      { status: "completed", durationMs: 2000 },
      { status: "completed", durationMs: 500 },
    ];

    render(
      <ProgressPanel
        steps={steps}
        stepStates={states}
        elapsedMs={2500}
        estimatedRemainingMs={0}
      />,
    );

    expect(screen.getByText("处理进度")).toBeInTheDocument();
  });
});
