package model

import (
	"time"

	"gorm.io/gorm"
)

// 用户角色。
const (
	RoleCustomer = "customer"
	RoleAdmin    = "admin"
)

// User 是注册用户的持久化模型，密码以 bcrypt 哈希存储，不通过 JSON 返回。
type User struct {
	ID          uint           `json:"id" gorm:"primaryKey"`
	Username    string         `json:"username" gorm:"uniqueIndex;size:64;not null"`
	Password    string         `json:"-" gorm:"size:255;not null"`
	Mobile      string         `json:"mobile,omitempty" gorm:"index;size:20"`
	Email       string         `json:"email,omitempty" gorm:"size:128"`
	Nickname    string         `json:"nickname,omitempty" gorm:"size:64"`
	Avatar      string         `json:"avatar,omitempty" gorm:"size:255"`
	Role        string         `json:"role" gorm:"size:20;default:customer"`
	Status      int            `json:"status" gorm:"default:1"`
	LastLoginAt *time.Time     `json:"lastLoginAt,omitempty"`
	LastLoginIP string         `json:"lastLoginIp,omitempty" gorm:"size:45"`
	CreatedAt   time.Time      `json:"createdAt"`
	UpdatedAt   time.Time      `json:"updatedAt"`
	DeletedAt   gorm.DeletedAt `json:"-"`
}

// TableName 使用独立的 shop_users 表，避免与数据库中已存在的 users 表（其他系统，以 mobile 为标识）冲突。
func (User) TableName() string { return "shop_users" }
