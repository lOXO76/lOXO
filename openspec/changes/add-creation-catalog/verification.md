# 后端工人交付报告

## 完成内容

在已有未提交开关改动上完成复核与补全，新增 JWT 目录接口 `GET /api/v1/user/creation/catalog`。功能关闭时返回 404，`reason=CREATION_DISABLED`。生成请求继续使用用户 API Key 走现有网关。

未修改前端共享契约、前端文件、`go.mod`、`go.sum` 或依赖锁文件；未提交 Git。

## 改动文件清单

以下路径均相对于仓库根目录，包含接续任务保留并核验的开关半成品：

```text
backend/cmd/server/wire_gen.go
backend/internal/handler/admin/setting_handler.go
backend/internal/handler/admin/setting_handler_audit.go
backend/internal/handler/admin/setting_handler_update.go
backend/internal/handler/admin/setting_handler_creation_test.go
backend/internal/handler/creation_handler.go
backend/internal/handler/dto/public_settings_injection_schema_test.go
backend/internal/handler/dto/settings.go
backend/internal/handler/handler.go
backend/internal/handler/setting_handler.go
backend/internal/handler/setting_handler_creation_test.go
backend/internal/handler/wire.go
backend/internal/server/api_contract_test.go
backend/internal/server/routes/creation_catalog_test.go
backend/internal/server/routes/user.go
backend/internal/service/creation_catalog.go
backend/internal/service/creation_catalog_test.go
backend/internal/service/domain_constants.go
backend/internal/service/setting_parse.go
backend/internal/service/setting_public.go
backend/internal/service/setting_service.go
backend/internal/service/setting_service_creation_test.go
backend/internal/service/setting_update.go
backend/internal/service/settings_view.go
backend/internal/service/wire.go
openspec/changes/add-creation-catalog/design.md
openspec/changes/add-creation-catalog/verification.md
```

## 关键设计决定

- 权限直接复用 `APIKeyService.GetAvailableGroups`，未新增权限模型。公共字段由显式结构序列化；路由测试核验完整 JSON 白名单。
- 开关使用默认开启语义，覆盖初始化、读写、管理端缺省/null 保留、审计、公共 DTO、SSR 注入及接口契约快照。
- 异步生图复用 `ImageTaskService.Enabled()`；视频账号复用 `IsSchedulable`、端点能力检查及视频能力读取函数。Grok 视频生成沿用网关共用的分组生图开关。
- MiniMax 版本按网关同一函数解析上游路径。v1 账号只列 v1，v2 账号列 v1/v2，因为网关支持旧请求转换。公开名或映射目标为 H3 的模型在 v1 账号下排除。
- 模型采用媒体定价、分组视频价、公开映射名和渠道别名，并应用分组白名单、渠道模型限制；不暴露映射目标。未知视频协议归属返回空数组，模型源为空同样正常返回。
- 每请求批量读取账号，不调用上游，不缓存用户权限与调度状态。渠道复用已有十分钟缓存及原有更新失效/跨实例通知。

## 验证

命令均在 `backend` 执行，环境自动使用 Go 1.27.3。

- `go run github.com/google/wire/cmd/wire ./cmd/server`：成功生成依赖注入。
- `go build ./...`：通过。
- `go test -tags=unit ./internal/service -run CreationCatalog -count=1`：通过，23 个顶层能力测试。
- `go test -tags=unit ./internal/handler ./internal/handler/admin ./internal/handler/dto ./internal/server/routes -run 'Creation|SchemaDoesNotDrift' -count=1`：全部通过，包含 5 个目录路由测试。
- `go test ./internal/service ./internal/handler ./internal/handler/admin ./internal/handler/dto ./internal/server ./internal/server/routes`：全部通过，普通构建下 server 包无测试。
- `go test -tags=integration ./internal/server/routes/...`：通过。
- `go test ./cmd/server`、`go test -tags=unit ./cmd/server`：通过。
- `go test -tags=unit ./internal/server -run TestAPIContracts -count=1`：通过。
- `go test -tags=unit ./internal/...`：最终全量通过，日志保存在 `/tmp/creation-backend-unit-final.log`。
- 仅改动 Go 文件的 `gofmt`、`git diff --check`：通过。

首次全量测试暴露了本次新增字段和能力逻辑的两类问题，均已修复：

```text
--- FAIL: TestAPIContracts
    --- FAIL: TestAPIContracts/GET_/api/v1/admin/settings
    --- FAIL: TestAPIContracts/GET_/api/v1/admin/settings_falls_back_to_config_oauth_defaults
        + (string) (len=16) "creation_enabled": (bool) true,
--- FAIL: TestCreationCatalogMiniMaxVersions
    --- FAIL: TestCreationCatalogMiniMaxVersions/H3_cannot_use_v1_account
        Error: Should be empty, but was [{public-video []}]
```

后续新增回归用例还识别出公开名为 H3、映射目标非 H3 的 v1 账号情形：

```text
--- FAIL: TestCreationCatalogMiniMaxVersions/public_H3_cannot_use_v1_account
    Error: Should be empty, but was [{MiniMax-H3 [minimax_v1]}]
```

已补公开模型名检查，最终目录定向测试及接口契约测试全部通过。

## 未完成项与风险

- 代码实现无已知未完成项；未进行真实上游生图/生视频调用。
- 分辨率/时长为账号能力并集和包络，具体模型与选中账号仍可能有更严格限制；未配置能力时以协议默认值为准。
- 未配置公开模型名时返回空模型数组；未知视频模型协议归属保留空协议数组。用户手动模型名仍由网关最终校验。
- 渠道元数据读取失败时目录退化到账号/分组模型来源；权限或账号读取失败正常返回服务错误。
