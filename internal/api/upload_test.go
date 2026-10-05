package api

import (
	"bytes"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
)

func uploadCtx(t *testing.T, filename string, content []byte) (*gin.Context, *httptest.ResponseRecorder, string) {
	t.Helper()
	gin.SetMode(gin.TestMode)
	body := &bytes.Buffer{}
	mw := multipart.NewWriter(body)
	part, err := mw.CreateFormFile("file", filename)
	if err != nil {
		t.Fatalf("formfile: %v", err)
	}
	if _, err := part.Write(content); err != nil {
		t.Fatalf("write: %v", err)
	}
	if err := mw.Close(); err != nil {
		t.Fatalf("close: %v", err)
	}
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest("POST", "/", body)
	c.Request.Header.Set("Content-Type", mw.FormDataContentType())
	return c, w, mw.FormDataContentType()
}

func TestSaveImageRejectsBadExt(t *testing.T) {
	c, w, _ := uploadCtx(t, "x.txt", []byte("not an image"))
	if _, ok := saveImage(c, t.TempDir(), "/uploads/products/"); ok {
		t.Fatal("非法扩展名应被拒绝")
	}
	if w.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400", w.Code)
	}
}

func TestSaveImageRejectsOversize(t *testing.T) {
	c, w, _ := uploadCtx(t, "big.png", bytes.Repeat([]byte("a"), 3<<20))
	if _, ok := saveImage(c, t.TempDir(), "/uploads/products/"); ok {
		t.Fatal("超大图片应被拒绝")
	}
	if w.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400", w.Code)
	}
}

func TestSaveImageSuccess(t *testing.T) {
	dir := t.TempDir()
	pngSig := []byte{0x89, 'P', 'N', 'G', '\r', '\n', 0x1a, '\n'}
	c, _, _ := uploadCtx(t, "a.png", pngSig)
	url, ok := saveImage(c, dir, "/uploads/products/")
	if !ok || !strings.HasPrefix(url, "/uploads/products/") {
		t.Fatalf("saveImage = %q, %v", url, ok)
	}
	name := strings.TrimPrefix(url, "/uploads/products/")
	if _, err := os.Stat(filepath.Join(dir, name)); err != nil {
		t.Fatalf("文件未落盘: %v", err)
	}
}

func TestSaveImageRejectsNonImage(t *testing.T) {
	c, w, _ := uploadCtx(t, "x.png", []byte("<html><script>alert(1)</script></html>"))
	if _, ok := saveImage(c, t.TempDir(), "/uploads/products/"); ok {
		t.Fatal("非图片内容应被拒绝")
	}
	if w.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400", w.Code)
	}
}

func TestNoSniffMiddleware(t *testing.T) {
	gin.SetMode(gin.TestMode)
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest("GET", "/uploads/x.png", nil)
	noSniffMiddleware()(c)
	if got := w.Header().Get("X-Content-Type-Options"); got != "nosniff" {
		t.Fatalf("header = %q, want nosniff", got)
	}
}
