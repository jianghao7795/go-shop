package api

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"testing"

	jwt "github.com/appleboy/gin-jwt/v2"
	"github.com/gin-gonic/gin"

	"shop/internal/model"
)

func reviewCtx(t *testing.T, orderID uint, productID int) (*gin.Context, *httptest.ResponseRecorder) {
	t.Helper()
	gin.SetMode(gin.TestMode)
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	body := fmt.Sprintf(`{"reviews":[{"productId":%d,"rating":4,"content":"ok"}]}`, productID)
	c.Request = httptest.NewRequest("POST", "/api/orders/"+strconv.Itoa(int(orderID))+"/reviews", strings.NewReader(body))
	c.Request.Header.Set("Content-Type", "application/json")
	c.Params = gin.Params{{Key: "id", Value: strconv.Itoa(int(orderID))}}
	c.Set("JWT_PAYLOAD", jwt.MapClaims{"user": "alice"})
	return c, w
}

func TestSubmitOrderReviewsUpdatesRating(t *testing.T) {
	db := newTestDB(t)
	p := model.Product{Name: "A", Price: 1000, Stock: 5}
	db.Create(&p)
	order := model.Order{OrderNo: "O-RV", UserID: "alice", Status: model.OrderStatusCompleted,
		Items: []model.OrderItem{{ProductID: p.ID, Name: "A", Price: 1000, Quantity: 1}}}
	db.Create(&order)

	c, w := reviewCtx(t, order.ID, p.ID)
	submitOrderReviews(db)(c)
	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200 (body=%s)", w.Code, w.Body.String())
	}
	var got model.Product
	db.First(&got, p.ID)
	if got.Rating != 4.0 {
		t.Fatalf("rating = %v, want 4.0", got.Rating)
	}
}
