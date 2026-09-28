package model

import (
	"time"

	"gorm.io/gorm"
)

// 内置角色名。
const (
	RoleAdmin    = "admin"
	RoleCustomer = "customer"
)

// Role 是后台角色。
type Role struct {
	ID          uint           `json:"id" gorm:"primaryKey"`
	Name        string         `json:"name" gorm:"uniqueIndex;size:64"`
	Description string         `json:"description" gorm:"size:128"`
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
