# 商品图文详情（富文本）实现计划

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** 给商品增加富文本「图文详情」：管理端用 wangEditor 编写图文混排内容，后端 bluemonday 清洗后存库，详情页 v-html 安全渲染。

**Architecture:** `Product` 加 `Detail` 文本列存 HTML；后端新增 `sanitizeProductDetail`（bluemonday 白名单）在 create/update 时清洗；图片上传复用头像上传逻辑抽成 `saveImage` 助手，新增 `uploads/products/` 目录与 `admin.POST /products/upload` 路由；前端新增 `RichEditor.vue`（wangEditor 核心 API 手动挂载）接入管理端表单，详情页用 `v-html` 渲染。

**Tech Stack:** Go + Gin + GORM + bluemonday；Vue 3 + Vant + wangEditor v5（`@wangeditor/editor`）。

**Spec:** `docs/superpowers/specs/2026-10-05-product-rich-detail-design.md`

## Global Constraints

- 图片上传仅允许 `.jpg/.jpeg/.png/.gif/.webp`，单文件 ≤ 2MB（与头像上传一致）。
- 详情 HTML 只能通过 `sanitizeProductDetail` 白名单后存库；前端 `v-html` 只渲染已清洗内容。
- 图片 URL 以相对路径 `/uploads/products/<file>` 返回；前端用 `avatarSrc` 解析成完整 URL。
- 商品图片上传路由需 `product:manage` 权限。
- 老商品（无 `detail`）详情页必须不报错、隐藏详情区块。

## Review Focus

1. 恶意 HTML（`<script>`、`onclick`、`onerror`）必须被清洗剥掉 → `TestSanitizeProductDetail`。
2. 非法扩展名（`.txt`/`.exe`）上传被拒 400 → `TestSaveImageRejectsBadExt`。
3. 超大图片（>2MB）上传被拒 400 → `TestSaveImageRejectsOversize`。
4. 相对路径图片 `/uploads/products/x.png` 清洗后保留且前端可显示 → `TestSanitizeProductDetail` + `avatarSrc` 复用（构建校验）。
5. 无 `detail` 的商品详情页不渲染空区块 → `v-if="product.detail"`（构建校验）。

---

### Task 1: 后端 Product.Detail 字段 + 清洗 + create/update 落库

**Files:**
- Modify: `internal/model/product.go`
- Modify: `internal/api/handlers_admin.go`
- Modify: `internal/api/payment_test.go`（`newTestDB` 增加 Product 迁移）
- Modify: `internal/api/handlers_admin_test.go`
- Modify: `go.mod` / `go.sum`（`go get` 自动）

**Interfaces:**
- Produces: `sanitizeProductDetail(html string) string`（Task 3/4 依赖后端行为，无跨任务函数依赖）
- Produces: `model.Product.Detail string`（JSON `detail`，Task 4 前端读取）

- [ ] **Step 1: 添加 bluemonday 依赖**

Run: `go get github.com/microcosm-cc/bluemonday`

- [ ] **Step 2: 写失败测试**

在 `internal/api/handlers_admin_test.go` 末尾追加（并更新文件头 import）：

```go
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
```

同时给 `internal/api/payment_test.go` 的 `newTestDB` 增加 `&model.Product{}`（无需测试，属测试基础设施）：

```go
if err := db.AutoMigrate(&model.Order{}, &model.Notification{}, &model.Product{}); err != nil {
```

- [ ] **Step 3: 运行测试确认失败**

Run: `go test ./internal/api/ -run 'TestSanitizeProductDetail|TestAdminCreateProductWithDetail'`
Expected: 编译失败（`sanitizeProductDetail` 未定义 / `model.Product` 无 `Detail` 字段）。

- [ ] **Step 4: 实现 model 字段**

`internal/model/product.go` 的 `Product` 结构体，`Description` 行后新增：

```go
	Detail        string         `json:"detail" gorm:"type:text"`
```

- [ ] **Step 5: 实现 payload + 清洗 + create/update**

`internal/api/handlers_admin.go`：

import 区新增 `"github.com/microcosm-cc/bluemonday"`。

`productPayload` 结构体新增：

```go
	Detail        string  `json:"detail"`
```

新增函数（放 `validateProduct` 附近）：

```go
// sanitizeProductDetail 用白名单清洗商品详情 HTML，防 XSS。
func sanitizeProductDetail(html string) string {
	p := bluemonday.UGCPolicy()
	p.AllowRelativeURLs(true)
	p.AllowElements("table", "thead", "tbody", "tr", "th", "td", "hr")
	p.AllowAttrs("colspan", "rowspan").OnElements("th", "td")
	return p.Sanitize(html)
}
```

`adminCreateProduct` 的 `item := model.Product{...}` 加 `Detail: sanitizeProductDetail(p.Detail),`：

```go
	item := model.Product{Name: p.Name, Description: p.Description, Detail: sanitizeProductDetail(p.Detail), Price: p.Price, OriginalPrice: p.OriginalPrice, Emoji: p.Emoji, Color: p.Color, Category: p.Category}
```

`adminUpdateProduct` 的 `updates` map 加 `"detail": sanitizeProductDetail(p.Detail),`：

```go
	updates := map[string]any{"name": p.Name, "description": p.Description, "detail": sanitizeProductDetail(p.Detail), "price": p.Price, "original_price": p.OriginalPrice, "emoji": p.Emoji, "color": p.Color, "category": p.Category}
```

- [ ] **Step 6: 运行测试确认通过**

Run: `go test ./internal/api/ -run 'TestSanitizeProductDetail|TestAdminCreateProductWithDetail'`
Expected: PASS。

- [ ] **Step 7: 提交**

```bash
git add internal/model/product.go internal/api/handlers_admin.go internal/api/handlers_admin_test.go internal/api/payment_test.go go.mod go.sum
git commit -m "feat: 商品 Detail 字段 + bluemonday 清洗 + create/update 落库"
```

---

### Task 2: 图片上传复用 + 商品图上传接口

**Files:**
- Modify: `internal/api/handlers_user.go`
- Modify: `internal/api/server.go`
- Modify: `internal/api/handlers_user_test.go`（或新增 `internal/api/upload_test.go`）

**Interfaces:**
- Produces: `saveImage(c *gin.Context, dir, urlPrefix string) (string, bool)`
- Produces: `uploadProductImage() gin.HandlerFunc`（Task 3 前端调用其路由）

- [ ] **Step 1: 写失败测试**

新建 `internal/api/upload_test.go`：

```go
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
	c, _, _ := uploadCtx(t, "a.png", []byte("fakepng"))
	url, ok := saveImage(c, dir, "/uploads/products/")
	if !ok || !strings.HasPrefix(url, "/uploads/products/") {
		t.Fatalf("saveImage = %q, %v", url, ok)
	}
	name := strings.TrimPrefix(url, "/uploads/products/")
	if _, err := os.Stat(filepath.Join(dir, name)); err != nil {
		t.Fatalf("文件未落盘: %v", err)
	}
}
```

- [ ] **Step 2: 运行测试确认失败**

Run: `go test ./internal/api/ -run TestSaveImage`
Expected: 编译失败（`saveImage` 未定义）。

- [ ] **Step 3: 抽取 saveImage 并新增 uploadProductImage**

`internal/api/handlers_user.go`：把 `uploadAvatar` 替换为：

```go
// saveImage 校验并保存上传图片到 dir，返回可访问的相对 URL；失败时已写好响应并返回 false。
func saveImage(c *gin.Context, dir, urlPrefix string) (string, bool) {
	file, err := c.FormFile("file")
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"message": "请选择图片文件"})
		return "", false
	}
	ext := strings.ToLower(filepath.Ext(file.Filename))
	allowed := map[string]bool{".jpg": true, ".jpeg": true, ".png": true, ".gif": true, ".webp": true}
	if !allowed[ext] {
		c.JSON(http.StatusBadRequest, gin.H{"message": "仅支持 jpg/png/gif/webp 图片"})
		return "", false
	}
	if file.Size > 2<<20 {
		c.JSON(http.StatusBadRequest, gin.H{"message": "图片不能超过 2MB"})
		return "", false
	}
	if err := os.MkdirAll(dir, 0o755); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"message": "保存失败"})
		return "", false
	}
	filename := fmt.Sprintf("%d%s", time.Now().UnixNano(), ext)
	if err := c.SaveUploadedFile(file, filepath.Join(dir, filename)); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"message": "保存失败"})
		return "", false
	}
	return urlPrefix + filename, true
}

// uploadAvatar 处理头像上传，保存到 uploads/avatars/ 并返回可访问的路径。
func uploadAvatar() gin.HandlerFunc {
	return func(c *gin.Context) {
		if url, ok := saveImage(c, "uploads/avatars", "/uploads/avatars/"); ok {
			c.JSON(http.StatusOK, gin.H{"url": url})
		}
	}
}

// uploadProductImage 处理商品详情图片上传，保存到 uploads/products/。
func uploadProductImage() gin.HandlerFunc {
	return func(c *gin.Context) {
		if url, ok := saveImage(c, "uploads/products", "/uploads/products/"); ok {
			c.JSON(http.StatusOK, gin.H{"url": url})
		}
	}
}
```

`internal/api/server.go` 在 admin 商品路由附近新增：

```go
	admin.POST("/products/upload", requirePermission(db, "product:manage"), uploadProductImage())
```

- [ ] **Step 4: 运行测试确认通过**

Run: `go test ./internal/api/ -run TestSaveImage`
Expected: PASS。

- [ ] **Step 5: 运行全量测试**

Run: `go test ./internal/api/`
Expected: 全部 PASS（含头像上传相关的既有测试，若有）。

- [ ] **Step 6: 提交**

```bash
git add internal/api/handlers_user.go internal/api/server.go internal/api/upload_test.go
git commit -m "feat: 商品图上传接口（复用 saveImage 助手，uploads/products）"
```

---

### Task 3: 前端 RichEditor 组件 + 管理端表单接入

**Files:**
- Create: `frontend/src/components/RichEditor.vue`
- Modify: `frontend/src/views/admin/ProductsView.vue`
- Modify: `frontend/package.json`（`npm install` 自动）

**Interfaces:**
- Consumes: `POST /api/admin/products/upload`（Task 2）
- Consumes: `avatarSrc(url)`（`frontend/src/lib/avatar.ts`）
- Produces: `<RichEditor v-model="form.detail" />` 组件（`v-model` 绑定 HTML 字符串）

- [ ] **Step 1: 安装 wangEditor**

Run: `cd frontend && npm install @wangeditor/editor`

- [ ] **Step 2: 创建 RichEditor 组件**

`frontend/src/components/RichEditor.vue`：

```vue
<script setup lang="ts">
import { onBeforeUnmount, onMounted, ref } from "vue";
import { showToast } from "vant";
import { createEditor, createToolbar, type IDomEditor, type IEditorConfig } from "@wangeditor/editor";
import "@wangeditor/editor/dist/css/style.css";
import http from "../lib/http";
import { avatarSrc } from "../lib/avatar";

const props = defineProps<{ modelValue: string }>();
const emit = defineEmits<{ (e: "update:modelValue", v: string): void }>();

const toolbarEl = ref<HTMLDivElement>();
const editorEl = ref<HTMLDivElement>();
let editor: IDomEditor | null = null;

onMounted(() => {
  const config: Partial<IEditorConfig> = {
    placeholder: "请输入图文详情",
    MENU_CONF: {
      uploadImage: {
        async customUpload(file: File, insertFn: (url: string) => void) {
          try {
            const fd = new FormData();
            fd.append("file", file);
            const res = await http.post("/api/admin/products/upload", fd);
            insertFn(avatarSrc(res.data.url));
          } catch (err) {
            const e = err as { response?: { data?: { message?: string } } };
            showToast(e?.response?.data?.message || "图片上传失败");
          }
        },
      },
    },
    onChange(ed) {
      emit("update:modelValue", ed.getHtml());
    },
  };

  editor = createEditor({
    selector: editorEl.value!,
    html: props.modelValue,
    config,
    mode: "default",
  });
  createToolbar({
    editor,
    selector: toolbarEl.value!,
    mode: "default",
  });
});

onBeforeUnmount(() => {
  editor?.destroy();
  editor = null;
});
</script>

<template>
  <div class="rich-editor">
    <div ref="toolbarEl" class="rich-toolbar"></div>
    <div ref="editorEl" class="rich-content"></div>
  </div>
</template>

<style scoped>
.rich-editor { width: 100%; border: 1px solid #ebedf0; border-radius: 6px; }
.rich-toolbar { border-bottom: 1px solid #ebedf0; }
.rich-content { min-height: 220px; overflow-y: auto; }
</style>
```

- [ ] **Step 3: 接入 ProductsView 表单**

`frontend/src/views/admin/ProductsView.vue`：

script 顶部新增 import：

```ts
import RichEditor from "../../components/RichEditor.vue";
```

`form` 新增 `detail` 字段（`reactive` 内加一行）：

```ts
const form = reactive({
  name: "",
  description: "",
  detail: "",
  price: "",
  ...
});
```

`openCreate` 的 `Object.assign` 加 `detail: ""`；`openEdit` 的 `Object.assign` 加 `detail: p.detail || ""`。

`buildBody` 返回对象加 `detail: form.detail`：

```ts
return {
  name: form.name,
  description: form.description,
  detail: form.detail,
  price: Number(form.price) || 0,
  ...
};
```

模板：在「描述」`van-field` 之后、价格之前新增详情编辑块：

```html
<van-field label="图文详情">
  <template #input>
    <RichEditor v-if="showForm" v-model="form.detail" />
  </template>
</van-field>
```

- [ ] **Step 4: 构建校验**

Run: `cd frontend && npm run build`
Expected: ✓ built，无 TS 报错。

- [ ] **Step 5: 提交**

```bash
git add frontend/src/components/RichEditor.vue frontend/src/views/admin/ProductsView.vue frontend/package.json frontend/package-lock.json
git commit -m "feat: 管理端商品表单接入 wangEditor 图文详情编辑"
```

---

### Task 4: 详情页渲染 + ShopProduct 类型

**Files:**
- Modify: `frontend/src/stores/shop.ts`
- Modify: `frontend/src/views/ProductDetailView.vue`

**Interfaces:**
- Consumes: `model.Product.detail`（Task 1，JSON `detail`）

- [ ] **Step 1: ShopProduct 加 detail 字段**

`frontend/src/stores/shop.ts` 的 `ShopProduct` 接口加一行：

```ts
export interface ShopProduct {
  id: number; name: string; description: string; detail?: string; price: number; originalPrice: number;
  sales: number; emoji: string; color: string; category: string;
}
```

- [ ] **Step 2: 详情页渲染详情区块**

`frontend/src/views/ProductDetailView.vue`：在 `.detail-main` section 与 `.detail-reviews` section 之间插入：

```html
<section v-if="product.detail" class="detail-rich">
  <h3>商品详情</h3>
  <div class="detail-rich-content" v-html="product.detail"></div>
</section>
```

`<style scoped>` 追加：

```css
.detail-rich { margin: 12px; padding: 12px; background: #fff; border-radius: 10px; }
.detail-rich h3 { margin: 0 0 8px; font-size: 16px; }
.detail-rich-content { font-size: 14px; color: #1f2430; line-height: 1.7; }
.detail-rich-content :deep(img) { max-width: 100%; height: auto; display: block; }
.detail-rich-content :deep(table) { border-collapse: collapse; width: 100%; }
.detail-rich-content :deep(td), .detail-rich-content :deep(th) { border: 1px solid #ebedf0; padding: 6px; }
```

- [ ] **Step 3: 构建校验**

Run: `cd frontend && npm run build`
Expected: ✓ built，无 TS 报错。

- [ ] **Step 4: 提交**

```bash
git add frontend/src/stores/shop.ts frontend/src/views/ProductDetailView.vue
git commit -m "feat: 商品详情页渲染图文详情（v-html）"
```

---

## Self-Review

- **Spec 覆盖**：模型字段/清洗/上传/编辑器/详情渲染/安全/测试 均有对应任务。`saveImage` 复用头像逻辑 ✓；`product:manage` 权限 ✓；相对路径 `avatarSrc` 解析 ✓。
- **占位符**：无 TBD/TODO；每个代码步骤都有实际代码。
- **类型一致性**：`sanitizeProductDetail(html string) string`、`saveImage(c, dir, urlPrefix) (string, bool)`、`uploadProductImage()`、`ShopProduct.detail?: string`、`<RichEditor v-model>` 在前后任务中命名一致。
- **Review Focus**：5 条分别由 `TestSanitizeProductDetail`、`TestSaveImageRejectsBadExt`、`TestSaveImageRejectsOversize`、`TestSanitizeProductDetail`+构建、`v-if`+构建覆盖。
