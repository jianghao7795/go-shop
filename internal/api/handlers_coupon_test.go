package api

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strconv"
	"testing"

	jwt "github.com/appleboy/gin-jwt/v2"
	"github.com/gin-gonic/gin"

	"shop/internal/model"
)

func TestListCouponsFallback(t *testing.T) {
	gin.SetMode(gin.TestMode)
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest("GET", "/api/coupons", nil)

	listCoupons(nil, false)(c)

	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", w.Code)
	}
	var got []model.Coupon
	if err := json.Unmarshal(w.Body.Bytes(), &got); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if len(got) != 4 {
		t.Fatalf("len = %d, want 4", len(got))
	}
}

func TestCouponTableName(t *testing.T) {
	if got := (model.Coupon{}).TableName(); got != "shop_coupons" {
		t.Fatalf("TableName = %q, want shop_coupons", got)
	}
}

func claimCtx(t *testing.T, couponID uint) (*gin.Context, *httptest.ResponseRecorder) {
	t.Helper()
	gin.SetMode(gin.TestMode)
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest("POST", "/api/coupons/"+strconv.Itoa(int(couponID))+"/claim", nil)
	c.Params = gin.Params{{Key: "id", Value: strconv.Itoa(int(couponID))}}
	c.Set("JWT_PAYLOAD", jwt.MapClaims{"user": "alice"})
	return c, w
}

func TestClaimCouponOnce(t *testing.T) {
	db := newTestDB(t)
	coupon := model.Coupon{Title: "新人券", Amount: 1000, MinAmount: 0}
	db.Create(&coupon)

	c, w := claimCtx(t, coupon.ID)
	claimCoupon(db)(c)
	if w.Code != http.StatusOK {
		t.Fatalf("first claim status = %d, want 200", w.Code)
	}

	c2, w2 := claimCtx(t, coupon.ID)
	claimCoupon(db)(c2)
	if w2.Code != http.StatusConflict {
		t.Fatalf("second claim status = %d, want 409", w2.Code)
	}
}
