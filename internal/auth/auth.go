package auth

import (
	"net/http"
	"os"
	"time"

	jwt "github.com/appleboy/gin-jwt/v2"
	"github.com/gin-gonic/gin"
	"golang.org/x/crypto/bcrypt"
	"gorm.io/gorm"

	"shop/internal/model"
)

type loginRequest struct {
	Username string `json:"username" binding:"required"`
	Password string `json:"password" binding:"required"`
}

// authIdentity 是认证通过后写入 JWT 的身份信息。
type authIdentity struct {
	Username string
	Role     string
}

// findUserByLogin 按「用户名或手机号」查找注册用户。
func findUserByLogin(db *gorm.DB, identifier string) (*model.User, error) {
	var user model.User
	if err := db.Where("username = ?", identifier).First(&user).Error; err == nil {
		return &user, nil
	}
	if err := db.Where("mobile = ?", identifier).First(&user).Error; err == nil {
		return &user, nil
	}
	return nil, gorm.ErrRecordNotFound
}

// New 构建 JWT 认证中间件：仅校验数据库中的注册用户，数据库不可用时登录一律失败。
func New(db *gorm.DB) (*jwt.GinJWTMiddleware, error) {
	secret := os.Getenv("JWT_SECRET")
	if secret == "" {
		secret = "change-this-shop-jwt-secret"
	}
	return jwt.New(&jwt.GinJWTMiddleware{
		Realm:       "shop",
		Key:         []byte(secret),
		Timeout:     24 * time.Hour,
		MaxRefresh:  7 * 24 * time.Hour,
		IdentityKey: "user",
		PayloadFunc: func(data any) jwt.MapClaims {
			if ident, ok := data.(authIdentity); ok {
				role := ident.Role
				if role == "" {
					role = model.RoleCustomer
				}
				return jwt.MapClaims{"user": ident.Username, "role": role}
			}
			return jwt.MapClaims{}
		},
		Authenticator: func(c *gin.Context) (any, error) {
			var req loginRequest
			if err := c.ShouldBindJSON(&req); err != nil {
				return nil, jwt.ErrMissingLoginValues
			}
			if db == nil {
				return nil, jwt.ErrFailedAuthentication
			}
			user, err := findUserByLogin(db, req.Username)
			if err != nil {
				return nil, jwt.ErrFailedAuthentication
			}
			if bcrypt.CompareHashAndPassword([]byte(user.Password), []byte(req.Password)) != nil {
				return nil, jwt.ErrFailedAuthentication
			}
			role := user.Role
			if role == "" {
				role = model.RoleCustomer
			}
			return authIdentity{Username: user.Username, Role: role}, nil
		},
		Unauthorized: func(c *gin.Context, code int, message string) {
			c.JSON(code, gin.H{"code": code, "message": message})
		},
		LoginResponse: func(c *gin.Context, code int, token string, expire time.Time) {
			c.JSON(http.StatusOK, gin.H{"token": token, "expire": expire, "user": "customer"})
		},
	})
}
