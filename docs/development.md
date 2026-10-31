# 开发指南

## 环境要求

- Go 1.26（版本来自 `go.mod`）。
- Node.js 24、npm；CI 使用 Node 24。
- MySQL 8，默认连接地址是 `192.168.57.47:3306`，生产必须通过环境变量覆盖凭据和地址。
- Redis 6+ 可选，默认地址 `192.168.57.47:6379`。
- Wails 3 beta 适用于桌面开发；Linux 桌面构建还需要 GTK/WebKit 开发包。
- Docker 用于服务端镜像和跨平台 CGO 构建。

## 依赖安装

```powershell
go mod download
cd frontend
npm ci
cd ..
```

## 配置变量

### 服务端

| 变量 | 默认值 | 说明 |
| --- | --- | --- |
| `MYSQL_HOST` | `192.168.57.47` | MySQL 主机 |
| `MYSQL_PORT` | `3306` | MySQL 端口 |
| `MYSQL_USER` | `root` | MySQL 用户 |
| `MYSQL_PASSWORD` | `123456` | 仅开发默认值，生产必须覆盖 |
| `MYSQL_DATABASE` | `shop` | 数据库名 |
| `REDIS_ADDR` | `192.168.57.47:6379` | Redis 地址 |
| `REDIS_PASSWORD` | 空 | Redis 密码 |
| `REDIS_DB` | `0` | Redis 库号 |
| `API_PORT` | `8080` | HTTP 监听端口 |
| `JWT_SECRET` | 启动时随机生成 | JWT 密钥；生产必须固定且保密 |
| `CORS_ALLOWED_ORIGINS` | 空 | 额外允许的来源，逗号分隔 |
| `ENABLE_SIMULATORS` | 空 | 严格为 `true` 时启动订单模拟器，仅测试使用 |

Redis 不可用时应用会启动并降级。`JWT_SECRET` 未设置时每次重启都会让旧 JWT 失效，不适合多实例或生产环境。

### 前端

| 变量 | 默认值 | 说明 |
| --- | --- | --- |
| `VITE_API_BASE` | `http://127.0.0.1:8080` | API 基础地址 |
| `VITE_AMAP_KEY` | 空 | 高德地图 Web JS API key |
| `VITE_AMAP_SECURITY_KEY` | 空 | 高德安全密钥 |
| `WAILS_VITE_PORT` | `9245` | Wails 开发前端端口 |
| `WAILS_VITE_HOST` | `0.0.0.0` | Vite 开发监听地址 |

`frontend/.env.example` 只包含高德变量示例；本地配置复制为 `.env.local`，不要提交密钥。

### 支付宝

支付宝支持 `config/alipay.local.json` 或环境变量 `ALIPAY_*`。配置优先级、密钥格式、沙箱/生产和回调限制见[支付宝配置](alipay-config.md)。

## 本地运行

### Wails 开发模式

```powershell
task dev
```

该任务会启动 Wails、Vite 前端和 Go 服务，前端默认使用 9245 端口。

### 单独运行前端

```powershell
cd frontend
npm run dev -- --host 127.0.0.1 --port 9245
```

### 运行服务端

```powershell
# 先保证 frontend/dist 已生成
go run -tags server .
```

服务端模式需要先存在 `frontend/dist`，因为主入口通过 `go:embed` 引用前端产物。服务端默认监听 8080。

## 测试和构建

```powershell
# 前端
cd frontend
npm test
npm run typecheck
npm run build
cd ..

# 后端
go vet ./...
go test ./...
go build ./...
```

前端测试使用 `node --test tests/*.test.mjs`，包含购物车状态/组件和富文本编辑器测试。后端测试使用 SQLite 临时库；真实 MySQL 购物车测试：

```powershell
$env:SHOP_MYSQL_INTEGRATION = '1'
go test ./internal/api -run '^TestMySQLCart' -count=1 -v
```

## 代码约定

- 后端业务处理器放在 `internal/api`，模型只描述持久化结构，不在模型中写请求处理逻辑。
- 所有用户资源查询都必须带当前用户条件；管理接口必须使用权限中间件。
- 金额用 `model.Price`，不要引入浮点金额计算。
- Redis 只做缓存/计数，不把业务正确性建立在缓存命中上。
- 前端 API 错误应显示可行动提示；购物车和结算错误需要保留本地状态或明确要求重新确认。
- 新增数据库字段时同时更新模型、迁移/启动迁移、SQLite 测试夹具和 MySQL 集成测试。
