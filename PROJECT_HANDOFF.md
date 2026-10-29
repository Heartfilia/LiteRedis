# LiteRedis 项目交接说明

最后更新：2026-07-15

## 新会话先做什么

1. 阅读本文件和 `API_OPTIMIZATION_PLAN.md`。
2. 执行 `git status --short`，保留当前全部未提交修改，不要 reset、checkout 或回退生成文件。
3. 当前 `API-001` 至 `API-020` 已全部完成，不要重复实施。
4. 后续需求应作为新任务评估，不再沿用本轮已完成计划。

可给新对话的直接指令：

```text
请先读取 PROJECT_HANDOFF.md 和 API_OPTIMIZATION_PLAN.md。保留当前工作区所有修改；API-001 至 API-020 已全部完成，不要重复实施。
```

## 当前项目是什么

LiteRedis 是一个跨平台 Redis 桌面客户端，使用 Wails 将 Go 后端能力绑定到 Vue 前端。

- 桌面框架：Wails `v2.12.0`。
- 后端：Go `1.22+`，Redis 客户端为 `go-redis/v9`。
- 前端：Vue 3、Pinia、Vite。
- 连接能力：普通 Redis、Cluster、SSH 隧道、HTTP/SOCKS Proxy、多 DB。
- 数据能力：Key 扫描和搜索，String/Hash/List/Set/ZSet 等类型查看与编辑，分页加载，内置 Redis Console。
- 发布能力：GitHub tag 作为版本号，GitHub Actions 构建 macOS/Linux/Windows，并通过 GitHub Releases 检查更新。

关键调用链：

```text
Vue 组件
  -> frontend/src/stores/*
  -> frontend/src/api/wails.js
  -> Wails 生成绑定 frontend/wailsjs/go/main/*
  -> app.go / update.go 的 App 公共方法
  -> backend/redis、backend/config、backend/ssh
  -> Redis / 文件系统 / GitHub Releases
```

关键入口：

- `main.go`：Wails 应用启动和窗口配置。
- `app.go`：前后端公开 API 汇总层。
- `update.go`：版本检查、下载和更新包校验。
- `backend/redis/`：连接生命周期、Key、Value、搜索、概览和 Console。
- `backend/config/`：连接、设置、窗口状态及 Debug 日志持久化。
- `backend/ssh/`：SSH 隧道和 Cluster 转发。
- `frontend/src/api/wails.js`：前端调用后端的统一包装层。
- `frontend/src/stores/`：连接状态、心跳、DB/Key 上下文和异步响应状态管理。
- `.github/workflows/release.yml`：按 tag 构建三平台产物并发布校验清单。

## 本轮目标和开发依据

本轮目标是审查并优化前端调用后端接口的正确性、并发安全、分页性能、错误传播和状态同步，不重新设计前端界面。

处理依据按以下优先级执行：

1. 用户明确需求和 `API_OPTIMIZATION_PLAN.md` 中的验收标准。
2. 现有 Wails、Vue/Pinia、Go 分层和命名习惯，保持现有功能兼容。
3. 数据安全优先：避免删除失败后误更新 UI、跨连接/DB 串数据、非原子写入和配置损坏。
4. 大数据 Key 使用游标或分段分页，避免一次性读取全集。
5. 网络 I/O 不长期持有全局锁；异步响应写入前必须校验连接、DB、Key 和请求序号。
6. 错误必须区分业务失败与连接失败；连接状态沿用连续两次网络失败后标记异常的规则。
7. 改动范围与测试风险匹配；并发、分页、原子操作、版本和安全逻辑必须有自动化测试。
8. 不处理无关重构，不回退工作区中来源不明或用户已有的修改。

标准开发流程：

1. 开始任务前将计划状态从 `[ ]` 改为 `[-]`。
2. 沿“组件 -> Store -> API 包装 -> App 绑定 -> 后端模块”读取完整调用链。
3. 先补能复现风险的测试，再做最小范围实现。
4. App 公共方法或结构体签名变化后，通过 Wails 构建重新生成绑定，并保留生成文件。
5. 执行对应后端、竞态、前端和生产构建验证。
6. 检查 `git diff` 与 `git diff --check`，确认无意外样式改动和无关文件 churn。
7. 只有验收和验证都通过后才将计划状态改为 `[x]`，并记录日期、修改摘要和命令结果。

## 当前进度

- `API-001` 至 `API-020` 已完成。
- `API-020` 的短期代码、文档和最终 Wails macOS 打包验证均已完成。
- 当前总进度为 `20/20` 完成，没有遗留半成品或未完成验证。
- 计划完成后又补充了 Stream 续页、应用内更新入口、下载超时与清理、架构匹配、默认值一致性及对应测试；发布流水线测试按用户要求未改。

本轮主要结果包括：

- 前端请求绑定连接、DB、Key 和序号，旧响应不会污染新上下文。
- Set/ZSet 重命名、CreateKey、String 保留 TTL 保存改为原子操作。
- Cluster SCAN、List/ZSet 搜索支持可靠分页。
- Redis Manager 的网络 I/O 移出全局锁，心跳不再重叠。
- 配置损坏会保留原文件并创建备份，配置权限收紧为 `0600`。
- 只读接口不再用伪造零值掩盖错误，连接失败计数统一。
- SemVer prerelease、更新包 SHA-256、IPv6 地址和 Console 空参数已修复。

详细逐项记录和涉及位置以 `API_OPTIMIZATION_PLAN.md` 为准。

## 已通过验证

```bash
GOCACHE=/tmp/literedis-go-cache go test ./...
GOCACHE=/tmp/literedis-go-cache go test -race ./...
GOCACHE=/tmp/literedis-go-cache go vet ./...
npm --prefix frontend run build
node --test frontend/src/utils/*.test.js frontend/src/stores/*.test.js
GOCACHE=/tmp/literedis-go-cache wails build -platform darwin/arm64 -clean -v 2 -nocolour
codesign --verify --deep --strict --verbose=2 build/bin/LiteRedis.app
git diff --check
```

上述命令均已通过。Go 测试覆盖配置损坏、文件权限、代理脱敏、Redis Lua 原子操作、Cluster/Stream 分页、搜索分页、连接生命周期、Console 解析、SemVer、checksum、更新下载超时与清理、资源架构选择和 IPv6；前端 Node 测试共 8 项。Wails `v2.12.0` 已完成 Darwin arm64 绑定生成、前端生产构建、Go 编译、应用打包和自签名；产物约 16 MB，主程序为 Mach-O 64-bit arm64，深度严格签名校验通过。

## 本轮完成状态

最终打包已于 2026-07-15 在可读取 macOS `sysctl` 的环境完成：

```bash
GOCACHE=/tmp/literedis-go-cache wails build \
  -platform darwin/arm64 \
  -clean \
  -v 2 \
  -nocolour
```

- 命令退出码为 `0`，生成 `build/bin/LiteRedis.app`。
- 应用包约 16 MB，主程序为 Mach-O 64-bit arm64。
- `codesign --verify --deep --strict --verbose=2` 校验通过。
- 本轮计划已全部完成，不再有“唯一未完成事项”。

## 更新内容（what's new）维护约定

版本更新后第一次打开应用会弹出「更新内容」弹窗，文案来源有两级：

1. **本地** `frontend/src/changelog.js` 的 `RELEASE_NOTES`：key 是不带 `v` 的版本号（如 `0.7.0`），发布前在文件末尾补一条即可，优先级最高，不依赖网络。
2. **在线兜底**：本地没有对应版本时，读 GitHub Release 说明；说明里只有 `Full Changelog` 时，自动改用「上一个发布 → 当前发布」的提交列表。

约定：

- 只有在明确要求时才更新 `changelog.js`，不要每轮改动都自动追加。
- 开发环境（`AppVersion=dev`）每次启动都会弹，并用 `RELEASE_NOTES` 里最新加的一条做预览，不写入"已读"标记。
- 正式版本弹过一次会写入配置里的 `last_seen_version`，同一版本不再出现。

## 工作区注意事项

- 当前改动尚未提交，包含约 27 个已修改文件和多组新增测试；这是本轮优化成果，不是待清理文件。
- `frontend/wailsjs/go/main/App.js`、`App.d.ts`、`models.ts` 已由 Wails 重新生成，包含 `RenameSetMember`、`RenameZSetMember`、三参数 `SetString`、六参数 `GetValue`、`KeyValue.next_stream_id` 和 `OperationResult.disconnected`，不得回退。
- 本轮范围明确不调整前端视觉样式；前端修改仅用于接口调用、请求守卫、分页和状态同步。
- Keychain 和 Windows Credential Manager 尚未接入。当前只完成文件权限、风险说明和日志脱敏；系统凭据存储属于后续独立迁移任务。
- SSH、Proxy、Cluster、断网和三平台安装器仍建议在真实平台做发布前手工冒烟测试，自动化测试不能完全替代真实网络环境。
