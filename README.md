# 优选商城

优选商城是一个基于 Wails 3 的跨平台商城应用，同时提供可独立运行的 HTTP 服务端。项目包含用户端商城、管理后台、订单与支付、优惠券、评价、客服、通知推送、商品规格和购物车结算能力。

## 文档导航

- [项目总览](docs/project-overview.md)：目录、模块边界、请求链路和启动流程。
- [接口参考](docs/api-reference.md)：公开接口、登录接口、用户接口和管理接口。
- [数据模型](docs/data-model.md)：MySQL 表、核心字段、关系和迁移策略。
- [开发指南](docs/development.md)：环境准备、配置、开发运行和测试命令。
- [部署与运维](docs/operations.md)：MySQL、Redis、支付宝、服务端部署、备份和排障。
- [购物车一致性](docs/cart-consistency.md)：购物车版本控制、缓存隔离和幂等结算。
- [订单、售后、物流与客服](docs/order-service-workflows.md)：订单生命周期和客服处理规则。
- [支付宝配置](docs/alipay-config.md)：沙箱、生产配置和回调说明。

## 快速开始

项目需要 Go 1.26、Node.js 24、npm，以及可访问的 MySQL。Redis 是可选依赖；不可用时目录缓存、权限缓存和限流会降级到数据库或内存实现。

```powershell
# 安装前端依赖并构建
cd frontend
npm ci
npm test
npm run build
cd ..

# 运行后端和完整 Go 测试
go test ./...

# Wails 开发模式
task dev
```

服务端默认监听 `:8080`，前端默认请求 `http://127.0.0.1:8080`。MySQL、Redis、支付宝和 CORS 配置均可通过环境变量覆盖，具体见[开发指南](docs/development.md)。

## 构建方式

```powershell
# 桌面应用，按当前操作系统构建
task build

# 仅构建 HTTP 服务端
task build:server

# 构建 Docker 服务端镜像
task build:docker

# 启动已构建的服务端
task run:server
```

`main.go` 会通过 `go:embed` 嵌入 `frontend/dist`，因此完整桌面构建或 Go 编译前必须先完成前端构建。生产服务端使用 `server,production` 构建标签，容器默认暴露 8080 端口。

## 当前验证状态

仓库当前包含前端组件/状态测试、Go 单元测试、购物车 MySQL 集成测试和生产构建检查。推荐在提交前执行：

```powershell
cd frontend
npm test
npm run build
cd ..
go test ./...
```

真实 MySQL 购物车测试需要显式设置 `SHOP_MYSQL_INTEGRATION=1`，测试会创建并删除独立临时数据库，不会使用业务库。

## 重要边界

- 金额在后端以整数分保存，前端展示为元；订单提交时以后端商品、规格、库存和优惠券数据为准。
- 支付未配置时，商城浏览、购物车和下单接口仍可使用，收银台会返回支付未配置；真实支付必须使用自己的支付宝沙箱或生产凭据验证。
- Redis 不是数据源。Redis 不可用时，业务数据仍使用 MySQL，缓存与限流功能按实现降级。
- 物流轨迹由商家录入，当前没有接入快递公司的自动轨迹服务。
- 售后当前以整单退款为主，部分退款、换货和图片举证不在现有范围内。

## 项目状态约定

工作区可能包含尚未提交的功能改动；文档描述以当前工作区代码为准。不要把 `config/alipay.local.json`、私钥、数据库密码或其他本地凭据提交到仓库。
