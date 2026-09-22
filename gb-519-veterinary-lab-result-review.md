请生成 `veterinary-lab-result-review`「兽医检验样本结果复核」Go 全栈项目，面向动物疫病检测实验室管理样本接收、检测方法、结果复核和签发，不做宠物医院预约、商城、账务或社交社区。

## 项目主要需求

复杂度下限：核心实体不少于 3 个、核心页面不少于 4 个、横切关注点不少于 2 个、共享前端组件不少于 3 个、自定义 hooks/utils 不少于 2 个、后端中间件不少于 2 个。

### 核心实体

`AnimalCase`（动物样本来源与风险级别）、`Specimen`（样本与保存条件）、`AssayRun`（检测运行与方法版本）、`ResultSignoff`（结果复核与签发）全链路贯穿数据库、Go 分层和前端。

### 核心页面

`/cases` 样本来源；`/specimens` 样本接收；`/assays` 检测运行；`/signoff` 结果签发；`/audit` 审计。`RiskTag` 在来源和样本页共用，`ResultPanel` 在运行和签发页共用。

### 横切关注点

RBAC + 双人复核联动 DB 角色、Go middleware、前端守卫和显隐；结果签发必须保留版本、证据、操作者和 request ID；全局错误处理、限流、脱敏日志独立实现。

### 共享枚举/组件

同步 `SpecimenState`（received/testing/hold/released/disposed）与 `SignoffState`（draft/peer_review/signed/rejected）。共享 `StatusBadge`、`ResultPanel`、`EmptyState`，hooks 为 `useAuth`、`usePagination`。

### 技术与规模要求

前端 React 18 + TypeScript + Vite + Material UI；后端 Go 1.22 + Gin + GORM；MySQL + Redis。目标 2700–3900 行、26–38 个 `.go` 文件。

### 文件结构强制清单

前端 `api/stores/types/components/common/hooks/pages/router/utils`；后端 `model/dto/repository/service/handler/router/middleware/constants/util`，每个实体分文件。

### 结构红线

严禁合并职责到单一文件；样本、检测运行和签发必须在多个层次分别实现。

### 部署与交付

根目录必须提供 `docker-compose.yml`（顶层 `name: veterinary-lab-result-review`，且不写 `version:`）、`.env` 和 `.env.example`（均含 `COMPOSE_PROJECT_NAME=veterinary-lab-result-review`）、`README.md`、`frontend/Dockerfile`、`backend/Dockerfile` 和 `frontend/nginx.conf`。前端端口 `18519`、后端端口 `19519`；Nginx `/api` 反代、healthcheck、命名卷和 `condition: service_healthy` 齐全，提供真实 `/healthz`、Git 初始化。
