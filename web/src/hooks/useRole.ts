import { useEffect, useState } from "react";
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

/** 返回当前用户的角色（super_admin/data_admin/normal_user/read_only_visitor），未登录时返回 "" */
export function useRole(): string {
  const navigate = useNavigate();
  const [role, setRole] = useState<string>("");

  useEffect(() => {
    const token = localStorage.getItem("token");
    if (!token) {
      navigate("/login", { replace: true });
      setRole("");
      return;
    }
    const payload = parseJwtPayload(token);
    setRole(payload?.role ?? "");
  }, [navigate]);

  return role;
}

/** 角色等级映射 */
const ROLE_ORDER: Record<string, number> = {
  read_only_visitor: 1,
  normal_user: 2,
  data_admin: 3,
  super_admin: 4,
};

/** 判断当前用户是否至少为某角色等级 */
export function useMinRole(minRole: string): boolean {
  const role = useRole();
  return (ROLE_ORDER[role] || 0) >= (ROLE_ORDER[minRole] || 99);
}

/** 判断是否为 super_admin */
export function useIsSuperAdmin(): boolean {
  return useMinRole("super_admin");
}

/** 判断是否为 data_admin+ */
export function useIsDataAdmin(): boolean {
  return useMinRole("data_admin");
}

/** 判断是否为 normal_user（不含 data_admin+） */
export function useIsNormalUser(): boolean {
  const role = useRole();
  return role === "normal_user";
}

/** 判断是否为 admin（向后兼容，等价于 useIsSuperAdmin） */
export function useIsAdmin(): boolean {
  return useMinRole("super_admin");
}

/** 判断是否为 operator+（向后兼容，等价于 useIsDataAdmin） */
export function useIsOperator(): boolean {
  return useMinRole("data_admin");
}
