# WeatherNow — 实时动态天气卡片

基于用户地理位置的实时天气展示应用。前端通过浏览器 Geolocation API 或 IP 定位获取用户位置，后端聚合高德地图多源 API 与离线 IP 数据库，提供精确到区县级的天气数据服务，前端根据天气状况渲染对应的粒子动画卡片。

## 核心特性

- **多源定位**：GPS 精确定位（区县级） + IP 定位（城市级），自动降级
- **离线 IP 库**：集成 ip2region，支持 IPv4/IPv6 离线解析，减少外部 API 依赖
- **智能降级链**：ip2region → 高德 IP API → 默认值，保障服务可用性
- **完整城市名**：GPS 定位时组合展示"市+区"（如"深圳市龙华区"），IP 定位展示城市级名称
- **内存缓存**：5 分钟 TTL 缓存，避免重复 API 调用
- **动态天气卡片**：根据天气状况（晴/云/雨/雪）渲染对应粒子动画
- **安全日志**：API Key 自动脱敏，日志中不暴露敏感信息
- **可测试架构**：HTTP 客户端与 IP 查询引擎均通过接口抽象，支持 Mock 注入

## 技术架构

```
┌─────────────────────────────────────────────────────────────┐
│                      Browser (Frontend)                      │
│  weather-cards.html · Geolocation API · CSS Animations      │
└──────────────────────────┬──────────────────────────────────┘
                           │ GET /weather?latitude=&longitude=
                           │ GET /weather?ip=
                           ▼
┌─────────────────────────────────────────────────────────────┐
│                go-zero REST Server (:8889)                    │
│  ┌──────────┐   ┌──────────────────┐   ┌────────────────┐   │
│  │  Handler  │──▶│  GetWeatherLogic │──▶│  In-Memory     │   │
│  │ (路由/参数)│   │  (业务编排)       │   │  Cache (5min)  │   │
│  └──────────┘   └───────┬──────────┘   └────────────────┘   │
│                         │                                    │
│         ┌───────────────┼───────────────┐                    │
│         ▼               ▼               ▼                    │
│  ┌─────────────┐ ┌─────────────┐ ┌──────────────┐           │
│  │ ip2region   │ │ Amap IP API │ │ Amap Regeo   │           │
│  │ (离线IPv4/6)│ │ (在线IPv4)  │ │ (逆地理编码)  │           │
│  └──────┬──────┘ └──────┬──────┘ └──────┬───────┘           │
│         │               │               │                    │
│         └───────────────┼───────────────┘                    │
│                         ▼                                    │
│                ┌─────────────────┐                           │
│                │ Amap Weather API│                           │
│                │ (天气查询)       │                           │
│                └─────────────────┘                           │
└─────────────────────────────────────────────────────────────┘
```

### 定位降级策略

| 场景 | 定位路径 | 精度 |
|------|---------|------|
| GPS 坐标可用 | 逆地理编码 → 区县 adcode | 区县级 |
| IP + ip2region 可用 | 离线 IP → 城市名 → 地理编码 → adcode | 城市级 |
| IP + 高德 IP API 可用 | 高德 IP API → adcode | 城市级 |
| IPv6 地址 | ip2region → 高德 IP API（跳过）→ 默认 | — |
| 本机/回环 IP | 默认 adcode（北京 110000） | — |

## 项目结构

```
.
├── weather-cards.html              # 前端单页应用
├── start.sh                        # 一键启动脚本
├── backend/
│   └── weatherapi/
│       ├── weatherapi.api          # go-zero API 定义文件
│       ├── weatherapi.go           # 服务入口
│       ├── .env                    # 环境变量配置
│       ├── etc/
│       │   └── weatherapi-api.yaml # 服务配置
│       ├── data/
│       │   ├── ip2region_v4.xdb    # IPv4 离线数据库
│       │   └── ip2region_v6.xdb    # IPv6 离线数据库
│       ├── internal/
│       │   ├── config/config.go    # 配置结构定义
│       │   ├── handler/            # HTTP 请求处理层
│       │   │   ├── getweatherhandler.go
│       │   │   └── routes.go
│       │   ├── logic/              # 核心业务逻辑层
│       │   │   ├── getweatherlogic.go
│       │   │   ├── getweatherlogic_test.go
│       │   │   └── getweatherlogic_integration_test.go
│       │   ├── svc/                # 服务上下文与依赖注入
│       │   │   └── servicecontext.go
│       │   └── types/types.go      # 请求/响应类型（goctl 生成）
│       ├── setup_ip2region.sh      # IP 数据库下载脚本
│       └── run_tests.sh            # 测试执行脚本
```

## 技术选型

| 组件 | 技术 | 选型理由 |
|------|------|---------|
| Web 框架 | go-zero 1.10 | 高性能微服务框架，内置 API 定义、代码生成、日志、链路追踪 |
| 代码生成 | goctl 1.10 | 从 `.api` 文件生成 handler/types/routes，保证接口一致性 |
| 天气数据 | 高德天气 API | 国内数据准确，免费额度充足，支持 adcode 精确查询 |
| IP 定位（离线） | ip2region | 离线查询零延迟，支持 IPv4/IPv6，无外部依赖 |
| IP 定位（在线） | 高德 IP API | 作为离线库的降级备选，仅支持 IPv4 |
| 内存缓存 | go-cache | 轻量级 TTL 缓存，适合单实例部署场景 |
| 环境配置 | godotenv | `.env` 文件管理敏感配置，避免硬编码 |
| 前端 | 原生 HTML/CSS/JS | 零依赖，单文件部署，CSS 粒子动画实现天气效果 |

## 环境要求

- Go 1.23+
- goctl 1.10+（`go install github.com/zeromicro/go-zero/tools/goctl@latest`）
- 高德 Web 服务 API Key（[申请地址](https://lbs.amap.com/api/webservice/guide/create-project/get-key)）

## 安装部署

### 1. 克隆项目

```bash
git clone <repository-url>
cd ai_test
```

### 2. 配置环境变量

```bash
cd backend/weatherapi
cp .env.example .env
```

编辑 `.env` 文件，填入高德 API Key：

```env
ENV=dev
AMAP_API_KEY=你的高德Web服务API_Key
IP2REGION_V4_DB_PATH=data/ip2region_v4.xdb
IP2REGION_V6_DB_PATH=data/ip2region_v6.xdb
```

### 3. 下载 IP 离线数据库

```bash
bash setup_ip2region.sh
```

### 4. 启动服务

```bash
# 方式一：一键启动（从项目根目录）
bash start.sh

# 方式二：手动启动
cd backend/weatherapi
go run weatherapi.go -f etc/weatherapi-api.yaml
```

服务启动后监听 `http://localhost:45854`。

### 5. 访问前端

在浏览器中打开 `weather-cards.html`。

前端后端地址配置（优先级从高到低）：

1. **URL 参数**：`weather-cards.html?backend=http://your-server:45854`
2. **Meta 标签**：修改 `weather-cards.html` 中 `<meta name="api-base-url" content="http://localhost:45854">`
3. **自动检测**：未配置时使用当前页面的 `window.location.origin`

## 使用指南

### IP 定位模式（默认）

直接打开 `weather-cards.html`，系统根据客户端 IP 自动获取天气。也可通过 URL 参数指定 IP：

```
weather-cards.html?ip=120.36.211.78
```

### GPS 精确定位模式

点击卡片左上角定位按钮，浏览器将请求位置权限，授权后自动切换为 GPS 模式，展示区县级精确天气。

## API 文档

### GET /weather

查询实时天气数据。

**请求参数**

| 参数 | 类型 | 必填 | 说明 |
|------|------|------|------|
| latitude | float64 | 否 | 纬度（与 longitude 同时提供时启用 GPS 定位） |
| longitude | float64 | 否 | 经度（与 latitude 同时提供时启用 GPS 定位） |
| ip | string | 否 | IP 地址（未提供时自动提取客户端 IP） |

**响应示例**

```json
{
  "temperature": 26.5,
  "description": "多云",
  "city": "深圳市龙华区",
  "icon": "cloud"
}
```

**响应字段**

| 字段 | 类型 | 说明 |
|------|------|------|
| temperature | float64 | 实时温度（摄氏度） |
| description | string | 天气描述（中文） |
| city | string | 城市名称，GPS 定位时为"市+区"格式，IP 定位时为城市名 |
| icon | string | 天气图标标识：sun / cloud / rain / snow / unknown |

### API 定义文件

接口定义在 [weatherapi.api](backend/weatherapi/weatherapi.api)，修改后通过 goctl 重新生成代码：

```bash
cd backend/weatherapi
goctl api go --api weatherapi.api --dir . --style goZero
```

## 测试

```bash
cd backend/weatherapi

# 运行全部测试（含竞态检测）
bash run_tests.sh

# 仅运行单元测试
go test -race -count=1 -v ./internal/logic/ ./internal/handler/ ./internal/svc/

# 查看覆盖率
go tool cover -func=coverage.out
```

测试架构特点：

- **纯函数测试**：`buildDisplayCity`、`maskAPIKey`、`parseCityFromRegion` 等工具函数的表驱动测试
- **Mock 注入**：通过 `HTTPClient` 接口注入 Mock HTTP 服务器，测试 API 调用逻辑
- **接口抽象**：`IP2RegionSearcher` 接口解耦 ip2region 实现，支持测试替身
- **集成测试**：完整的 API 调用链路测试，覆盖成功/失败/边界场景

## 架构设计理念

### 分层解耦

遵循 go-zero 推荐的 Handler → Logic → Service 分层架构，Handler 负责参数解析与 IP 提取，Logic 负责业务编排，ServiceContext 管理共享依赖。各层通过接口交互，便于独立测试与替换。

### 降级容错

定位服务采用多级降级策略，任何一级失败不影响整体可用性。离线 IP 库优先使用，减少网络依赖；在线 API 作为补充；最终兜底默认值保障响应不中断。

### 安全考量

- API Key 通过环境变量注入，不硬编码在代码中
- 日志输出自动脱敏（`maskAPIKey`），URL 中的 Key 替换为星号
- CORS 按环境区分：dev 环境允许所有来源（`*`），sit/prod 环境通过 `CORSAllowOrigins` 配置白名单域名

### 扩展性

- **缓存层**：当前使用 go-cache 内存缓存，可替换为 Redis 实现分布式缓存
- **定位源**：通过接口抽象可扩展更多定位方式（如 WiFi 定位、基站定位）
- **天气源**：Logic 层可扩展为多天气源聚合，增加数据准确性
- **前端**：单文件架构可轻松集成到任何 Web 框架或构建系统

## 贡献规范

1. Fork 本仓库
2. 创建功能分支（`git checkout -b feature/your-feature`）
3. 修改 API 定义时，更新 `.api` 文件并用 `goctl` 重新生成代码
4. 确保所有测试通过（`bash run_tests.sh`）
5. 提交代码（`git commit -m 'Add your feature'`）
6. 推送分支（`git push origin feature/your-feature`）
7. 创建 Pull Request

## 许可证

MIT License

## 联系方式

如有问题或建议，请提交 Issue。
