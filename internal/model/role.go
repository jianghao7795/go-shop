package model

import (
	"time"

	"gorm.io/gorm"
)

// 权限点（固定，按后台模块划分）。
const (
	PermProduct      = "product:manage"
	PermCategory     = "category:manage"
	PermOrder        = "order:manage"
	PermUser         = "user:manage"
	PermCoupon       = "coupon:manage"
	PermNotification = "notification:manage"
)

// 内置角色名。
const (
	RoleAdmin    = "admin"
	RoleCustomer = "customer"
)

// AllPermissions 是全部权限点。
var AllPermissions = []string{
	PermProduct, PermCategory, PermOrder, PermUser, PermCoupon, PermNotification,
}

// Role 是后台角色。
type Role struct {
	ID          uint           `json:"id" gorm:"primaryKey"`
	Name        string         `json:"name" gorm:"uniqueIndex;size:64"`
	Description string         `json:"description" gorm:"size:128"`
	Permissions []string       `json:"permissions" gorm:"serializer:json"`
	CreatedAt   time.Time      `json:"createdAt"`
	UpdatedAt   time.Time      `json:"updatedAt"`
	DeletedAt   gorm.DeletedAt `json:"-"`
}

func (Role) TableName() string { return "shop_roles" }

// UserRole 是用户与角色的多对多关联。
type UserRole struct {
	ID     uint `json:"id" gorm:"primaryKey"`
	UserID uint `json:"userId" gorm:"index"`
	RoleID uint `json:"roleId" gorm:"index"`
}

func (UserRole) TableName() string { return "shop_user_roles" }
