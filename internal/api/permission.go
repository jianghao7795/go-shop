package api

import (
	"slices"

	"gorm.io/gorm"

	"shop/internal/model"
)

// hasPermission 判断权限列表是否包含某权限点。
func hasPermission(perms []string, perm string) bool {
	return slices.Contains(perms, perm)
}

// effectivePermissions 返回用户在数据库中的有效权限点并集（一次 JOIN，避免 N+1）。
func effectivePermissions(db *gorm.DB, username string) []string {
	var codes []string
	db.Model(&model.Permission{}).
		Select("DISTINCT shop_permissions.code").
		Joins("JOIN shop_role_permissions rp ON rp.permission_id = shop_permissions.id").
		Joins("JOIN shop_user_roles ur ON ur.role_id = rp.role_id").
		Joins("JOIN shop_users u ON u.id = ur.user_id").
		Where("u.username = ?", username).
		Scan(&codes)
	return codes
}
