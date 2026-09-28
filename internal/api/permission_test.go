package api

import (
	"testing"

	"shop/internal/model"
)

func TestHasPermission(t *testing.T) {
	perms := []string{model.PermProduct, model.PermOrder}
	if !hasPermission(perms, model.PermProduct) {
		t.Fatal("should have product permission")
	}
	if hasPermission(perms, model.PermUser) {
		t.Fatal("should not have user permission")
	}
}
