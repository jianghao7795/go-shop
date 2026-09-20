package database

import (
	"os"
	"strings"

	"gorm.io/driver/mysql"
	"gorm.io/gorm"
)

// DSN 返回连接目标数据库的 MySQL DSN，配置项均可通过环境变量覆盖。
func DSN() string {
	host := envOr("MYSQL_HOST", "192.168.59.161")
	port := envOr("MYSQL_PORT", "3306")
	user := envOr("MYSQL_USER", "root")
	password := envOr("MYSQL_PASSWORD", "123456")
	database := envOr("MYSQL_DATABASE", "shop")
	return user + ":" + password + "@tcp(" + host + ":" + port + ")/" + database + "?parseTime=true&charset=utf8mb4&loc=Local"
}

// AdminDSN 返回不指定数据库的 DSN，用于在目标库不存在时创建它。
func AdminDSN() string {
	database := envOr("MYSQL_DATABASE", "shop")
	return strings.Replace(DSN(), "/"+database+"?", "/?", 1)
}

// Open 连接数据库；若目标库不存在则先创建再连接。
func Open() (*gorm.DB, error) {
	db, err := gorm.Open(mysql.Open(DSN()), &gorm.Config{})
	if err == nil {
		return db, nil
	}
	admin, adminErr := gorm.Open(mysql.Open(AdminDSN()), &gorm.Config{})
	if adminErr != nil {
		return nil, err
	}
	database := envOr("MYSQL_DATABASE", "shop")
	if createErr := admin.Exec("CREATE DATABASE IF NOT EXISTS `" + database + "` CHARACTER SET utf8mb4 COLLATE utf8mb4_unicode_ci").Error; createErr != nil {
		return nil, createErr
	}
	adminSQL, _ := admin.DB()
	adminSQL.Close()
	return gorm.Open(mysql.Open(DSN()), &gorm.Config{})
}

func envOr(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}
