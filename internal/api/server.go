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
	} else if migrateErr := db.AutoMigrate(&model.Product{}, &model.User{}, &model.Address{}, &model.Order{}, &model.Notification{}); migrateErr != nil {
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
	}

	gin.SetMode(gin.ReleaseMode)
	router := gin.New()
	router.Use(gin.Recovery(), corsMiddleware())

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
	protected.GET("/me", meHandler)
	protected.GET("/addresses", listAddresses(db))
	protected.POST("/addresses", createAddress(db))
	protected.PUT("/addresses/:id", updateAddress(db))
	protected.DELETE("/addresses/:id", deleteAddress(db))
	protected.PUT("/addresses/:id/default", setDefaultAddress(db))
	protected.GET("/orders", listOrders(db))
	protected.POST("/orders", createOrder(db))
	protected.GET("/orders/:id", getOrder(db))
	protected.PUT("/orders/:id/status", updateOrderStatus(db, hub))
	protected.GET("/coupons", listCoupons())
	protected.GET("/notifications", listNotifications(db))
	protected.GET("/notifications/unread", unreadCount(db))
	protected.PUT("/notifications/read-all", markAllNotificationsRead(db))
	protected.PUT("/notifications/:id/read", markNotificationRead(db))

	router.GET("/api/health", healthHandler(databaseReady))
	router.GET("/api/products/:id", productDetailHandler(db, databaseReady))
	router.GET("/api/products", productsHandler(db, databaseReady))

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
