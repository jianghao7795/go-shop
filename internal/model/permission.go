package model

import (
	"time"

	"gorm.io/gorm"
)

type Permission struct {
	ID        uint           `json:"id" gorm:"primaryKey"`
	Code      string         `json:"code" gorm:"uniqueIndex;size:64"`
	Name      string         `json:"name" gorm:"size:64"`
	Path      string         `json:"path" gorm:"size:128"`
	CreatedAt time.Time      `json:"createdAt"`
	UpdatedAt time.Time      `json:"updatedAt"`
	DeletedAt gorm.DeletedAt `json:"-"`
}

func (Permission) TableName() string { return "shop_permissions" }

type RolePermission struct {
	ID           uint `json:"id" gorm:"primaryKey"`
	RoleID       uint `json:"roleId" gorm:"index"`
	PermissionID uint `json:"permissionId" gorm:"index"`
}

func (RolePermission) TableName() string { return "shop_role_permissions" }

// SeedPermissions 是内置的 6 个初始权限点。
var SeedPermissions = []Permission{
	{Code: "product:manage", Name: "商品管理", Path: "/admin/products"},
	{Code: "category:manage", Name: "分类管理", Path: "/admin/categories"},
	{Code: "order:manage", Name: "订单管理", Path: "/admin/orders"},
	{Code: "user:manage", Name: "用户管理", Path: "/admin/users"},
	{Code: "coupon:manage", Name: "优惠券管理", Path: "/admin/coupons"},
	{Code: "notification:manage", Name: "通知管理", Path: "/admin/notifications"},
}
