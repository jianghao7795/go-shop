package model

import (
	"encoding/json"
	"testing"
)

func TestPriceMarshalJSON(t *testing.T) {
	got, err := json.Marshal(Price(19900))
	if err != nil {
		t.Fatal(err)
	}
	if string(got) != "199" {
		t.Fatalf("marshal = %s, want 199", got)
	}
	got, _ = json.Marshal(Price(19990))
	if string(got) != "199.9" {
		t.Fatalf("marshal = %s, want 199.9", got)
	}
}

func TestPriceUnmarshalJSON(t *testing.T) {
	var p Price
	if err := json.Unmarshal([]byte("199"), &p); err != nil {
		t.Fatal(err)
	}
	if p != Price(19900) {
		t.Fatalf("p = %d, want 19900", p)
	}
	var q Price
	if err := json.Unmarshal([]byte("199.9"), &q); err != nil {
		t.Fatal(err)
	}
	if q != Price(19990) {
		t.Fatalf("q = %d, want 19990", q)
	}
}

func TestPriceRoundTrip(t *testing.T) {
	type wrapper struct {
		Price Price `json:"price"`
	}
	w := wrapper{Price: Price(19990)}
	b, err := json.Marshal(w)
	if err != nil {
		t.Fatal(err)
	}
	var back wrapper
	if err := json.Unmarshal(b, &back); err != nil {
		t.Fatal(err)
	}
	if back.Price != Price(19990) {
		t.Fatalf("roundtrip = %d, want 19990", back.Price)
	}
}

func TestPriceYuanString(t *testing.T) {
	if s := Price(19900).YuanString(); s != "199.00" {
		t.Fatalf("s = %q, want 199.00", s)
	}
	if s := Price(19990).YuanString(); s != "199.90" {
		t.Fatalf("s = %q, want 199.90", s)
	}
	if s := Price(30).YuanString(); s != "0.30" {
		t.Fatalf("s = %q, want 0.30", s)
	}
}
