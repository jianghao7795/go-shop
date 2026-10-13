package api

import (
	"testing"

	"github.com/alicebob/miniredis/v2"

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
