package api

import (
	"gorm.io/gorm"

	"shop/internal/model"
)

// hasPermission 判断权限列表是否包含某权限点。
func hasPermission(perms []string, perm string) bool {
	for _, p := range perms {
		if p == perm {
			return true
		}
	}
	return false
}

// effectivePermissions 返回用户在数据库中的有效权限点并集。
func effectivePermissions(db *gorm.DB, username string) []string {
	var user model.User
	if db.Where("username = ?", username).First(&user).Error != nil {
		return nil
	}
	var roleIDs []uint
	db.Model(&model.UserRole{}).Where("user_id = ?", user.ID).Pluck("role_id", &roleIDs)
	if len(roleIDs) == 0 {
		return nil
	}
	var permIDs []uint
	db.Model(&model.RolePermission{}).Where("role_id IN ?", roleIDs).Pluck("permission_id", &permIDs)
	if len(permIDs) == 0 {
		return nil
	}
	var codes []string
	db.Model(&model.Permission{}).Where("id IN ?", permIDs).Pluck("code", &codes)
	return codes
}
