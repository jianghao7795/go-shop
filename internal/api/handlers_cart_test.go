package api

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	jwt "github.com/appleboy/gin-jwt/v2"
	"github.com/gin-gonic/gin"

	"shop/internal/model"
)

func cartCtx(t *testing.T, method, path, body string) (*gin.Context, *httptest.ResponseRecorder) {
	t.Helper()
	gin.SetMode(gin.TestMode)
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	var req *http.Request
	if body != "" {
		req = httptest.NewRequest(method, path, strings.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
	} else {
		req = httptest.NewRequest(method, path, nil)
	}
	c.Request = req
	c.Set("JWT_PAYLOAD", jwt.MapClaims{"user": "alice"})
	return c, w
}

func TestCartSync(t *testing.T) {
	db := newTestDB(t)
	p := model.Product{Name: "A", Price: 1000}
	db.Create(&p)

	c, w := cartCtx(t, "PUT", "/api/cart", fmt.Sprintf(`[{"productId":%d,"quantity":2}]`, p.ID))
	putCart(db)(c)
	if w.Code != http.StatusOK {
		t.Fatalf("put status = %d, want 200", w.Code)
	}

	c2, w2 := cartCtx(t, "GET", "/api/cart", "")
	getCart(db)(c2)
	if w2.Code != http.StatusOK {
		t.Fatalf("get status = %d, want 200", w2.Code)
	}
	var views []cartItemView
	if err := json.Unmarshal(w2.Body.Bytes(), &views); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if len(views) != 1 || views[0].ProductID != p.ID || views[0].Quantity != 2 || views[0].Name != "A" {
		t.Fatalf("views = %+v", views)
	}
}

func TestCartSyncOverwrite(t *testing.T) {
	db := newTestDB(t)
	p1 := model.Product{Name: "A", Price: 1000}
	p2 := model.Product{Name: "B", Price: 2000}
	db.Create(&p1)
	db.Create(&p2)

	// 第一次放 p1
	c, _ := cartCtx(t, "PUT", "/api/cart", fmt.Sprintf(`[{"productId":%d,"quantity":2}]`, p1.ID))
	putCart(db)(c)
	// 第二次放 p2（全量覆盖）
	c2, _ := cartCtx(t, "PUT", "/api/cart", fmt.Sprintf(`[{"productId":%d,"quantity":1}]`, p2.ID))
	putCart(db)(c2)

	c3, w3 := cartCtx(t, "GET", "/api/cart", "")
	getCart(db)(c3)
	var views []cartItemView
	json.Unmarshal(w3.Body.Bytes(), &views)
	if len(views) != 1 || views[0].ProductID != p2.ID {
		t.Fatalf("views = %+v, want only p2", views)
	}
}
