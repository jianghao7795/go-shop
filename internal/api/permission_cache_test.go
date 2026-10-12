package api

import (
	"context"
	"testing"
	"time"

	"github.com/alicebob/miniredis/v2"
	"github.com/glebarez/sqlite"
	"gorm.io/gorm"

	"shop/internal/model"
	"shop/internal/redisx"
)

func permTestDB(t *testing.T) *gorm.DB {
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatal(err)
	}
	db.AutoMigrate(&model.User{}, &model.Role{}, &model.Permission{}, &model.UserRole{}, &model.RolePermission{})
	return db
}

func TestEffectivePermissionsUsesCache(t *testing.T) {
	mr := miniredis.RunT(t)
	permCache = redisx.New(redisx.Config{Addr: mr.Addr()})
	t.Cleanup(func() { permCache = nil })

	db := permTestDB(t) // 空库，无任何数据

	cacheEffectivePermissions("alice", []string{"product:manage"})
	got := effectivePermissions(db, "alice")
	if !hasPermission(got, "product:manage") {
		t.Fatalf("expected cached permission product:manage, got %v", got)
	}
}

func TestCachedPermsStaleAfterInvalidate(t *testing.T) {
	mr := miniredis.RunT(t)
	permCache = redisx.New(redisx.Config{Addr: mr.Addr()})
	t.Cleanup(func() { permCache = nil })

	cacheEffectivePermissions("alice", []string{"product:manage"})
	if _, ok := cachedEffectivePermissions("alice"); !ok {
		t.Fatal("expected cache hit before invalidation")
	}
	invalidatePermCache()
	if _, ok := cachedEffectivePermissions("alice"); ok {
		t.Fatal("expected cache miss after invalidation")
	}
}

func TestEffectivePermissionsRedisDown(t *testing.T) {
	mr := miniredis.RunT(t)
	permCache = redisx.New(redisx.Config{Addr: mr.Addr()})
	t.Cleanup(func() { permCache = nil })

	db := permTestDB(t)
	user := model.User{Username: "alice", Password: "x"}
	db.Create(&user)
	role := model.Role{Name: "staff"}
	db.Create(&role)
	p := model.Permission{Code: "order:manage"}
	db.Create(&p)
	db.Create(&model.UserRole{UserID: user.ID, RoleID: role.ID})
	db.Create(&model.RolePermission{RoleID: role.ID, PermissionID: p.ID})

	mr.Close() // 模拟 Redis 中途宕机（Available 仍为 true）

	got := effectivePermissions(db, "alice")
	if !hasPermission(got, "order:manage") {
		t.Fatalf("expected DB fallback to return order:manage, got %v", got)
	}
}

func TestCachedPermsCorruptValue(t *testing.T) {
	mr := miniredis.RunT(t)
	permCache = redisx.New(redisx.Config{Addr: mr.Addr()})
	t.Cleanup(func() { permCache = nil })

	_ = permCache.Set(context.Background(), "perm:alice", "not-json", time.Minute)
	if _, ok := cachedEffectivePermissions("alice"); ok {
		t.Fatal("expected miss for corrupt cache value")
	}
}

func TestEmptyCodesCached(t *testing.T) {
	mr := miniredis.RunT(t)
	permCache = redisx.New(redisx.Config{Addr: mr.Addr()})
	t.Cleanup(func() { permCache = nil })

	cacheEffectivePermissions("nobody", nil)
	codes, ok := cachedEffectivePermissions("nobody")
	if !ok {
		t.Fatal("expected cache hit for empty codes")
	}
	if len(codes) != 0 {
		t.Fatalf("expected empty codes, got %v", codes)
	}
}
