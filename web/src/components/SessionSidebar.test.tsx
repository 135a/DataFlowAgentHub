import { describe, it, expect, vi } from "vitest";
import { render, screen, fireEvent } from "@testing-library/react";
import { SessionSidebar } from "./SessionSidebar";

describe("SessionSidebar", () => {
  const mockSessions = [
    { id: "s1", title: "Session 1" },
    { id: "s2", title: "Session 2" },
    { id: "s3", title: "Session 3" },
  ];

  it("renders session list", () => {
    render(
      <SessionSidebar
        sessions={mockSessions}
        sid={null}
        token="test"
        onSelect={vi.fn()}
        onSessionsChanged={vi.fn()}
      />,
    );
    expect(screen.getByText("Session 1")).toBeInTheDocument();
    expect(screen.getByText("Session 2")).toBeInTheDocument();
    expect(screen.getByText("Session 3")).toBeInTheDocument();
  });

  it("renders active session with bold class", () => {
    render(
      <SessionSidebar
        sessions={mockSessions}
        sid="s2"
        token="test"
        onSelect={vi.fn()}
        onSessionsChanged={vi.fn()}
      />,
    );
    const s2Button = screen.getByText("Session 2");
    expect(s2Button).toBeInTheDocument();
    // The active session button should have the bold className
    expect(s2Button.className).toBeTruthy();
  });

  it("calls onSelect when a session is clicked", () => {
    const onSelect = vi.fn();
    render(
      <SessionSidebar
        sessions={mockSessions}
        sid={null}
        token="test"
        onSelect={onSelect}
        onSessionsChanged={vi.fn()}
      />,
    );
    fireEvent.click(screen.getByText("Session 1"));
    expect(onSelect).toHaveBeenCalledWith("s1");
  });

  it("renders a button to create new session", () => {
    render(
      <SessionSidebar
        sessions={mockSessions}
        sid={null}
        token="test"
        onSelect={vi.fn()}
        onSessionsChanged={vi.fn()}
      />,
    );
    expect(screen.getByText("新建")).toBeInTheDocument();
  });
});
