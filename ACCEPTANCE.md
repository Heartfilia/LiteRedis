# 可靠性验收记录（2026-11-11）

本轮以验收和修复为主，不改集群概览入口、不扩功能。用户于 2026-11-11 授权将当前全部改动提交并推送到 `origin/main`，不打版本标签、不发布安装包。测试包不是发行包，原生人工验收尚未全部完成。

## 已通过与未覆盖

| 层次 | 结果与范围 |
| --- | --- |
| 前端 | 95 项单测；43 项 Playwright 浏览器测试（23 项可靠性专项、6 项 key 分页及加载样式回归）；lint / build |
| Go | `go test -race ./...`、`go vet ./...`、`go build` |
| Redis 7.2.6 / 3.0.7 | String/Hash/List/Set/ZSet 导入、默认跳过和显式覆盖、完整内容与 TTL 读回；String/Hash CAS、普通集合编辑、批量 TTL/删除；7 支持 COPY，3 明确不支持；Stream 仅在 7 测试 |
| Redis 7.2.6 ACL | EVAL / 临时 key 权限拒绝、脚本内部 HMSET/PEXPIRE/PERSIST/RENAME 拒绝保留目标值和 TTL；DEL 清理失败返回临时 key 且保留过期时间 |
| 真实网络 | TCP 代理在 RPUSH 执行后丢弃回复；结果未知、固定 DB 4、实际只写入一次，不自动重放 |
| 三节点 Cluster | 临时构建 key 同槽；缓存拓扑变化时 ASK/MOVED 返回重定向错误，不跟随写入 |
| macOS arm64 | Wails 原生包构建成功；隔离配置启动、WebView 渲染已观察。交互未通过完整验收 |

真实验收为明确列出的路径，不代表穷举所有故障。没有执行真实 OOM/maxmemory 淘汰、崩溃恢复或完整原生任务取消验收；不承诺 Lua 回滚、取消已发脚本或一致性快照。跨连接复制共用的 staged 写入已真实验证，完整 UI 跨连接复制流程仍需人工读回。

本轮真实 ACL 暴露 `ERR ACL failure in script` 格式，已保守归类 `partial_possible`。macOS WebView 未实现浏览器 confirm/alert 回调，因此改为应用内异步确认；默认取消、Esc、焦点循环、重复请求和保存中关闭已有浏览器回归。最新退出策略：有已连接 Redis 才弹确认（无草稿也确认，有草稿合并一次提示）；无连接直接退出，不提醒断线草稿，但保存中仍阻止。Cmd+W 仅关闭 key 并保护草稿；macOS File / Close Key 使用原生快捷键桥接。**所有浏览器测试使用 Wails mock，不等同原生窗口或文件对话框测试。**

## 可复现命令

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
