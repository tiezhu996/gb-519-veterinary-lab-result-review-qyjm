# 验证记录

验证日期：2026-08-22（Asia/Shanghai）

## 代码质量

以下命令均实际执行成功：

```bash
cd backend
go test ./...
go test -race ./...
go vet ./...
go build ./...

cd ../frontend
npm run typecheck
npm run build

cd ..
docker compose config --quiet
```

- 非测试 Go 代码：3136 行。
- 非测试 `.go` 文件：38 个。
- 服务层回归测试覆盖草稿 v1/v2、提交 v3、独立复核 v4、operator 越权签发、同人伪装 reviewer、复核后编辑锁定，以及版本证据和原子审计数量。

## 空卷 Compose 与 API

执行 `KEEP_RUNNING=1 ./scripts/validate.sh`，脚本先运行 `docker compose down -v --remove-orphans`，再从空命名卷构建并启动 MySQL、Redis、backend、frontend。验证结果：

- `/healthz` 返回 database=`ready`、redis=`ready`。
- admin/reviewer/operator/viewer 四个账号均可获得真实 JWT 会话。
- viewer 可读取来源记录，写请求与审计请求均返回 HTTP 403。
- operator 创建签发草稿 v1，编辑证据形成 v2，提交 peer_review 形成 v3；其签发请求返回 HTTP 422。
- admin 创建并提交自己的签发草稿后，自签请求返回 HTTP 422，验证异人复核不只依赖角色名。
- 独立 reviewer 将 operator 的签发记录签为 v4；`preparedBy=operator`、`reviewedBy=reviewer`。
- 四个修订均保留非空 evidence、actor、request ID，且请求 ID 顺序与实际四次成功请求一致。
- 实体审计历史与审计汇总覆盖创建、编辑、提交、签发和至少两个独立操作者。

## 内置 Browser

仅使用 Codex 内置 Browser，在 `http://127.0.0.1:18519` 实际验证：

- 登录页可切换 admin/reviewer/operator/viewer 并建立真实会话。
- `/cases`、`/specimens`、`/assays`、`/signoff`、`/audit` 五个页面均正常加载。
- `RiskTag` 在来源和样本页渲染；`ResultPanel` 在检测运行和结果签发页展示证据、版本、操作者与 request ID。
- admin 通过确认对话框将 AC-001 从 registered 推进到 sampling，页面刷新后的目标行状态正确。
- operator 在 peer_review 记录上只看到“等待复核员”；同一制单人在可复核角色下显示“需异人复核”。
- reviewer 通过确认对话框将 RS-002 从 peer_review 签为 signed，目标行变为终态。
- viewer 不显示新增或推进按钮，不显示审计导航，直接访问 `/audit` 会重定向到 `/cases`。
- 390 x 844 下 `innerWidth`、`body.scrollWidth`、`documentElement.scrollWidth` 均为 390；宽表格仅在内部滚动。
- 桌面和移动端首屏/结果区截图已检查，未发现文字遮挡或控件重叠；控制台 error/warning 日志为空。

## 清理

最终执行：

```bash
docker compose down -v --remove-orphans
```

并确认没有名称包含 `veterinary-lab-result-review` 的容器、网络或数据卷。
