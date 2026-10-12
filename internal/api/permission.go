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

// effectivePermissions 返回用户在数据库中的有效权限点并集（优先读缓存，miss 则查库回填）。
func effectivePermissions(db *gorm.DB, username string) []string {
	if codes, ok := cachedEffectivePermissions(username); ok {
		return codes
	}
	// 查库前捕获 epoch，回填用它：若 RBAC 写在查库期间发生，回填的是旧 epoch，
	// 读取时会因 epoch 失配而回源，避免「新 epoch + 旧 codes」的脏组合被服务。
	epoch, _ := permEpoch()
	codes := queryEffectivePermissions(db, username)
	cacheEffectivePermissions(username, codes, epoch)
	return codes
}

// queryEffectivePermissions 用一次 JOIN 查用户的有效权限码（避免 N+1）。
func queryEffectivePermissions(db *gorm.DB, username string) []string {
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
