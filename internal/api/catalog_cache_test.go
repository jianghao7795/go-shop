package api

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/alicebob/miniredis/v2"
	"github.com/gin-gonic/gin"

	"shop/internal/model"
	"shop/internal/redisx"
)

func TestCachedCategoriesRoundTrip(t *testing.T) {
	mr := miniredis.RunT(t)
	catalogCache = redisx.New(redisx.Config{Addr: mr.Addr()})
	t.Cleanup(func() { catalogCache = nil })

	cats := []model.Category{{Key: "food", Name: "食品"}, {Key: "digital", Name: "数码"}}
	cacheCategories(cats)
	got, ok := cachedCategories()
	if !ok || len(got) != 2 || got[0].Key != "food" {
		t.Fatalf("round trip: got %v, ok=%v", got, ok)
	}
}

func TestInvalidateCategoryCache(t *testing.T) {
	mr := miniredis.RunT(t)
	catalogCache = redisx.New(redisx.Config{Addr: mr.Addr()})
	t.Cleanup(func() { catalogCache = nil })

	cacheCategories([]model.Category{{Key: "food", Name: "食品"}})
	invalidateCategoryCache()
	if _, ok := cachedCategories(); ok {
		t.Fatal("expected miss after invalidate")
	}
}

func TestCachedProductRoundTrip(t *testing.T) {
	mr := miniredis.RunT(t)
	catalogCache = redisx.New(redisx.Config{Addr: mr.Addr()})
	t.Cleanup(func() { catalogCache = nil })

	p := model.Product{ID: 7, Name: "耳机", OnShelf: true}
	cacheProduct(p)
	got, ok := cachedProduct(7)
	if !ok || got.Name != "耳机" {
		t.Fatalf("round trip: got %+v, ok=%v", got, ok)
	}
}

func TestInvalidateProductCache(t *testing.T) {
	mr := miniredis.RunT(t)
	catalogCache = redisx.New(redisx.Config{Addr: mr.Addr()})
	t.Cleanup(func() { catalogCache = nil })

	cacheProduct(model.Product{ID: 7, Name: "耳机"})
	invalidateProductCache(7)
	if _, ok := cachedProduct(7); ok {
		t.Fatal("expected miss after invalidate")
	}
}

func TestCachedProductsRoundTrip(t *testing.T) {
	mr := miniredis.RunT(t)
	catalogCache = redisx.New(redisx.Config{Addr: mr.Addr()})
	t.Cleanup(func() { catalogCache = nil })

	items := []model.Product{{ID: 1, Name: "A"}, {ID: 2, Name: "B"}}
	cacheProducts(true, "", "sales_desc", items, 0)
	got, ok := cachedProducts(true, "", "sales_desc")
	if !ok || len(got) != 2 || got[0].Name != "A" {
		t.Fatalf("round trip: got %v, ok=%v", got, ok)
	}
}

func TestCachedProductsStaleEpoch(t *testing.T) {
	mr := miniredis.RunT(t)
	catalogCache = redisx.New(redisx.Config{Addr: mr.Addr()})
	t.Cleanup(func() { catalogCache = nil })

	cacheProducts(false, "", "id", []model.Product{{ID: 1, Name: "A"}}, 0)
	invalidateProductCache(1) // INCR product:epoch
	if _, ok := cachedProducts(false, "", "id"); ok {
		t.Fatal("expected miss after epoch bump")
	}
}

func TestMarkPaidInvalidatesProductCache(t *testing.T) {
	mr := miniredis.RunT(t)
	catalogCache = redisx.New(redisx.Config{Addr: mr.Addr()})
	t.Cleanup(func() { catalogCache = nil })

	db := newTestDB(t)
	p := model.Product{Name: "A", Price: 1000, Stock: 5}
	db.Create(&p)
	cacheProduct(p)
	order := model.Order{OrderNo: "O-CACHE", UserID: "u1", Status: model.OrderStatusPending, Amount: 1000,
		Items: []model.OrderItem{{ProductID: p.ID, Name: "A", Price: 1000, Quantity: 2}}}
	db.Create(&order)

	if err := markPaid(db, order.ID, "trade-cache"); err != nil {
		t.Fatalf("markPaid: %v", err)
	}
	if _, ok := cachedProduct(p.ID); ok {
		t.Fatal("expected product cache invalidated after markPaid")
	}
}

func TestProductsHandlerDoesNotCacheFallback(t *testing.T) {
	mr := miniredis.RunT(t)
	catalogCache = redisx.New(redisx.Config{Addr: mr.Addr()})
	t.Cleanup(func() { catalogCache = nil })

	db := newTestDB(t)
	gin.SetMode(gin.TestMode)
	router := gin.New()
	router.GET("/api/products", productsHandler(db, true))

	w := httptest.NewRecorder()
	router.ServeHTTP(w, httptest.NewRequest("GET", "/api/products", nil))
	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", w.Code)
	}
	if mr.Exists(productListKey(false, "", "")) {
		t.Fatal("fallback products must not be cached")
	}
}
