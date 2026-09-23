package api

import (
	"net/http"
	"regexp"
	"strings"

	"github.com/gin-gonic/gin"
	"golang.org/x/crypto/bcrypt"
	"gorm.io/gorm"

	"shop/internal/model"
)

var (
	mobileRe = regexp.MustCompile(`^1[3-9]\d{9}$`)
	emailRe  = regexp.MustCompile(`^[^@\s]+@[^@\s]+\.[^@\s]+$`)
)

// profileUpdate 是「编辑个人资料」请求体，四个字段均可空。
type profileUpdate struct {
	Nickname string `json:"nickname"`
	Avatar   string `json:"avatar"`
	Mobile   string `json:"mobile"`
	Email    string `json:"email"`
}

// validateProfile 校验资料更新请求，合法返回空字符串，否则返回错误提示。
func validateProfile(p profileUpdate) string {
	if len([]rune(strings.TrimSpace(p.Nickname))) > 64 {
		return "昵称不能超过 64 个字符"
	}
	if p.Avatar != "" {
		if len(p.Avatar) > 255 || (!strings.HasPrefix(p.Avatar, "http://") && !strings.HasPrefix(p.Avatar, "https://")) {
			return "头像需要是 http(s) 开头的链接"
		}
	}
	if p.Mobile != "" && !mobileRe.MatchString(p.Mobile) {
		return "手机号格式不正确"
	}
	if p.Email != "" && !emailRe.MatchString(p.Email) {
		return "邮箱格式不正确"
	}
	return ""
}

func registerHandler(db *gorm.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		var req struct {
			Username string `json:"username" binding:"required"`
			Password string `json:"password" binding:"required"`
		}
		if err := c.ShouldBindJSON(&req); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"message": "用户名和密码不能为空"})
			return
		}
		if db == nil {
			c.JSON(http.StatusServiceUnavailable, gin.H{"message": "数据库不可用，无法注册"})
			return
		}
		var count int64
		db.Model(&model.User{}).Where("username = ?", req.Username).Count(&count)
		if count > 0 {
			c.JSON(http.StatusConflict, gin.H{"message": "用户名已存在"})
			return
		}
		hash, err := bcrypt.GenerateFromPassword([]byte(req.Password), bcrypt.DefaultCost)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"message": "密码加密失败"})
			return
		}
		if err := db.Create(&model.User{Username: req.Username, Password: string(hash)}).Error; err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"message": "注册失败，请稍后重试"})
			return
		}
		c.JSON(http.StatusOK, gin.H{"message": "注册成功", "username": req.Username})
	}
}

// profileJSON 把用户模型转成对外返回的个人资料。
func profileJSON(user model.User) gin.H {
	return gin.H{
		"id":       user.ID,
		"username": user.Username,
		"nickname": user.Nickname,
		"avatar":   user.Avatar,
		"mobile":   user.Mobile,
		"email":    user.Email,
		"role":     user.Role,
	}
}

// meHandler 返回当前登录用户的完整资料。
func meHandler(db *gorm.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		if db == nil {
			c.JSON(http.StatusServiceUnavailable, gin.H{"message": "数据库不可用"})
			return
		}
		var user model.User
		if err := db.Where("username = ?", currentUser(c)).First(&user).Error; err != nil {
			c.JSON(http.StatusNotFound, gin.H{"message": "用户不存在"})
			return
		}
		c.JSON(http.StatusOK, profileJSON(user))
	}
}

// updateProfile 更新当前登录用户的资料（昵称/头像/手机号/邮箱）。
func updateProfile(db *gorm.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		var req profileUpdate
		if err := c.ShouldBindJSON(&req); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"message": "请求格式错误"})
			return
		}
		if msg := validateProfile(req); msg != "" {
			c.JSON(http.StatusBadRequest, gin.H{"message": msg})
			return
		}
		if db == nil {
			c.JSON(http.StatusServiceUnavailable, gin.H{"message": "数据库不可用"})
			return
		}
		username := currentUser(c)
		// 用 map 更新，保证空字符串能真正清空字段（struct 会跳过零值）。
		if err := db.Model(&model.User{}).Where("username = ?", username).Updates(map[string]any{
			"nickname": req.Nickname,
			"avatar":   req.Avatar,
			"mobile":   req.Mobile,
			"email":    req.Email,
		}).Error; err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"message": "保存失败，请稍后重试"})
			return
		}
		var user model.User
		if err := db.Where("username = ?", username).First(&user).Error; err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"message": "读取失败，请稍后重试"})
			return
		}
		c.JSON(http.StatusOK, profileJSON(user))
	}
}
