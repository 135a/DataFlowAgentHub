import { useMemo } from "react";
import { useNavigate } from "react-router-dom";

interface JwtPayload {
  workspace_id?: string;
  sub?: string;
  exp?: number;
}

function parseJwtPayload(token: string): JwtPayload | null {
  try {
    const parts = token.split(".");
    if (parts.length !== 3) return null;
    const payload = JSON.parse(atob(parts[1]));
    return payload as JwtPayload;
  } catch {
    return null;
  }
}

/** Return the workspace_id from the JWT token in localStorage. Redirects to /login if missing. */
export function useWorkspaceId(): string {
  const navigate = useNavigate();

  return useMemo(() => {
    const token = localStorage.getItem("token");
    if (!token) {
      navigate("/login", { replace: true });
      return "";
    }
    const payload = parseJwtPayload(token);
    if (!payload?.workspace_id) {
      navigate("/login", { replace: true });
      return "";
    }
    return payload.workspace_id;
  }, [navigate]);
}

/** Non-hook version for use outside React components. */
export function getWorkspaceIdFromToken(): string | null {
  const token = localStorage.getItem("token");
  if (!token) return null;
  const payload = parseJwtPayload(token);
  return payload?.workspace_id ?? null;
}
