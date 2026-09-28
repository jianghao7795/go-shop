package api

import "testing"

func TestHasPermission(t *testing.T) {
	perms := []string{"product:manage", "order:manage"}
	if !hasPermission(perms, "product:manage") {
		t.Fatal("should have product permission")
	}
	if hasPermission(perms, "user:manage") {
		t.Fatal("should not have user permission")
	}
}
