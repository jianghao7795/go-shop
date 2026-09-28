package api

import (
	"reflect"
	"testing"

	"shop/internal/model"
)

func TestUnionPermissions(t *testing.T) {
	roles := []model.Role{
		{Permissions: []string{model.PermProduct, model.PermOrder}},
		{Permissions: []string{model.PermOrder, model.PermCoupon}},
	}
	got := unionPermissions(roles)
	want := []string{model.PermProduct, model.PermOrder, model.PermCoupon}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("unionPermissions = %v, want %v", got, want)
	}
}

func TestHasPermission(t *testing.T) {
	perms := []string{model.PermProduct, model.PermOrder}
	if !hasPermission(perms, model.PermProduct) {
		t.Fatal("should have product permission")
	}
	if hasPermission(perms, model.PermUser) {
		t.Fatal("should not have user permission")
	}
}
