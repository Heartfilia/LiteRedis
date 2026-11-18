# 可靠性验收记录（2026-11-11）

本轮以验收和修复为主，不改集群概览入口、不扩功能。用户于 2026-11-11 授权将当前全部改动提交并推送到 `origin/main`，不打版本标签、不发布安装包。测试包不是发行包，原生人工验收尚未全部完成。

## 已通过与未覆盖

| 层次 | 结果与范围 |
| --- | --- |
| 前端 | 104 项单测；80 项 Playwright 浏览器测试（含 12 项 SSH 账号管理、6 项连接复制、8 项设置页、4 项第三方平台导入、3 项连接图标、27 项可靠性、6 项分页/加载回归）；lint / build |
| Go | `go test -race ./...`、`go vet ./...`、`go build` |
| Redis 7.2.6 / 3.0.7 | String/Hash/List/Set/ZSet 导入、默认跳过和显式覆盖、完整内容与 TTL 读回；String/Hash CAS、普通集合编辑、批量 TTL/删除；7 支持 COPY，3 明确不支持；Stream 仅在 7 测试 |
| Redis 7.2.6 ACL | EVAL / 临时 key 权限拒绝、脚本内部 HMSET/PEXPIRE/PERSIST/RENAME 拒绝保留目标值和 TTL；DEL 清理失败返回临时 key 且保留过期时间 |
| 真实网络 | TCP 代理在 RPUSH 执行后丢弃回复；结果未知、固定 DB 4、实际只写入一次，不自动重放 |
| 三节点 Cluster | 临时构建 key 同槽；缓存拓扑变化时 ASK/MOVED 返回重定向错误，不跟随写入 |
| macOS arm64 | Wails 原生包构建成功；隔离配置启动、WebView 渲染已观察。交互未通过完整验收 |

真实验收为明确列出的路径，不代表穷举所有故障。没有执行真实 OOM/maxmemory 淘汰、崩溃恢复或完整原生任务取消验收；不承诺 Lua 回滚、取消已发脚本或一致性快照。跨连接复制共用的 staged 写入已真实验证，完整 UI 跨连接复制流程仍需人工读回。

本轮真实 ACL 暴露 `ERR ACL failure in script` 格式，已保守归类 `partial_possible`。macOS WebView 未实现浏览器 confirm/alert 回调，因此改为应用内异步确认；默认取消、Esc、焦点循环、重复请求和保存中关闭已有浏览器回归。最新退出策略：有已连接 Redis 才弹确认（无草稿也确认，有草稿合并一次提示）；无连接直接退出，不提醒断线草稿，但保存中仍阻止。Cmd+W 仅关闭 key 并保护草稿；macOS File / Close Key 使用原生快捷键桥接。**所有浏览器测试使用 Wails mock，不等同原生窗口或文件对话框测试。**

## 可复现命令

### 超大 String 只读预览（2026-11-18）

- 后端读取 string 时先在同一 pipeline 里取 TYPE / TTL / STRLEN 和前 8001 字节：超过 5MB 的值只返回前 2000 个字符，不再传输 5MB 内容；8KB 到 5MB 之间的值再补一次有上限的读取，保持原有可编辑行为。
- 2000 字符按 UTF-8 字符边界截取，中文等多字节值不会截出半个字符；超大值的 `truncated` 和 `read_only_reason=truncated` 保持不变，且不生成 `edit_token`，后端保存接口会拒绝截断预览。
- 前端超大值 textarea 只读，保存、复制、JSON 格式化和树形预览按钮均禁用；提示文案改为「仅展示前 2000 个字符，只读预览」。
- 验证：新增 1 项后端测试（中文超大值保留完整字符），并加强现有截断/二进制/变更预览的浏览器回归；`go test -race ./...`、`go vet ./...`、104 项前端单测、80 项 Playwright、lint/build、`git diff --check` 通过。
- 已打包工程测试包到 `build/bin/LiteRedis.app`，构建标识 `0.7.5-local.large-key-readonly.20261118.152552`，macOS arm64，本地 ad-hoc 签名校验通过，可执行文件 SHA256 `5b2b6b7db9f9239ba7c59d3e539d2bb3086c38ae8e4fb5654e255ee8251febae`。快照与旧包备份位于 `/private/var/folders/hb/xgj26dls1c91mc9fp63lfkm40000gn/T/opencode/literedis-large-key-readonly-build-20261118-152552`。未替换或启动 `/Applications` 中的应用，未提交或推送。

### 应用包更新代理默认地址（2026-11-18）

- 设置页「应用包更新代理」在地址为空时默认填入 `http://127.0.0.1:1081`：首次打开、点击重置、以及启用代理后输入框里都直接有默认值，不需要手动输入一次；已经保存过的地址保持不变。
- 默认值只影响表单回填，不改变后端配置读取逻辑；用户启用代理并保存后才写入 `proxy_url`，原有 http / socks5 地址格式校验不变。
- 验证：新增 2 项前端单测和 2 项浏览器回归（首次默认值并保存、已有地址不被覆盖）；`go test -race ./...`、`go vet ./...`、104 项前端单测、80 项 Playwright、lint/build、`git diff --check` 通过。
- 已打包工程测试包到 `build/bin/LiteRedis.app`，构建标识 `0.7.5-local.proxy-default.20261118.104614`，macOS arm64，本地 ad-hoc 签名校验通过，可执行文件 SHA256 `f787b8a9f812983edc40a378eb071b28e73c3f9ab73026524a83c5f5fd9fef6c`。快照与旧包备份位于 `/private/var/folders/hb/xgj26dls1c91mc9fp63lfkm40000gn/T/opencode/literedis-proxy-default-build-20261118-104614`。未替换或启动 `/Applications` 中的应用，未提交或推送。

### 连接图标自定义字符（2026-11-18）

- 连接配置新增 `icon_text`：最多 4 个字符，后端保存和配置导入都会清理换行/制表符并按字符（rune）截断，前端输入同样即时限制；留空时仍显示名称首字。
- 图标底图尺寸不变（侧栏 20px、折叠栏 28px、编辑弹窗预览 34px）。自定义多字符按每行 2 个拆行，2 个字符一行、3-4 个字符两行；字号随字符数缩小，中文四字两行已通过边界回归确认不超出底图。
- 复制连接会沿用图标字符；配置导出/导入和第三方平台导入可正常保留或留空该字段。
- 验证：新增 5 项前端单测和 3 项浏览器回归（自定义/中文两行显示、折叠栏显示、编辑弹窗保存）；`go test -race ./...`、`go vet ./...`、102 项前端单测、78 项 Playwright、lint/build、`git diff --check` 通过。
- 已打包工程测试包到 `build/bin/LiteRedis.app`，构建标识 `0.7.5-local.icon-text.20261118.102902`，macOS arm64，本地 ad-hoc 签名校验通过，可执行文件 SHA256 `8ac9d9f4bc3b47da451dd373277f44fe23a0da4c3a6d612fd4d5efa0ececc924`。快照与旧包备份位于 `/private/var/folders/hb/xgj26dls1c91mc9fp63lfkm40000gn/T/opencode/literedis-icon-text-build-20261118-102902`。未替换或启动 `/Applications` 中的应用，未提交或推送。

### 第三方平台导入实现（2026-11-17）

- 按用户要求于 17:44 打包至 `build/bin/LiteRedis.app`，构建标识 `0.7.5-local.thirdparty-import2.20261117.174429`，macOS arm64，本地 ad-hoc 签名校验通过。可执行文件 SHA256：`f1e800adbe95c4efca648e7696c222b0668e295614bd3931cbdc8447601acd7d`。源码快照、构建身份和旧包备份位于 `/private/var/folders/hb/xgj26dls1c91mc9fp63lfkm40000gn/T/opencode/literedis-thirdparty-import2-build-20261117-174429`。未启动应用、未替换 `/Applications`、未操作真实用户配置、未提交或推送。

- 新增后端解析模块 `backend/config/thirdparty_import.go`：ARDM `.ano` 支持 UTF-8 Base64 与已解码 JSON、旧连接数组和新 `{connections, groups}`；Tiny RDM `.zip` 读取根目录 `connections.yaml`，限制解压后 YAML 不超过 8MiB，避免异常压缩包。两者统一转换为 `ConnectionConfig`，不读取或写入 Redis。
- 导入采用“预览 → 确认 → 单次令牌应用”：后端在内存暂存解析结果 15 分钟，令牌只能用一次；密码不会回传前端。确认后通过现有 `ImportConfiguration` 只新增连接，同组重名自动加序号，不覆盖现有记录；内联 SSH 配置由 `ensureSSHProfiles` 整理为独立账号，结果会统计新增 SSH 账号数。
- 按地址去重：以 `host:port` 生成端点键，与现有连接地址相同的一律跳过并在预览中给出原因（含现有连接名），绝不覆盖；同一次文件内地址重复只保留第一条。启用 SSH 时端点键同时包含跳板机用户/主机/端口，同一 Redis 地址经不同跳板机会保留为不同连接；Cluster 连接没有单独 host 时用首个节点地址参与判断。预览阶段和应用阶段都会再检查一次，避免预览后新增连接造成重复。
- 映射范围：名称、地址/端口、ACL、密码、只读、DB、分组、Cluster 初始节点、SSH（密码或私钥）、TLS 开关/CA/跳过校验、自定义代理、图标颜色。明确跳过并提示：Sentinel、Unix Socket、双向 TLS（客户端证书/私钥）、自定义 SNI、SSH Agent、系统代理。ARDM TLS 的事实行为是 `rejectUnauthorized: false`，导入保持跳过证书校验并提示；ARDM 的 DB 编号保留但提示它表示上次使用的库。
- 前端在设置页保留 ARDM 和 Tiny RDM 两行入口，不显示编号小标题；按钮改为可用，接入应用内确认弹窗、取消清理、成功/失败提示和连接列表刷新。新增中英文文案，移除“功能暂未开放”说明。
- 验证：`go test -race ./...`、`go vet ./...`、`go build ./...`、97 项前端单测、75 项 Playwright 浏览器测试（新增 4 项第三方导入）、lint/build、`git diff --check` 全部通过。新增 13 项 Go 配置测试，覆盖两种格式、Base64/旧数组、字段映射、跳过原因、重名与不覆盖、内联 SSH 账号统计，以及地址去重（现有地址跳过、不同端口保留、同文件重复只留第一条、不同 SSH 跳板机保留、Cluster 回退节点地址、原因包含“不会覆盖现有连接”）。
- 未覆盖：没有使用真实用户的 `.ano` / `.zip` 导出文件，也没有在原生窗口验证系统文件选择器；浏览器测试使用 Wails mock。工程测试包已更新为上述 17:44 版本，但原生导入交互仍待人工验收；未提交或推送。

### 第三方平台导入入口预览（2026-11-17）

- 后续按用户要求移除「平台 1」小标题并再次打包：`build/bin/LiteRedis.app`（16:51），标识 `0.7.5-local.import-preview2.20261117.165133`，macOS arm64，本地 ad-hoc 签名校验通过。SHA256：`b24c0550aad8fb6e3d50de6ce9f706e6670f45b5404bc1cbf2ae7f4a6ee6513a`。快照与旧包备份位于 `/private/var/folders/hb/xgj26dls1c91mc9fp63lfkm40000gn/T/opencode/literedis-import-preview2-build-20261117-165133`。未启动、未替换 `/Applications`、未操作用户配置、未提交或推送。

- 仅增加设置页入口样式，在安装说明上方显示第三方平台导入分区及 AnotherRedisDesktopManager 平台行；平台行只显示名称和文件类型，不再显示「平台 1」小标题，导入按钮禁用并明确提示尚未开放。没有接入原有导入接口，也没有文件读取或配置写入。
- 已通过 6 项设置浏览器回归（中英文、深浅色、分区顺序、无水平溢出、按钮禁用、安装说明和开发者隐藏保护）、lint/build 和 diff 检查。Vite 单包体积提示仍为非阻断警告。本次未打包、提交或推送，等待用户确认样式。

### SSH 账号管理补充（2026-11-17）

- 最新打包（16:14）：按用户要求将与 DB Index 等高的 SSH 选择框调整打包到 `build/bin/LiteRedis.app`，构建标识 `0.7.5-local.ssh-compact.20261117.161449`，macOS arm64，本地 ad-hoc 签名校验通过。可执行文件 SHA256：`597495ee1ea4e953657c27fdb820205a27312d378758e88411bfdfc3ab29b437`。源码快照、身份记录和旧包备份位于 `/private/var/folders/hb/xgj26dls1c91mc9fp63lfkm40000gn/T/opencode/literedis-ssh-compact-build-20261117-161449`。未启动、未替换 `/Applications`、未操作用户配置、未提交或推送。下面“高度微调尚未重新打包”为之前阶段记录。

- 后续用户反馈 40px 仍偏高：SSH 账号选择框取消固定高度，复用 DB Index 输入框的字号、上下内边距和边框样式。深浅色浏览器回归直接比较两者实际高度，2 项通过，lint 和 diff 检查通过。本次高度微调尚未重新打包；16:02 包仍使用 40px。

- 再次按用户要求于 2026-11-17 16:02 打包最新界面优化及连接复制：`build/bin/LiteRedis.app`，构建标识 `0.7.5-local.ssh-copy.20261117.160217`，macOS arm64，本地 ad-hoc 签名校验通过。可执行文件 SHA256：`03b28609f82355cdacfe0a638b8f3ffb844dab136c7041ac141c0954573223b4`。源码快照、构建身份及旧工程包备份位于 `/private/var/folders/hb/xgj26dls1c91mc9fp63lfkm40000gn/T/opencode/literedis-ssh-copy-build-20261117-160217`（旧包在 `backup-project/LiteRedis.app`）。未启动应用、未替换 `/Applications`、未操作真实连接配置、未提交或推送；原生交互仍待人工验收。

- 后续界面优化与连接复制：SSH 页返回 Redis 的按钮改为独立绿色并加返回箭头，深浅色适配；SSH 选择框统一为 40px 高并保留键盘焦点态，移除下方重复管理按钮，顶部切换和下拉新建继续保留。
- 主侧栏及连接管理列表右键均支持「复制连接」。深复制全部配置，清空 ID、时间和排序，由后端创建独立记录；保留分组，名称追加 `copy`，同组忽略大小写/首尾空格重名时递增 `copy 2`、`copy 3`。成功后打开副本供修改，不自动连接；SSH 仍引用原共享账号。
- 此次增加 2 项单测、8 项浏览器回归（深浅色样式、两处入口、完整配置、副本修改不影响原记录、编号、失败、重复提交、英文/代理）。最新 97 项单测、71 项浏览器测试、lint/build、Go 全量竞态测试及 vet 通过，`git diff --check` 通过。本次优化尚未重新打包，工程 App 仍为下述 15:46 初版 SSH 包；未提交或推送。

- 连接管理默认展示 Redis；右上角新建按钮左侧切换到 SSH 账号管理，左侧账号列表、右侧配置表单。Redis 启用 SSH 后选择账号，下拉框底部可跳转新建，保存后返回并选中，Redis 未保存表单保留。
- 支持 SSH 账号创建、编辑、删除、独立认证测试及主机指纹重置确认。切换子页面保留 SSH 草稿，切换账号/关闭管理器前确认丢弃，测试或保存中阻止重复操作。
- 旧内联 SSH 配置在读取时无损整理为稳定账号关联，不在只读加载时改写文件；下一次显式保存时落盘。完全相同的凭据复用账号，密码不同的账号不合并，密码和口令首尾空格保留。配置仍为本地明文，沿用 0600 文件保护，未新增浏览器凭据持久化。
- 共享账号修改后更新所有引用的配置快照，对活跃 SSH 连接复用原传输层失效机制并提示重连；被引用账号禁止直接删除。禁用 Redis SSH 并保存后取消关联。
- 导出包含独立 SSH 账号（含未使用账号）及兼容旧版本的内联快照；导入按凭据匹配，冲突名称加后缀，不覆盖无关本地共享账号。新增导入 SSH 账号数量提示。
- 新增 5 项 Go 配置测试覆盖迁移稳定性、去重、引用、更新、删除、校验、损坏文件保护、旧导入和跨设备导出/导入；另用本地临时 SSH 服务验证认证成功后连接关闭，不打开 SSH channel 或远程命令。
- 新增 10 项 Wails mock 浏览器测试覆盖深浅色、英文、独立测试、下拉新建往返、草稿、引用删除、测试中操作保护、共享连接失效和旧 Redis 配置回显。前端 95 项单测、63 项浏览器测试、lint/build，以及 Go 全量竞态测试、vet/build 通过；Wails 绑定已重新生成。Vite 仍有单包超过 500kB 的非阻断提示。
- 本轮未访问用户真实 SSH/Redis，未运行应用安装包的新功能联动验收，未替换 `/Applications`、打包、提交或推送。Playwright 仍仅本地运行，CI 冒烟测试未恢复。
- 后续按用户要求于 2026-11-17 15:46 完成 macOS arm64 工程测试包：`build/bin/LiteRedis.app`，构建标识 `0.7.5-local.ssh.20261117.154640`，本地 ad-hoc 签名校验通过（不是发行签名或公证）。可执行文件 SHA256：`83c5458e3b91e7bbf919bf0561d0015852d47c6e5bbf6d483157b6b8203a4399`。构建快照、身份记录及旧工程包备份位于临时目录 `literedis-ssh-build-20261117-154640`。未启动新包、未修改用户连接配置、未替换 `/Applications`，原生交互待用户测试。

前端（在 `frontend/` 中分别执行）：

```sh
npm test
npm run lint
npm run e2e
npm run build
```

先完成前端 build，再在工程根目录执行，避免 Go embed 与 dist 重建并行：

```sh
go test -race ./...
go vet ./...
go build -o /path/to/temp/LiteRedis-build-check .
LITEREDIS_TEST_REDIS7=/path/to/redis-7.2.6/src/redis-server \
LITEREDIS_TEST_REDIS3=/path/to/redis-3.0.7/src/redis-server \
go test -race -count=1 -run '^TestAcceptance' -v .
```

`LITEREDIS_TEST_REDIS7/3` 指向二进制，不是现有 Redis 地址；不设置则跳过对应验收。测试启动独立 loopback 端口、临时目录和无持久化实例，结束回收进程；不访问用户连接，不对外部 Redis 执行清库。创建端口与启动间仍有竞争，若被占用须重跑。

### 本机 Redis 源码来源

Docker daemon 未运行，改用官方仓库 tag 源码（下载域名返回 403）：

- `https://codeload.github.com/redis/redis/tar.gz/refs/tags/7.2.6`
  - SHA256：`86c8a1c315ce95df7771bcc37e6b3a5f41f4e23eb0d83cb0044609ac8f9d3875`
- `https://codeload.github.com/redis/redis/tar.gz/refs/tags/3.0.7`
  - SHA256：`546da8db6ff2d4d84a89db7939a4cf1af351fc6fe182e8d529bc0d86bdbc02e7`

7.2.6 在解压目录执行 `make -C src -j4 MALLOC=libc BUILD_TLS=no redis-server`。3.0.7 为适配现代 macOS/ARM，仅修改临时源码 `src/config.h` 的 HAVE_BACKTRACE 条件，将 Apple 分支限制为 `defined(__APPLE__) && !defined(__aarch64__)`，再执行：

```sh
make -C src -j4 MALLOC=libc CFLAGS=-DMAC_OS_X_VERSION_10_6=1060 redis-server
```

宏用于旧代码 fstat 兼容，关闭 ARM 不兼容 backtrace；没有修改 LiteRedis 的 Redis 业务逻辑，也不是生产 Redis 安装建议。

## 测试包与隔离启动

- 路径：`build/bin/LiteRedisAcceptance.app`（darwin/arm64，单独的应用标识）。
- 独立测试包不安装到 `/Applications`；正常名称包现已另外构建并安装，见下节。
- 从临时源码快照将 `wails.json` 的 name / outputfilename 改为 `LiteRedisAcceptance`，info 标为 `0.0.0` 本地验收包，再执行 `wails build -m -nosyncgomod -skipbindings -platform darwin/arm64`。工程配置和版本号不变。
- Wails 自动 ad-hoc self-sign；没有发行签名、公证或上架。
- 最新紧凑确认框测试包构建于 14:23，退出卡片宽度 360px，仅有已连接 Redis 时提示；可执行文件 SHA256：`91c154e8282649b5a3b05b411b0544dfe6c55a30e4e93361ef852f535bb3d653`。上版美化包保留在临时目录 `LiteRedisAcceptance-before-compact-dialog.app`，更早包保留在 `LiteRedisAcceptance-before-dialog-polish.app` / `LiteRedisAcceptance-before-shortcuts.app`。
- 浏览器深浅色预览已检查，截图保留在临时材料根目录 `quit-light.png` / `quit-dark-draft.png`；原生浅色退出框也已检查，深色及草稿状态仍待人工。
- 默认直接双击仍会使用正常 HOME 下的 LiteRedis 配置。**验收请使用隔离 HOME 启动，不要直接双击访问真实连接。**

本机临时材料根目录：

```text
/private/var/folders/hb/xgj26dls1c91mc9fp63lfkm40000gn/T/opencode/literedis-acceptance-20261111
```

保留 `app-source`、`native-home`、Redis 源码/二进制和原生截图；本轮临时手工实例已停止，测试自动创建的进程也已回收，隔离配置中的旧端口不可直接沿用。可用新的 loopback 测试 Redis，并只修改该隔离 HOME 中的连接：

```sh
open -n "$PWD/build/bin/LiteRedisAcceptance.app" --env HOME=/path/to/isolated-home
```

本机直接执行二进制曾很快退出，原因未确定；通过 LaunchServices `open -n ... --env HOME=...` 启动可稳定渲染。

## 正常名称本地安装与原生实测（2026-11-11 15:49）

- 已获授权备份并替换 `/Applications/LiteRedis.app` 与工程 `build/bin/LiteRedis.app`，两份可执行文件 SHA256 相同：`4466ac7047ac941070ac7ddd56a5aa57c9616d850571cb75dee095f45ebd6826`，均通过 `codesign --verify --deep --strict`。这是本地 ad-hoc 签名，不是发行签名/公证。
- 从完整源码快照执行 `wails build -m -nosyncgomod -skipbindings -platform darwin/arm64 -ldflags '-X main.AppVersion=0.7.0-local.20261111.152730'`；源码版本配置未改，不以 bundle plist 的默认版本判别此次构建。
- 旧应用经正常退出后才替换，未强杀。替换前后用户 `connections.json` 哈希一致；未修改连接配置，未连接或读写用户真实 Redis。
- 已按明确路径重新打开 `/Applications/LiteRedis.app`，核实实际进程路径、正常渲染与已保存连接列表（均未连接），不是临时包或旧版进程。
- 安装材料目录：`/private/var/folders/hb/xgj26dls1c91mc9fp63lfkm40000gn/T/opencode/literedis-install-20261111-152730`。旧安装包在 `backup-installed/LiteRedis.app`，旧工程包在 `backup-project/LiteRedis.app`；另保留替换时移出的原包、`installation.json`、隔离 HOME、日志和截图。
- 原生实测仅使用隔离 HOME 和临时 `127.0.0.1:54166` Redis；测试完成后正常退出隔离应用并停止该 Redis。

| 原生实测项 | 结果 / 证据 |
| --- | --- |
| 无连接 Quit 菜单、Cmd+Q、窗口红色关闭按钮 | 均直接退出，无需确认；对应 PID 15098 / 17154 / 17067 已退出 |
| 有连接 Cmd+Q | 紧凑浅色确认框正常；Esc 取消后仍可操作连接，`native-quit-confirm.png` |
| 有连接窗口关闭按钮 | 弹确认、不直接退出，`native-window-close-confirm.png` |
| 默认取消及显式退出 | 确认框默认 Enter 取消；再次请求，Tab 切换接受按钮再 Enter 才退出，PID 15239 已退出 |
| Cmd+W 关闭已打开、无草稿的 key | 只关闭 key，连接及工作区仍在，`native-draft-open.png` / `native-key-closed.png` |

这只是部分原生验收通过，不代表下面所有门禁完成。WebView DOM 未可靠暴露，模拟鼠标和部分键盘输入效果不稳定，草稿编辑、无 key 再次 Cmd+W、文件面板和任务取消没有可靠证据，不勾为通过。

## Key 模糊搜索分页修复（2026-11-11）

- 按用户授权仅连接本地已保存的 `kgj` 配置，通过原有 SSH 和 Cluster 路径只读查询；隔离 HOME 复用已信任 SSH 指纹，不修改连接配置，不读取 value、不写入 Redis。
- `EXISTS hermes:thor:aweme_ios_2280_did` 返回 1。应用后端 `ScanKeys` 以 `hermes:thor:aweme_ios*` / 每页 200 读取，实测三页 200 / 200 / 195，共 595 个不重复 key，目标在第三页；此次数据状态下后端无漏页或错误。这不是静态快照，后续数量可能变化。
- 前端加载更多会卸载已显示列表，虚拟列表保留旧滚动偏移且绑定新容器时未同步，是可复现的显示缺陷。现改为追加加载保留原列表和滚动位置，虚拟窗口限制在有效范围内，解绑旧容器监听并同步新容器滚动位置。
- 单搜索及保留多搜索模式均展示分页错误，失败保留已有结果，重试清除旧错误；追加结果按 key 名去重，空后续页不清空列表。
- 新增 4 项浏览器回归（修复前可复现列表被卸载），以及 1 项虚拟窗口边界单测；当前 95 项单测、41 项浏览器测试、lint/build 通过。完整浏览器回归曾发现新增 count watcher 过早触发 List 编辑器未初始化的 computed，已移除该 watcher 并重新全量通过。
- 真实 Redis 后端只读测试与浏览器 Wails mock 测试分别执行；尚未将本次前端修复安装到正常运行的 macOS 应用，不宣称原生联动已验收。发布 CI 仍按用户要求不运行 Playwright，本地测试保留。
- 已于 18:34 将当前修复打包至工程 `build/bin/LiteRedis.app`（darwin/arm64，Go 1.26.11），构建标识 `0.7.3-local.pagination.20261111.183432`，通过本地 ad-hoc 签名校验。可执行文件 SHA256：`4344747b91127c9cc3d28b9488c69083175eeed47920a60fbc51b614e59b8311`。旧工程包保留在临时材料 `literedis-pagination-build-20261111-183432/backup-project/LiteRedis.app`；未替换 `/Applications/LiteRedis.app`、未结束用户正在运行的应用，待用户手动退出旧版并打开工程新包测试。

### 加载动画修复补充（2026-11-12）

- 原 `.loading, .loading::before` 同时给提示容器与伪元素添加尺寸、边框及旋转，并误伤加载更多按钮的 `.loading` 状态；现使用独立 `.key-scan-loading` 容器，仅其伪元素旋转，文字居中且保持静止。
- 新增深浅色首次加载样式回归，以及加载更多按钮不整体旋转/不产生额外圆圈的断言；43 项浏览器测试、lint/build 通过，原生视觉待用户复测。
- 工程包 `build/bin/LiteRedis.app` 已更新，构建标识 `0.7.3-local.loading.20261112.091340`，可执行文件 SHA256 `99ae54d67645537a38469729a9cd906e0f63c8f5b1a498752f2c1e400670266d`，本地签名校验通过。上一版分页测试包保留在临时材料 `literedis-loading-build-20261112-091340/backup-project/LiteRedis.app`；未替换 `/Applications`、未结束正在运行的旧版。

## 发布前人工门禁（未完成，不能勾为通过）

仅连接临时 Redis，并记录截图、实际数据读回和结果：

- [ ] 修改 String / Hash 草稿后，用红色关闭按钮、Cmd+Q、关闭标签分别验证默认取消、Esc 保留草稿，以及明确接受后退出/切换。
- [ ] 有已连接 Redis 时无草稿 Cmd+Q 仍弹确认，有草稿只需确认一次；无连接（含仅保存配置、全部断线）关闭按钮和 Cmd+Q 均直接退出；后台仍有连接不能跳过确认；保存中即使断线也阻止退出。
- [ ] Cmd+W 只关闭当前 key，无 key 时不退出，取消关闭保留草稿，保存中阻止关闭；File / Close Key 与键盘行为一致。
- [ ] 延迟保存响应期间关闭窗口：必须阻止退出；保存失败后草稿仍在。
- [ ] 写后丢回复：显示未知结果，草稿保留、重复保存被拦截；核实实际值后才能显式解锁。
- [ ] 原生导出保存对话框：取消不写文件；成功文件可导回；失败/取消不替换已有目标文件。
- [ ] 原生导入打开对话框：取消无任务/写入；确认显示目标连接、DB、跳过模式和限制；已有 key 内容/TTL 不变。
- [ ] 多 key 导入/复制中取消：停止后续操作，面板计数与失败清单一致；已完成写入不回滚，读回验证。
- [ ] 跨连接复制与同实例跨 DB：确认目标 DB，验证所有类型内容/TTL，已有目标跳过，旧 Redis COPY 提示清晰。
- [ ] 深/浅色对话框、键盘焦点、Console 浮层上的确认无误遮挡。

原生辅助功能目前只可靠暴露窗口/菜单，未可靠暴露 WebView DOM。已通过项详见上表；上述复合门禁仍包含未验证场景，故保持未勾选。当前结论是“新版已本地安装，自动回归、核心真实 Redis 场景及部分原生关闭交互通过，其余桌面交互待验”，不是“可直接发布”。
