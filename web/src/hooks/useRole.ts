import { useMemo } from "react";
import { useNavigate } from "react-router-dom";

interface JwtPayload {
  workspace_id?: string;
  role?: string;
  sub?: string;
  exp?: number;
}

function parseJwtPayload(token: string): JwtPayload | null {
  try {
    const parts = token.split(".");
    if (parts.length !== 3) return null;
    return JSON.parse(atob(parts[1])) as JwtPayload;
  } catch {
    return null;
  }
}

/** 返回当前用户的角色（admin/operator/viewer），未登录时返回 "" */
export function useRole(): string {
  const navigate = useNavigate();

  return useMemo(() => {
    const token = localStorage.getItem("token");
    if (!token) {
      navigate("/login", { replace: true });
      return "";
    }
    const payload = parseJwtPayload(token);
    if (!payload?.role) return "";
    return payload.role;
  }, [navigate]);
}

/** 判断当前用户是否至少为某角色等级 */
export function useMinRole(minRole: string): boolean {
  const role = useRole();
  const order: Record<string, number> = { viewer: 1, operator: 2, admin: 3 };
  return (order[role] || 0) >= (order[minRole] || 99);
}

/** 判断是否为 admin */
export function useIsAdmin(): boolean {
  return useMinRole("admin");
}

/** 判断是否为 operator+ */
export function useIsOperator(): boolean {
  return useMinRole("operator");
}
