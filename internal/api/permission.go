package api

import (
	"gorm.io/gorm"

	"shop/internal/model"
)

// unionPermissions 合并多个角色的权限点（去重）。
func unionPermissions(roles []model.Role) []string {
	seen := map[string]bool{}
	var perms []string
	for _, r := range roles {
		for _, p := range r.Permissions {
			if !seen[p] {
				seen[p] = true
				perms = append(perms, p)
			}
		}
	}
	return perms
}

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
	var roles []model.Role
	db.Where("id IN ?", roleIDs).Find(&roles)
	return unionPermissions(roles)
}
