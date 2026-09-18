package model

// User 是注册用户的持久化模型，密码以 bcrypt 哈希存储，不通过 JSON 返回。
type User struct {
	ID       uint   `json:"id" gorm:"primaryKey"`
	Username string `json:"username" gorm:"uniqueIndex;size:64"`
	Password string `json:"-" gorm:"size:255"`
}

// TableName 使用独立的 shop_users 表，避免与数据库中已存在的 users 表（其他系统，以 mobile 为标识）冲突。
func (User) TableName() string { return "shop_users" }
