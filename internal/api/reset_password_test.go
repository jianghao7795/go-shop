package api

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/glebarez/sqlite"
	"golang.org/x/crypto/bcrypt"
	"gorm.io/gorm"

	"shop/internal/model"
)

func resetPwdCtx(t *testing.T, userID uint, password string) (*gin.Context, *httptest.ResponseRecorder) {
	t.Helper()
	gin.SetMode(gin.TestMode)
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	body := fmt.Sprintf(`{"password":%q}`, password)
	c.Request = httptest.NewRequest("PUT", "/api/admin/users/"+strconv.Itoa(int(userID))+"/password", strings.NewReader(body))
	c.Request.Header.Set("Content-Type", "application/json")
	c.Params = gin.Params{{Key: "id", Value: strconv.Itoa(int(userID))}}
	return c, w
}

func TestAdminResetPassword(t *testing.T) {
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatal(err)
	}
	db.AutoMigrate(&model.User{})
	u := model.User{Username: "alice", Password: "oldhash"}
	db.Create(&u)

	c, w := resetPwdCtx(t, u.ID, "newpass123")
	adminResetPassword(db)(c)
	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200 (body=%s)", w.Code, w.Body.String())
	}

	var got model.User
	db.First(&got, u.ID)
	if bcrypt.CompareHashAndPassword([]byte(got.Password), []byte("newpass123")) != nil {
		t.Fatal("新密码不匹配")
	}
}

func TestAdminResetPasswordTooShort(t *testing.T) {
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatal(err)
	}
	db.AutoMigrate(&model.User{})
	u := model.User{Username: "alice", Password: "x"}
	db.Create(&u)

	c, w := resetPwdCtx(t, u.ID, "123")
	adminResetPassword(db)(c)
	if w.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400", w.Code)
	}
}
