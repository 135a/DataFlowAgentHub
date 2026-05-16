import type { CSSProperties } from "react";

interface SkeletonProps {
  variant: "line" | "rect";
  width?: string | number;
  height?: string | number;
  style?: CSSProperties;
}

const pulseKeyframes = `
@keyframes sk-pulse {
  0% { opacity: 0.4; }
  50% { opacity: 0.8; }
  100% { opacity: 0.4; }
}
`;

// Inject keyframes once
let injected = false;
function injectStyles() {
  if (injected) return;
  injected = true;
  const style = document.createElement("style");
  style.textContent = pulseKeyframes;
  document.head.appendChild(style);
}

export function Skeleton({ variant, width, height, style }: SkeletonProps) {
  injectStyles();

  const base: CSSProperties = {
    background: "#e0e0e0",
    borderRadius: variant === "rect" ? 8 : 4,
    animation: "sk-pulse 1.5s ease-in-out infinite",
    width: width ?? (variant === "line" ? "100%" : 200),
    height: height ?? (variant === "line" ? 14 : 120),
    marginBottom: variant === "line" ? 8 : 12,
    ...style,
  };

  return <div style={base} />;
}

/** Pre-built page-level skeleton matching the app layout */
export function PageSkeleton() {
  return (
    <div style={{ fontFamily: "system-ui", padding: 16, maxWidth: 960, margin: "0 auto" }}>
      <Skeleton variant="line" width={200} height={24} />
      <Skeleton variant="line" width="60%" height={16} style={{ marginTop: 16 }} />
      <Skeleton variant="line" width="40%" height={16} />
      <Skeleton variant="rect" width="100%" height={300} style={{ marginTop: 24 }} />
    </div>
  );
}
