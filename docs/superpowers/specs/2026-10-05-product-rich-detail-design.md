# 商品图文详情（富文本）设计

- 日期：2026-10-05
- 状态：待评审
- 规模：架构级（引入富文本编辑器 + 后端 HTML 清洗）

## 目标

给商品增加「图文详情」：管理端用富文本编辑器编写图文混排的详情内容（文字 + 内联图片），详情页安全地渲染。替换当前「一句话 `description` + emoji 色块图」的极简详情。

## 现状

- `model.Product` 只有 `Description`（一句话），无详情字段。
- 详情页 `ProductDetailView.vue` 图片区是「emoji + 色块背景」，正文只有一行短描述。
- 已有头像上传 `uploadAvatar`（存 `uploads/avatars/`，返回相对路径 `/uploads/avatars/<file>`），前端用 `avatarSrc` 把 `/uploads` 相对路径拼成完整 URL。
- 管理端商品表单 `ProductsView.vue` 无富文本、无图片上传。

## 关键决策

1. **编辑器：wangEditor v5**（`@wangeditor/editor`）。用其框架无关核心 API（`createEditor` / `createToolbar`）在 Vue 组件内手动挂载/销毁，规避 `@wangeditor/editor-for-vue` 对 Vue 3.5 的兼容风险。
2. **存储：`Product.Detail` 存 HTML 字符串**（`gorm:"type:text"`），图片内联在 HTML 中，不建独立 `Images` 数组、不建独立图片表。
3. **XSS：后端存库前用 `github.com/microcosm-cc/bluemonday` 白名单清洗**，剥掉 script/事件/危险标签；前端 `v-html` 渲染已清洗内容。
4. **图片上传：复用头像上传逻辑**，抽公共助手，新增 `uploads/products/` 目录；编辑器上传走 axios `http` 实例（自动带鉴权 + baseURL），插入的图片 URL 用 `avatarSrc` 解析成完整地址，保证编辑器预览与详情页都可直接显示。

## 数据模型

`shop_products` 新增一列（`AutoMigrate` 自动加列）：

| 字段 | 类型 | 说明 |
|---|---|---|
| detail | text | 图文详情 HTML（已清洗） |

## 后端设计

- `internal/api/handlers_user.go`：把 `uploadAvatar` 的上传校验/落盘逻辑抽成 `saveImage(c, dir) (url string, ok bool)`；新增 `uploadProductImage()` 存到 `uploads/products/`，返回 `{url}`（相对路径，同头像）。
- `internal/api/server.go`：新增 `admin.POST("/products/upload", requirePermission(db, "product:manage"), uploadProductImage())`。
- `internal/api/handlers_admin.go`：
  - `productPayload` 加 `Detail string \`json:"detail"\``。
  - `adminCreateProduct` / `adminUpdateProduct` 写 `detail`（写入前 `sanitizeProductDetail(p.Detail)`）。
- 新增 `sanitizeProductDetail(html string) string`：`bluemonday.UGCPolicy()` + `AllowRelativeURLs(true)`，白名单覆盖 wangEditor 常见输出（`p/br/strong/em/u/s/h1-h6/ul/ol/li/blockquote/a[href]/img[src,alt,width,height]/pre/code`，按需补 `hr/table`）。
- 详情接口 `productDetailHandler` 返回整个 Product，`detail` 自动带出，无需改。

## 前端设计

- `frontend/src/views/admin/ProductsView.vue`：
  - 表单弹窗加一个编辑器容器（工具栏 + 内容区）。
  - `onMounted` 创建编辑器，`onBeforeUnmount` 销毁。
  - 图片上传用 `MENU_CONF.uploadImage.customUpload(file, insertFn)`：构造 `FormData`，用 `http.post('/api/admin/products/upload', fd)`，成功后 `insertFn(avatarSrc(res.data.url))`。
  - `openEdit` 把 `p.detail` 塞进编辑器；`buildBody` 带 `detail`（读编辑器当前 HTML）。
- `frontend/src/views/ProductDetailView.vue`：新增「商品详情」区块，`<div v-html="product.detail">`（内容已在后端清洗；无 `detail` 时隐藏该区块）。
- `frontend/src/stores/shop.ts`：`ShopProduct` 类型加 `detail?: string`。

## 安全

- 后端 `bluemonday` 白名单是唯一权威清洗点（无论前端如何，存库前必清洗）。
- `AllowRelativeURLs(true)` 让 `/uploads/...` 图片路径存活；编辑器实际插入的是 `avatarSrc` 解析后的完整 http URL。
- 前端 `v-html` 只渲染后端已清洗的 `detail`，不直接渲染用户原始输入。

## 测试

- 后端：`TestSanitizeProductDetail`（剥 script/onclick，保留 p/img/相对路径）；`TestAdminCreateProductWithDetail`（create 带 detail 落库断言）；`TestUploadProductImage`（非法扩展名/超大被拒）。
- 前端：`npm run build` 校验类型与构建。

## 非目标（YAGNI）

- 不引入富文本之外的图片库/画廊组件。
- 不做商品主图/封面图（仍用 emoji 色块）；本需求只做「详情」内容。
- 不做前端 DOMPurify（后端清洗已覆盖，双保险留待以后）。
