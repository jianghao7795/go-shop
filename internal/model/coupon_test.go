package model

import (
	"testing"
	"time"
)

func TestDefaultCouponPeriod(t *testing.T) {
	start, end := DefaultCouponPeriod()
	if !start.Before(time.Now()) {
		t.Fatalf("start should be in the past: %v", start)
	}
	if !end.After(time.Now()) {
		t.Fatalf("end should be in the future: %v", end)
	}
	if !start.Before(end) {
		t.Fatalf("start should be before end: %v -> %v", start, end)
	}
}
