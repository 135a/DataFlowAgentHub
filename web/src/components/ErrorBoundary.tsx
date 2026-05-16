import { Component, type ErrorInfo, type ReactNode } from "react";

interface Props {
  children: ReactNode;
}

interface State {
  hasError: boolean;
  error: Error | null;
}

export class ErrorBoundary extends Component<Props, State> {
  constructor(props: Props) {
    super(props);
    this.state = { hasError: false, error: null };
  }

  static getDerivedStateFromError(error: Error): State {
    return { hasError: true, error };
  }

  componentDidCatch(error: Error, info: ErrorInfo) {
    console.error("ErrorBoundary caught:", error, info);
  }

  handleRetry = () => {
    this.setState({ hasError: false, error: null });
  };

  render() {
    if (this.state.hasError) {
      return (
        <div
          style={{
            fontFamily: "system-ui",
            maxWidth: 480,
            margin: "80px auto",
            padding: 32,
            textAlign: "center",
          }}
        >
          <h1 style={{ fontSize: 20, marginBottom: 8 }}>页面出错了</h1>
          <p style={{ color: "#666", fontSize: 14, marginBottom: 16 }}>
            {this.state.error?.message || "未知错误"}
          </p>
          {import.meta.env.DEV && this.state.error?.stack && (
            <pre
              style={{
                textAlign: "left",
                fontSize: 11,
                background: "#f5f5f5",
                padding: 12,
                borderRadius: 6,
                overflow: "auto",
                maxHeight: 200,
                marginBottom: 16,
              }}
            >
              {this.state.error.stack}
            </pre>
          )}
          <button
            onClick={this.handleRetry}
            style={{
              padding: "8px 24px",
              fontSize: 14,
              cursor: "pointer",
            }}
          >
            重试
          </button>
        </div>
      );
    }

    return this.props.children;
  }
}
