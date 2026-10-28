# LiteRedis API 优化计划

最后更新：2026-07-15

当前进度：`20/20` 已完成。`API-020` 的短期加固及最终 Wails macOS 打包验证均已完成；计划完成后的低优先级优化也已记录。

新会话继续处理前，请先阅读同目录的 `PROJECT_HANDOFF.md`。该文件记录项目架构、开发流程、已完成改动、工作区状态和下一步命令。

## 目标与范围

- 检查前端调用后端接口时的请求上下文、错误处理、状态同步和并发行为。
- 修复可能导致数据丢失、跨连接串数据、分页遗漏、连接状态错误或配置损坏的问题。
- 优化大数据量场景下的接口性能，避免一次性读取完整集合。
- 本轮不调整前端视觉样式；必要时只修改前端请求和状态管理逻辑。
- 保持现有功能兼容，避免无关重构。

## 状态规则

- `[ ]` 待处理
- `[-]` 处理中
- `[x]` 已完成
- 开始处理时先将对应任务改为 `[-]`。
- 完成后改为 `[x]`，并填写完成日期、修改摘要和验证结果。
- 如果任务范围发生变化，在原任务下面追加说明，不删除原始问题记录。

## 当前验证基线

- `GOCACHE=/tmp/literedis-go-cache go test ./...`：通过。
- `GOCACHE=/tmp/literedis-go-cache go test -race ./...`：通过。
- `GOCACHE=/tmp/literedis-go-cache go vet ./...`：通过。
- `npm --prefix frontend run build`：通过。
- `node --test frontend/src/utils/*.test.js frontend/src/stores/*.test.js`：8 项通过。
- `git diff --check`：通过。
- 带生产标签的底层 Go 编译在补齐 Wails 所需的 `UniformTypeIdentifiers` 链接参数后通过。
- `GOCACHE=/tmp/literedis-go-cache wails build -platform darwin/arm64 -clean -v 2 -nocolour`：通过，Wails `v2.12.0` 完成绑定生成、前端生产构建、Go 编译、应用打包和自签名。
- `build/bin/LiteRedis.app`：约 16 MB；主程序为 Mach-O 64-bit arm64；`codesign --verify --deep --strict --verbose=2` 校验通过。

## P0：优先处理

### API-001 删除 Key 后必须校验后端结果

- [x] 状态：已完成
- 风险：前端当前忽略 `DeleteKey` 返回的 `OperationResult.success`，删除失败时仍可能从界面移除 Key，造成界面状态与 Redis 实际数据不一致。
- 涉及位置：`frontend/src/stores/workspace.js:682`
- 处理方案：校验后端结果；仅在成功后更新本地列表和当前编辑状态；失败时保留数据并走统一错误处理。
- 验收标准：模拟删除失败时，Key 仍保留在列表中，右侧内容不被错误清空，并显示基础错误信息；删除成功行为保持不变。
- 完成记录：2026-07-15。前端仅在 `DeleteKey.success` 为真时移除本地 Key；业务失败通过全局错误提示反馈，连接错误继续进入断线处理。验证：`npm --prefix frontend run build` 通过，`git diff --check` 通过。

### API-002 为异步请求增加完整上下文标识

- [x] 状态：已完成
- 风险：连接、DB 或 Key 切换后，旧请求可能晚于新请求返回，并把旧结果写入当前界面；同名 Key 在不同连接中可能发生结果串写。
- 涉及位置：`frontend/src/stores/workspace.js:517`、`frontend/src/stores/workspace.js:643`，以及 Hash、Set、List、ZSet 编辑器的异步加载请求。
- 处理方案：为请求绑定连接 ID、DB、Key、请求序号；响应写入前校验全部上下文；切换连接、DB、Key 或关闭 Key 时使旧请求失效。
- 验收标准：快速切换连接、DB 和同名 Key 时不会闪回旧数据；旧分页和搜索请求不会污染新会话。
- 完成记录：2026-07-15。Store 请求令牌绑定连接 ID、DB、Key、作用域和序号；连接、DB、Key 或搜索会话切换后旧响应不再写入。Hash、Set、List、ZSet 编辑器增加组件级请求守卫和选择序号保护。验证：`node --test frontend/src/utils/requestGuard.test.js` 3 项通过，`npm --prefix frontend run build` 通过，`git diff --check` 通过。

### API-003 原子化 Set/ZSet 成员重命名

- [x] 状态：已完成
- 风险：当前先执行 `SREM/ZREM`，再执行 `SADD/ZADD`；第二步失败时原成员已经丢失。
- 涉及位置：`frontend/src/components/editor/SetEditor.vue:443`、`frontend/src/components/editor/ZSetEditor.vue:414`，并新增对应后端接口。
- 处理方案：使用后端 Lua 脚本或事务封装成员重命名，校验源成员、目标成员和执行结果，保证操作整体成功或整体不变。
- 验收标准：目标冲突、连接中断和第二步异常时不会丢失原成员；成功时分值及数据保持正确。
- 完成记录：2026-07-15。新增 Redis Lua 原子接口 `RenameSetMember` 和 `RenameZSetMember`；前端不再执行先删后加，ZSet 在脚本内读取并保留当前 score。验证：4 个 miniredis 后端测试通过，`GOCACHE=/tmp/literedis-go-cache go test ./...` 通过，前端生产构建通过。

### API-004 修复 Cluster Key 分页游标

- [x] 状态：已完成
- 风险：Cluster 扫描每次从各节点游标 `0` 开始，再对临时结果切片；大集群中会重复、遗漏 Key，且无法可靠继续分页。
- 涉及位置：`backend/redis/keys.go:91`
- 处理方案：为每个主节点保存独立游标和完成状态；分页令牌携带节点扫描上下文；去重后按页返回，并在全部节点完成后结束。
- 验收标准：多节点、跨多页扫描无重复和遗漏；连接关闭或拓扑变化时旧分页令牌安全失效。
- 完成记录：2026-07-15。Cluster SCAN 改为一次性后端续页令牌，保存主节点顺序、节点 cursor、待消费批次与去重状态；令牌支持过期、容量限制、拓扑校验和失败重试恢复。验证：跨节点分页、批次剩余数据和令牌单次消费 3 项测试通过，`go test ./...` 通过。

### API-005 SelectDB 时恢复 Proxy Dialer

- [x] 状态：已完成
- 风险：重新选择 DB 时只恢复 SSH Dialer，没有恢复 HTTP/SOCKS Proxy；代理连接在切换或恢复 DB 后可能绕过代理或直接失败。
- 涉及位置：`backend/redis/client.go:245`
- 处理方案：统一客户端创建配置，确保普通连接、SSH、HTTP Proxy、SOCKS Proxy 在 Connect、Reconnect 和 SelectDB 路径使用同一套 Dialer 构建逻辑。
- 验收标准：各类代理连接切换 DB 后仍通过原代理访问；断网重连后配置不丢失。
- 完成记录：2026-07-15。SelectDB 通过统一的活动连接 Options 构建逻辑恢复 SSH 转发或 HTTP/SOCKS Proxy，保持原地址、认证和 DB 配置。验证：Proxy 恢复、SSH 优先级、非法 Proxy 3 项测试通过，`go test ./...` 通过。

### API-006 配置 JSON 损坏时禁止静默覆盖为空配置

- [x] 状态：已完成
- 风险：配置文件解析失败时被当作空配置，后续保存可能覆盖原文件，导致连接配置永久丢失。
- 涉及位置：`backend/config/store.go:32`
- 处理方案：解析失败时返回明确错误；保留原文件，并创建带时间戳的损坏文件备份；未确认恢复前禁止以空配置覆盖。
- 验收标准：使用损坏 JSON 启动时可得到明确错误，原文件和备份均存在，不会自动写入空连接列表。
- 完成记录：2026-07-15。JSON 解析失败时按修改时间和内容哈希创建只读备份，原配置不修改；读取错误透传至前端并通过已有全局提示展示，所有保存路径被阻止。验证：损坏备份与禁止覆盖 2 项测试通过，`go test ./...` 和前端构建通过。

### API-007 为大 List/ZSet 搜索提供真正分页

- [x] 状态：已完成
- 风险：List 搜索使用 `LRANGE 0 -1`，大 Key 会阻塞并占用大量内存；ZSet 搜索可能丢弃一次扫描批次中未返回的剩余成员，且缺少可靠续页游标。
- 涉及位置：`backend/redis/search.go:137`、`backend/redis/search.go:164`
- 处理方案：List 使用分段范围扫描并返回续页状态；ZSet 保存扫描游标及批次偏移，确保单批未消费结果可继续返回；增加取消和过期机制。
- 验收标准：大 List/ZSet 搜索不会一次拉取全部数据；连续加载结果无重复、无遗漏；切换搜索条件后旧游标失效。
- 完成记录：2026-07-15。List 搜索使用分块 LRANGE 和精确索引 cursor，ZSet 搜索返回完整 ZSCAN 批次并透传 cursor；List/ZSet 前端搜索结果支持续页，搜索续页与新搜索共享请求作用域。验证：List/ZSet 分页 2 项测试、请求守卫测试、`go test ./...` 和前端构建通过。

## P1：一致性与稳定性

### API-008 统一只读接口的错误返回

- [x] 状态：已完成
- 风险：部分后端接口失败时返回合法零值，例如 `DBSize` 失败返回 `0`，前端无法区分“数据库为空”和“请求失败”。
- 涉及位置：`app.go:227`、`frontend/src/stores/workspace.js:363`，并审查其他只读接口。
- 处理方案：统一使用 `(value, error)` 或公共结果结构；前端分别处理成功零值、连接错误和业务错误。
- 验收标准：断网时 DB 数量不会被错误刷新为 `0`；错误能进入统一连接状态和提示流程。
- 完成记录：2026-07-15。DBSize、配置读取、集合计数和连接概览不再吞掉错误或返回伪造零值；前端按连接保存 key 总数快照，刷新失败保留最后成功值并进入连接错误流程。验证：`go test ./...`、前端构建和 `git diff --check` 通过。

### API-009 编辑连接配置后重建或失效活动客户端

- [x] 状态：已完成
- 风险：修改主机、端口、认证、SSH 或代理配置后，界面配置已更新，但活动 Redis 客户端仍使用旧连接参数。
- 涉及位置：`frontend/src/components/ConnectionForm.vue:445`、`frontend/src/components/ConnectionManager.vue:397`，以及后端连接管理接口。
- 处理方案：保存配置后比较连接关键字段；如已变化，显式关闭旧客户端并要求重连，或由后端原子重建客户端。
- 验收标准：修改连接参数后不会继续访问旧地址；失败时连接状态和编辑结果保持一致。
- 完成记录：2026-07-15。保存连接后比较传输配置；Host、认证、DB、Cluster、Proxy 或 SSH 变化会关闭并移除旧客户端，元数据变化只热更新配置。前端依据 `disconnected` 立即清理连接状态。验证：传输/元数据分支测试、`go test ./...` 和前端构建通过。

### API-010 避免网络 I/O 长时间占用全局 Manager 锁

- [x] 状态：已完成
- 风险：Connect 和 SelectDB 在 SSH 建连、Dial、Ping 等网络操作期间持有全局互斥锁，一个慢连接可能阻塞所有其他连接操作。
- 涉及位置：`backend/redis/client.go:47`、`backend/redis/client.go:245`
- 处理方案：锁内只读取或提交共享状态；客户端构建和 Ping 在锁外执行；提交时再次校验连接版本，防止并发覆盖。
- 验收标准：一个连接超时期间，其他连接仍可刷新、断开或重连；并发操作通过竞态检查。
- 完成记录：2026-07-15。Manager 增加每连接操作版本；Connect/SelectDB 在锁外建连和 Ping，提交时校验版本，Disconnect/配置变更可使迟到操作失效。资源关闭移至锁外，新连接失败保留旧客户端。验证：连接替换测试、`go test ./...`、前端构建和 `go test -race ./backend/redis` 通过。

### API-011 防止心跳任务重叠

- [x] 状态：已完成
- 风险：异步 `setInterval` 没有进行中保护；网络慢时下一轮心跳会在上一轮未结束前启动，造成重复请求和状态抖动。
- 涉及位置：`frontend/src/stores/connections.js:187`
- 处理方案：改为单次完成后再调度下一次，或增加 in-flight guard；停止心跳时取消后续调度和旧响应。
- 验收标准：慢网络下同一连接最多只有一个心跳请求；停止或关闭连接后不会被旧心跳重新标绿。
- 完成记录：2026-07-15。异步 setInterval 改为单轮完成后 setTimeout 调度，加入 generation 和 in-flight 防护；停止、隐藏或手动断开后旧响应不再更新状态。验证：串行调度单测、请求守卫单测和前端构建通过。

### API-012 String 保存时正确保留 TTL

- [x] 状态：已完成
- 风险：保存 String 时可能重新应用已经过时的剩余 TTL，或意外把有 TTL 的 Key 变成永久 Key。
- 涉及位置：`frontend/src/components/editor/StringEditor.vue:99`、`backend/redis/value.go:164`
- 处理方案：优先使用 Redis `KEEPTTL`，或增加后端“更新值并保留当前 TTL”的原子接口；明确处理 Key 在保存期间过期的情况。
- 验收标准：编辑保存前后 TTL 连续递减，不会回跳或变成永久；已过期 Key 不会被意外重建。
- 完成记录：2026-07-15。String 保存改为单 Key Lua 原子更新，读取 PTTL 后写值并恢复 TTL；永久 Key 保持永久，缺失 Key 返回错误且不重建。String 编辑器增加请求上下文守卫。验证：TTL 保留/缺失 Key 2 项测试、`go test ./...`、前端单测和构建通过。

### API-013 正确比较预发布版本

- [x] 状态：已完成
- 风险：当前版本比较可能把 `0.4.3-beta.1` 与 `0.4.3-beta.2` 视为相同；GitHub `/releases/latest` 不包含预发布版本。
- 涉及位置：`update.go:42`
- 处理方案：使用符合 SemVer 的比较逻辑；根据当前版本通道决定是否查询 prerelease，并避免 beta 用户错误升级或降级。
- 验收标准：稳定版和 beta 版本按 SemVer 正确排序；不同发布通道行为有明确测试覆盖。
- 完成记录：2026-07-15。版本比较改用 `x/mod/semver`；稳定版查询 GitHub latest，prerelease 当前版本查询 releases 列表并按 SemVer 选最高非 draft 版本。验证：SemVer 比较、发布通道和 prerelease 识别 3 组测试通过，`go test ./...` 通过。

### API-014 修复 SSH 的 IPv6 地址拼接

- [x] 状态：已完成
- 风险：手工使用 `host:port` 拼接会生成无效 IPv6 地址，也是当前 `go vet` 的唯一已知失败项。
- 涉及位置：`backend/ssh/tunnel.go:59`
- 处理方案：统一改用 `net.JoinHostPort`，同时审查 Redis、SSH 和代理地址的其他拼接位置。
- 验收标准：IPv4、主机名和 IPv6 地址均可正确连接；`go vet ./...` 通过。
- 完成记录：2026-07-15。Redis 与 SSH 地址统一使用 `net.JoinHostPort`，支持 IPv4、主机名和 IPv6。验证：Redis/SSH IPv6 地址测试、`go test ./...` 和 `GOCACHE=/tmp/literedis-go-cache go vet ./...` 通过。

## P2：补充风险项

### API-015 移动连接分组时保留 Cluster 扫描配置

- [x] 状态：已完成
- 风险：上下文菜单移动分组时可能遗漏 `allow_cluster_scan`，导致已开启的“允许加载 Key 及模糊搜索”被重置。
- 涉及位置：`frontend/src/components/ConnectionManager.vue:339`
- 处理方案：避免手工重建不完整配置对象，改为复制完整连接配置后只修改分组字段。
- 验收标准：连接跨组移动前后所有连接选项完全一致。
- 完成记录：2026-07-15。连接管理的分组移动改为复制完整连接配置后仅覆盖 group，并深复制 Cluster 地址和 SSH 配置，避免当前及未来字段丢失。验证：前端生产构建和 `git diff --check` 通过。

### API-016 消除创建和重命名操作的部分成功状态

- [x] 状态：已完成
- 风险：Set/ZSet 创建或重命名、`CreateKey` 后应用 TTL 等多步操作中，后一步失败会留下半完成数据。
- 涉及位置：后端 Key 创建接口及 Set/ZSet 写接口，前端对应调用链。
- 处理方案：可原子化的操作使用 Lua 或事务；不可原子化时返回分步结果，并提供明确补偿逻辑。
- 验收标准：故意让第二步失败时数据状态可预测，不会静默留下错误 Key 或丢失源数据。
- 完成记录：2026-07-15。CreateKey 改为单 Key Lua 原子执行存在检查、类型初始化和 TTL；目标存在时原数据与 TTL 不变。结合已完成的 Set/ZSet 原子重命名，消除主要多步写入风险。验证：创建/冲突 2 项测试和 `go test ./...` 通过。

### API-017 Console 命令解析保留空字符串参数

- [x] 状态：已完成
- 风险：当前解析器可能无法保留 `SET key ""` 这类空引号参数，导致执行语义与 redis-cli 不一致。
- 涉及位置：内置 Redis Console 的前端解析与后端执行接口。
- 处理方案：使用明确的命令词法解析规则，保留空参数、转义字符和单双引号边界。
- 验收标准：空字符串、带空格字符串和转义引号命令均与 redis-cli 行为一致。
- 完成记录：2026-07-15。Console 词法解析增加 tokenStarted 状态，保留单双引号空参数，并继续支持引号拼接、转义空格和未闭合引号错误。验证：6 组解析用例、未闭合错误和真实空字符串 SET 测试通过，`go test ./...` 通过。

### API-018 扩大连接状态错误覆盖范围

- [x] 状态：已完成
- 风险：连接概览、List/ZSet 加载更多等接口失败后，可能只显示局部错误，没有及时把连接状态标记为异常。
- 涉及位置：连接概览及各编辑器分页调用链、连接状态 Store。
- 处理方案：建立统一的 Redis 网络错误识别与上报入口；业务错误不误判断线，网络错误触发连续失败计数和状态更新。
- 验收标准：相关接口断网失败后连接状态按既定“两次失败”规则变更；恢复连接后错误面板正确消失。
- 完成记录：2026-07-15。Connections Store 统一网络错误识别、成功重置和失败阈值；心跳、连接概览、Console 与各编辑器更新同一连接状态，业务错误不计数。验证：阈值/业务错误单测、心跳单测和前端构建通过。

### API-019 为更新包增加完整性校验

- [x] 状态：已完成
- 风险：更新包当前缺少 checksum 或签名验证，下载损坏或发布资源被替换时无法检测。
- 涉及位置：`update.go` 及 GitHub Release 构建流程。
- 处理方案：发布时生成 SHA-256 校验文件；客户端下载后校验资源名称、大小和哈希，再允许安装。
- 验收标准：正确包校验通过；篡改或不完整包被拒绝，并给出明确错误。
- 完成记录：2026-07-15。Release 工作流生成并上传 SHA256SUMS；客户端要求校验清单存在，并在打开安装器前校验资源大小和 SHA-256，失败时清理临时目录。验证：正确哈希、错误哈希、错误大小及清单异常测试通过，`go test ./...` 通过。

### API-020 降低本地明文凭据风险

- [x] 状态：已完成
- 风险：Redis、SSH 或代理凭据保存在本地配置 JSON 中，其他本机进程或备份可能读取。
- 涉及位置：配置模型、配置存储和各平台打包说明。
- 处理方案：先确认现有文件权限；记录风险和迁移兼容方案；后续评估 macOS Keychain、Windows Credential Manager 等系统凭据存储。
- 验收标准：短期确保配置文件权限合理并有文档说明；长期方案不破坏旧配置迁移。
- 完成记录：2026-07-15。macOS/Linux 的 `connections.json` 新建和加载时强制为 `0600`；损坏配置备份同样使用 `0600`；代理 URL 和代理解析错误中的认证信息会脱敏；README 已明确本地明文风险及后续系统凭据存储迁移要求。权限和脱敏测试、`go test ./...`、`go test -race ./...`、`go vet ./...`、前端构建及前端单测均通过。最终使用 Wails `v2.12.0` 完成 `darwin/arm64` 清理构建，生成约 16 MB 的 `build/bin/LiteRedis.app`；主程序架构为 arm64，深度严格签名校验通过。Keychain/Credential Manager 属于后续独立迁移，不在本轮短期加固范围内。

## 当前交接状态

- `API-001` 至 `API-020` 已全部完成并通过对应验证。
- 最终 Wails macOS arm64 打包及产物核验已完成，本计划没有未完成事项。
- 当前工作区包含本轮全部未提交修改及新增测试，不得清理、回退或覆盖用户原有修改。
- Wails 已重新生成 `frontend/wailsjs/go/main/App.js`、`App.d.ts` 和 `models.ts`，这些生成结果包含新接口签名，必须保留。

## 计划完成后的低优先级优化

- 2026-07-15：Stream 使用排他 ID 游标分段加载，支持继续查看旧记录；连接、DB 或 Key 切换后旧页响应不会写入，边界记录会去重。
- 2026-07-15：设置页接通 `StartUpdate`；下载增加 15 分钟总超时，成功打开安装器后延迟清理临时目录，应用启动时清理超过 24 小时的遗留目录。
- 2026-07-15：更新资源选择增加操作系统和 CPU 架构匹配，避免 Intel/arm64 等平台误选安装包。
- 2026-07-15：所有加载数量的非法值统一回退到文档默认值 `20`；前端同时进行数值归一化。
- 2026-07-15：新增 Stream 合页去重、旧响应保护、加载数量、下载超时、临时目录清理和资源架构选择测试。
- 按用户要求未修改发布流水线测试步骤；前端未新增或调整 CSS。
- 验证：`go test ./...`、`go test -race ./...`、`go vet ./...`、8 项前端 Node 测试和 Wails Darwin arm64 清理构建均通过；最终 `.app` 架构、签名结构和 `Info.plist` 校验通过。

## 执行与验证要求

每完成一个任务，至少执行与改动范围匹配的验证：

1. 后端改动执行 `GOCACHE=/tmp/literedis-go-cache go test ./...` 和 `go vet ./...`。
2. 前端接口交互改动执行 `npm --prefix frontend run build`。
3. 并发、分页、原子操作和版本比较必须补充自动化测试，不只依赖手工验证。
4. SSH、Proxy、Cluster 和断网恢复类改动记录实际验证环境；无法实测的平台必须明确标注剩余风险。
5. 完成记录应包含日期、修改文件、验证命令和结果；失败项不得标记为 `[x]`。

## 推荐执行顺序

`API-001` -> `API-002` -> `API-003` -> `API-004` -> `API-005` -> `API-006` -> `API-007` -> P1 -> P2

其中 `API-002` 是后续异步分页和状态同步的基础；`API-005` 与 `API-009`、`API-010` 可在统一客户端生命周期时连续处理，但应分别验收和标记。
