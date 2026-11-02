# 创作目录接口

`GET /api/v1/user/creation/catalog` 使用面板 JWT，普通用户可访问。响应采用面板通用的 `data` 包装，`data` 中的结构与 `frontend/src/features/creation/types.ts` 的 `CreationCatalog` 一致。

目录只描述当前用户可绑定且支持创作的分组。分组权限直接复用 `APIKeyService.GetAvailableGroups`，包含专属授权、订阅、企业限制。所有生成请求仍由浏览器使用用户在所选分组中的 API Key 调用原有网关。

## 功能开关

`creation_enabled` 默认开启，旧部署缺少该设置时同样开启。公共设置、SSR 注入和管理端设置都包含该字段。管理端更新省略该字段或传入 `null` 时保留当前值，显式传入 `false` 时关闭，并记录设置审计。

已登录请求在关闭时收到 HTTP 404，错误 `reason` 为 `CREATION_DISABLED`；未登录请求仍由 JWT 鉴权返回 401。

## 能力来源

- 生图需要分组允许生图。OpenAI、Grok 返回 `openai` 协议，Gemini 返回 `gemini` 协议；三者均支持参考图。异步能力直接读取 `ImageTaskService.Enabled()`，只适用于 OpenAI 和 Grok，与网关对象存储开关保持一致。
- 视频只采用分组内活跃、可调度且通过网关端点能力检查的账号。Video 平台按 `video_protocol` 返回 Seedance、Gemini 或 MiniMax；OpenAI 的 Seedance 端点能力及 Grok 的媒体生成准入规则也直接复用。Grok 网关目前对图片和视频生成共用分组生图开关，目录保持同样限制。
- MiniMax 使用网关同一 `miniMaxVideoV2` 函数按账号上游 URL 的路径识别版本。v1 上游仅公布 `minimax_v1`；v2 上游公布 `minimax_v1` 与 `minimax_v2`，因为网关会把旧版请求转换成 v2。H3 映射到 v1 上游的模型不列入目录。
- 视频分辨率和时长复用 `readVideoAccountCapabilities`，只汇总有效的显式配置。分辨率取并集，时长取最小下界和最大上界。未配置时返回 `[]` 和 `null`。这些是分组包络，具体模型、账号仍可能有更严格限制。
- 模型来源为同平台渠道及分组媒体计费条目、分组 `video_model_prices`、账号公开映射名和渠道公开别名。按分组白名单和渠道模型限制收窄，去重排序，不输出通配符或账号映射目标。无法枚举时返回空数组，允许客户端手动填写。
- 视频模型协议优先根据账号映射、已知模型家族及端点能力判定；未知归属保留空协议数组。调度及上游最终决定手动模型名是否可用。
- Composite、Anthropic、Antigravity 等分组不列入目录。

## 查询与缓存

每次请求重新判断用户权限和账号可调度状态，账号通过一次 `ListSchedulableByPlatforms` 批量读取，无上游请求。渠道元数据复用 `ChannelService` 现有十分钟缓存，沿用渠道/分组更新失效及跨实例通知机制；不新增用户目录缓存。渠道元数据读取失败时退化为其他模型来源，不影响空模型目录返回；分组权限或账号读取失败则返回正常服务错误。

目录只输出共享契约的字段，不返回账号 ID、凭据、上游地址、账号数量或调度详情。
