package api

import (
	"log"
	"os"
	"strconv"

	"github.com/gin-gonic/gin"

	"shop/internal/auth"
	"shop/internal/database"
	"shop/internal/model"
)

// Start 启动 Gin API 服务器，监听 :8080（可用 API_PORT 覆盖）。
func Start() {
	db, err := database.Open()
	databaseReady := err == nil
	if err != nil {
		log.Printf("mysql unavailable, serving catalog fallback: %v", err)
	} else if migrateErr := db.AutoMigrate(&model.Product{}, &model.User{}, &model.Address{}, &model.Order{}, &model.Notification{}, &model.Category{}, &model.Coupon{}, &model.Role{}, &model.UserRole{}, &model.Permission{}, &model.RolePermission{}); migrateErr != nil {
		log.Printf("mysql migration failed: %v", migrateErr)
		databaseReady = false
	} else {
		var count int64
		db.Model(&model.Product{}).Count(&count)
		if count == 0 {
			if seedErr := db.Create(&model.FallbackProducts).Error; seedErr != nil {
				log.Printf("product seed failed: %v", seedErr)
			} else {
				log.Printf("seeded %d products", len(model.FallbackProducts))
			}
		}
		var catCount int64
		db.Model(&model.Category{}).Count(&catCount)
		if catCount == 0 {
			if seedErr := db.Create(&model.Categories).Error; seedErr != nil {
				log.Printf("category seed failed: %v", seedErr)
			} else {
				log.Printf("seeded %d categories", len(model.Categories))
			}
		}
		var couponCount int64
		db.Model(&model.Coupon{}).Count(&couponCount)
		if couponCount == 0 {
			if seedErr := db.Create(&model.Coupons).Error; seedErr != nil {
				log.Printf("coupon seed failed: %v", seedErr)
			} else {
				log.Printf("seeded %d coupons", len(model.Coupons))
			}
		}
		// 补齐历史优惠券的有效期（新增 start_at/end_at 字段后，旧数据可能为空）
		couponStart, couponEnd := model.DefaultCouponPeriod()
		db.Model(&model.Coupon{}).Where("start_at IS NULL").Update("start_at", couponStart)
		db.Model(&model.Coupon{}).Where("end_at IS NULL").Update("end_at", couponEnd)
		// 种子权限点
		for _, p := range model.SeedPermissions {
			var existing model.Permission
			if db.Where("code = ?", p.Code).First(&existing).Error != nil {
				db.Create(&p)
			}
		}
		// 种子 admin 角色并关联全部权限（含旧 roles.permissions JSON 的迁移）
		var adminRole model.Role
		if db.Where("name = ?", model.RoleAdmin).First(&adminRole).Error != nil {
			adminRole = model.Role{Name: model.RoleAdmin, Description: "超级管理员"}
			db.Create(&adminRole)
		}
		var allPerms []model.Permission
		db.Find(&allPerms)
		// 确保 admin 角色关联全部权限（每权限幂等，新增权限点后重启也会自动补链）
		for _, p := range allPerms {
			var n int64
			db.Model(&model.RolePermission{}).Where("role_id = ? AND permission_id = ?", adminRole.ID, p.ID).Count(&n)
			if n == 0 {
				db.Create(&model.RolePermission{RoleID: adminRole.ID, PermissionID: p.ID})
			}
		}
		var customerRole model.Role
		if db.Where("name = ?", model.RoleCustomer).First(&customerRole).Error != nil {
			db.Create(&model.Role{Name: model.RoleCustomer, Description: "普通用户"})
		}
		// 迁移现有 role='admin' 的用户到 admin 角色
		var legacyAdmins []model.User
		db.Where("role = ?", model.RoleAdmin).Find(&legacyAdmins)
		for _, u := range legacyAdmins {
			var n int64
			db.Model(&model.UserRole{}).Where("user_id = ? AND role_id = ?", u.ID, adminRole.ID).Count(&n)
			if n == 0 {
				db.Create(&model.UserRole{UserID: u.ID, RoleID: adminRole.ID})
			}
		}
	}

	gin.SetMode(gin.ReleaseMode)
	router := gin.New()
	router.Use(gin.Recovery(), corsMiddleware())
	router.Static("/uploads", "./uploads")

	jwtMiddleware, err := auth.New(db)
	if err != nil {
		log.Fatalf("auth init failed: %v", err)
	}
	hub := newNotificationHub()
	router.GET("/api/notifications/stream", tokenFromQuery(), jwtMiddleware.MiddlewareFunc(), streamNotifications(hub))
	router.POST("/api/login", jwtMiddleware.LoginHandler)
	router.POST("/api/register", registerHandler(db))

	protected := router.Group("/api")
	protected.Use(jwtMiddleware.MiddlewareFunc())
	protected.GET("/me", meHandler(db))
	protected.PUT("/profile", updateProfile(db))
	protected.POST("/profile/avatar", uploadAvatar())
	protected.GET("/addresses", listAddresses(db))
	protected.POST("/addresses", createAddress(db))
	protected.PUT("/addresses/:id", updateAddress(db))
	protected.DELETE("/addresses/:id", deleteAddress(db))
	protected.PUT("/addresses/:id/default", setDefaultAddress(db))
	protected.GET("/orders", listOrders(db))
	protected.POST("/orders", createOrder(db))
	protected.GET("/orders/:id", getOrder(db))
	protected.POST("/orders/:id/pay", payOrder(db))
	protected.PUT("/orders/:id/status", updateOrderStatus(db, hub))
	protected.GET("/coupons", listCoupons(db, databaseReady))
	protected.GET("/notifications", listNotifications(db))
	protected.GET("/notifications/unread", unreadCount(db))
	protected.PUT("/notifications/read-all", markAllNotificationsRead(db))
	protected.PUT("/notifications/:id/read", markNotificationRead(db))

	admin := router.Group("/api/admin")
	admin.Use(jwtMiddleware.MiddlewareFunc())
	// 用户
	admin.GET("/users", requirePermission(db, "user:manage"), adminListUsers(db))
	admin.PUT("/users/:id/status", requirePermission(db, "user:manage"), adminUpdateUserStatus(db))
	// 角色
	admin.GET("/roles", requirePermission(db, "user:manage"), adminListRoles(db))
	admin.POST("/roles", requirePermission(db, "user:manage"), adminCreateRole(db))
	admin.PUT("/roles/:id", requirePermission(db, "user:manage"), adminUpdateRole(db))
	admin.DELETE("/roles/:id", requirePermission(db, "user:manage"), adminDeleteRole(db))
	// 权限点
	admin.GET("/permissions", requirePermission(db, "user:manage"), adminListPermissions(db))
	admin.POST("/permissions", requirePermission(db, "user:manage"), adminCreatePermission(db))
	admin.DELETE("/permissions/:id", requirePermission(db, "user:manage"), adminDeletePermission(db))
	// 用户角色
	admin.GET("/users/:id/roles", requirePermission(db, "user:manage"), adminGetUserRoles(db))
	admin.PUT("/users/:id/roles", requirePermission(db, "user:manage"), adminSetUserRoles(db))
	// 商品
	admin.GET("/products", requirePermission(db, "product:manage"), adminListProducts(db))
	admin.POST("/products", requirePermission(db, "product:manage"), adminCreateProduct(db))
	admin.PUT("/products/:id", requirePermission(db, "product:manage"), adminUpdateProduct(db))
	admin.DELETE("/products/:id", requirePermission(db, "product:manage"), adminDeleteProduct(db))
	admin.PUT("/products/:id/featured", requirePermission(db, "product:manage"), adminToggleFeatured(db))
	// 分类
	admin.GET("/categories", requirePermission(db, "category:manage"), adminListCategories(db))
	admin.POST("/categories", requirePermission(db, "category:manage"), adminCreateCategory(db))
	admin.PUT("/categories/:id", requirePermission(db, "category:manage"), adminUpdateCategory(db))
	admin.DELETE("/categories/:id", requirePermission(db, "category:manage"), adminDeleteCategory(db))
	// 订单
	admin.GET("/orders", requirePermission(db, "order:manage"), adminListOrders(db))
	admin.GET("/orders/:id", requirePermission(db, "order:manage"), adminGetOrder(db))
	admin.PUT("/orders/:id/status", requirePermission(db, "order:manage"), adminUpdateOrderStatus(db, hub))
	// 优惠券
	admin.GET("/coupons", requirePermission(db, "coupon:manage"), adminListCoupons(db))
	admin.POST("/coupons", requirePermission(db, "coupon:manage"), adminCreateCoupon(db))
	admin.PUT("/coupons/:id", requirePermission(db, "coupon:manage"), adminUpdateCoupon(db))
	admin.DELETE("/coupons/:id", requirePermission(db, "coupon:manage"), adminDeleteCoupon(db))
	// 通知
	admin.POST("/notifications", requirePermission(db, "notification:manage"), adminSendNotification(db, hub))

	router.GET("/api/health", healthHandler(databaseReady))
	router.GET("/api/categories", listCategories(db, databaseReady))
	router.GET("/api/products/:id", productDetailHandler(db, databaseReady))
	router.GET("/api/products", productsHandler(db, databaseReady))

	go startSimulators(db, hub)

	port := os.Getenv("API_PORT")
	if port == "" {
		port = "8080"
	}
	if _, err := strconv.Atoi(port); err != nil {
		port = "8080"
	}
	log.Printf("Gin API listening on :%s", port)
	if err := router.Run(":" + port); err != nil {
		log.Printf("Gin server stopped: %v", err)
	}
}

func corsMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		c.Header("Access-Control-Allow-Origin", "*")
		c.Header("Access-Control-Allow-Headers", "Content-Type, Authorization")
		c.Header("Access-Control-Allow-Methods", "GET, POST, PUT, DELETE, OPTIONS")
		if c.Request.Method == "OPTIONS" {
			c.AbortWithStatus(204)
			return
		}
		c.Next()
	}
}
