// Package rbac 提供统一的角色权限常量与判断函数。
// 所有需要角色等级比较的包（middleware、sqlrun 等）应引用此包，
// 避免在多处重复定义 roleOrder 映射导致逻辑不一致。
package rbac

// RoleOrder 定义角色等级顺序，数值越大权限越高。
// 等级顺序：read_only_visitor < normal_user < data_admin < super_admin
var RoleOrder = map[string]int{
	"read_only_visitor": 1,
	"normal_user":       2,
	"data_admin":        3,
	"super_admin":       4,
}

// MinRole 返回一个判断函数，检查 userRole 是否 >= required 角色等级。
// 如果 required 不是已知角色，返回 false。
// 如果 userRole 不是已知角色，返回 false（未知角色视为无权限）。
func MinRole(required string) func(string) bool {
	requiredLevel, ok := RoleOrder[required]
	if !ok {
		// required 不是有效角色，返回始终拒绝的函数
		return func(userRole string) bool {
			return false
		}
	}

	return func(userRole string) bool {
		userLevel, ok := RoleOrder[userRole]
		if !ok {
			return false
		}
		return userLevel >= requiredLevel
	}
}

// IsSuperAdmin 检查角色是否为超级管理员。
func IsSuperAdmin(role string) bool { return role == "super_admin" }

// IsDataAdmin 检查角色是否为数据管理员。
func IsDataAdmin(role string) bool { return role == "data_admin" }

// IsNormalUser 检查角色是否为普通用户。
func IsNormalUser(role string) bool { return role == "normal_user" }

// IsReadOnlyVisitor 检查角色是否为只读访客。
func IsReadOnlyVisitor(role string) bool { return role == "read_only_visitor" }
