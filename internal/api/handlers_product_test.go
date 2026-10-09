package api

import "testing"

func TestProductSortOrder(t *testing.T) {
	cases := map[string]string{
		"":            "id DESC",
		"price_asc":   "price ASC",
		"price_desc":  "price DESC",
		"sales_desc":  "sales DESC",
		"rating_desc": "rating DESC",
		"unknown":     "id DESC",
	}
	for in, want := range cases {
		if got := productSortOrder(in); got != want {
			t.Fatalf("productSortOrder(%q) = %q, want %q", in, got, want)
		}
	}
}
