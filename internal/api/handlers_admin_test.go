package api

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/glebarez/sqlite"
	"gorm.io/gorm"

	"shop/internal/model"
)

func TestValidateProduct(t *testing.T) {
	tests := []struct {
		name string
		p    productPayload
		ok   bool
	}{
		{"合法", productPayload{Name: "商品A", Price: 10, Category: "Home"}, true},
		{"缺名称", productPayload{Price: 10, Category: "Home"}, false},
		{"价格为负", productPayload{Name: "A", Price: -1, Category: "Home"}, false},
		{"缺分类", productPayload{Name: "A", Price: 10}, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := validateProduct(tt.p); (got == "") != tt.ok {
				t.Fatalf("validateProduct(%+v)=%q, ok=%v", tt.p, got, tt.ok)
			}
		})
	}
}

func TestValidateCategory(t *testing.T) {
	tests := []struct {
		name string
		p    categoryPayload
		ok   bool
	}{
		{"合法", categoryPayload{Key: "Home", Name: "家居生活"}, true},
		{"缺 key", categoryPayload{Name: "家居生活"}, false},
		{"缺名称", categoryPayload{Key: "Home"}, false},
		{"空白 key", categoryPayload{Key: "  ", Name: "家居生活"}, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := validateCategory(tt.p); (got == "") != tt.ok {
				t.Fatalf("validateCategory(%+v)=%q, ok=%v", tt.p, got, tt.ok)
			}
		})
	}
}

func TestValidateCoupon(t *testing.T) {
	tests := []struct {
		name string
		p    couponPayload
		ok   bool
	}{
		{"合法", couponPayload{Title: "新人专享券", Amount: 10, MinAmount: 99}, true},
		{"缺标题", couponPayload{Amount: 10, MinAmount: 99}, false},
		{"金额为 0", couponPayload{Title: "A", Amount: 0, MinAmount: 99}, false},
		{"金额为负", couponPayload{Title: "A", Amount: -1, MinAmount: 99}, false},
		{"门槛为负", couponPayload{Title: "A", Amount: 10, MinAmount: -1}, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := validateCoupon(tt.p); (got == "") != tt.ok {
				t.Fatalf("validateCoupon(%+v)=%q, ok=%v", tt.p, got, tt.ok)
			}
		})
	}
}

func TestValidateRole(t *testing.T) {
	tests := []struct {
		name string
		p    rolePayload
		ok   bool
	}{
		{"合法", rolePayload{Name: "运营", PermissionIDs: []uint{1, 2}}, true},
		{"合法-空权限", rolePayload{Name: "只读"}, true},
		{"缺名称", rolePayload{PermissionIDs: []uint{1}}, false},
		{"空白名称", rolePayload{Name: "  "}, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := validateRole(tt.p); (got == "") != tt.ok {
				t.Fatalf("validateRole(%+v)=%q, ok=%v", tt.p, got, tt.ok)
			}
		})
	}
}

func newProductTestDB(t *testing.T) *gorm.DB {
	t.Helper()
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	if err := db.AutoMigrate(&model.Product{}); err != nil {
		t.Fatalf("migrate: %v", err)
	}
	return db
}

func TestSanitizeProductDetail(t *testing.T) {
	in := `<p onclick="alert(1)">你好<img src="/uploads/products/a.png" onerror="alert(2)"></p><script>alert(3)</script><table><tr><td>参数</td></tr></table>`
	out := sanitizeProductDetail(in)
	if strings.Contains(out, "alert") || strings.Contains(out, "<script") {
		t.Fatalf("未剥离危险内容: %q", out)
	}
	if !strings.Contains(out, "a.png") {
		t.Fatalf("img src 应保留: %q", out)
	}
	if !strings.Contains(out, "<table") {
		t.Fatalf("table 应保留: %q", out)
	}
}

func adminCreateProductBody(t *testing.T, body string) (*gin.Context, *httptest.ResponseRecorder) {
	t.Helper()
	gin.SetMode(gin.TestMode)
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest("POST", "/api/admin/products", strings.NewReader(body))
	c.Request.Header.Set("Content-Type", "application/json")
	return c, w
}

func TestAdminCreateProductWithDetail(t *testing.T) {
	db := newProductTestDB(t)
	c, w := adminCreateProductBody(t, `{"name":"A","price":10,"category":"Home","detail":"<p onclick=alert(1)>详情</p>"}`)
	adminCreateProduct(db)(c)
	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", w.Code)
	}
	var p model.Product
	if err := db.First(&p).Error; err != nil {
		t.Fatalf("query: %v", err)
	}
	if !strings.Contains(p.Detail, "详情") {
		t.Fatalf("detail 未落库: %q", p.Detail)
	}
	if strings.Contains(p.Detail, "onclick") {
		t.Fatalf("detail 未被清洗: %q", p.Detail)
	}
}
