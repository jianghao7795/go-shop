package api

import (
	"testing"

	"github.com/glebarez/sqlite"
	"gorm.io/gorm"

	"shop/internal/model"
)

func TestHasPermission(t *testing.T) {
	perms := []string{"product:manage", "order:manage"}
	if !hasPermission(perms, "product:manage") {
		t.Fatal("should have product permission")
	}
	if hasPermission(perms, "user:manage") {
		t.Fatal("should not have user permission")
	}
}

func TestEffectivePermissions(t *testing.T) {
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatal(err)
	}
	db.AutoMigrate(&model.User{}, &model.Role{}, &model.Permission{}, &model.UserRole{}, &model.RolePermission{})
	user := model.User{Username: "alice", Password: "x"}
	db.Create(&user)
	role := model.Role{Name: "staff"}
	db.Create(&role)
	p1 := model.Permission{Code: "product:manage"}
	p2 := model.Permission{Code: "order:manage"}
	db.Create(&p1)
	db.Create(&p2)
	db.Create(&model.UserRole{UserID: user.ID, RoleID: role.ID})
	db.Create(&model.RolePermission{RoleID: role.ID, PermissionID: p1.ID})
	db.Create(&model.RolePermission{RoleID: role.ID, PermissionID: p2.ID})

	got := effectivePermissions(db, "alice")
	if len(got) != 2 || !hasPermission(got, "product:manage") || !hasPermission(got, "order:manage") {
		t.Fatalf("got = %v, want [product:manage order:manage]", got)
	}
	if got := effectivePermissions(db, "nobody"); len(got) != 0 {
		t.Fatalf("nobody got = %v, want empty", got)
	}
}
