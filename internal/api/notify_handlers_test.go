package api

import (
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
)

func TestTokenFromQuerySetsAuthorization(t *testing.T) {
	gin.SetMode(gin.TestMode)
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest("GET", "/api/notifications/stream?token=abc.def.ghi", nil)

	tokenFromQuery()(c)

	if got := c.Request.Header.Get("Authorization"); got != "Bearer abc.def.ghi" {
		t.Fatalf("Authorization = %q", got)
	}
}

func TestTokenFromQueryKeepsExistingAuthorization(t *testing.T) {
	gin.SetMode(gin.TestMode)
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest("GET", "/api/notifications/stream?token=xyz", nil)
	c.Request.Header.Set("Authorization", "Bearer existing")

	tokenFromQuery()(c)

	if got := c.Request.Header.Get("Authorization"); got != "Bearer existing" {
		t.Fatalf("Authorization = %q", got)
	}
}
