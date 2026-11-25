# LiteRedis 项目交接说明

最后更新：2026-11-25

## 最新补充（2026-11-25，累计改动提交推送）

- 用户授权将累计改动推送 origin/main，仅推代码，不创建/推送新tag、不发布安装包。
- 最终状态：顶部Tab入口与标签栏暂停；搜索条件收藏入口隐藏、组件/数据保留；环境采用侧栏15px D/T/P标签，移除顶部环境条；保留生产保护、冲突对比、分段查看、诊断和模板功能。
- 推送前验证：135项前端单测、109项浏览器测试通过，5项已暂停收藏入口交互测试显式跳过（隐藏和数据保留测试通过）；Go全量race/vet、lint/build、diff检查通过。
- fetch发现远端已有v0.8.3指向上次提交1af9c64；双语日志补齐该版，当前新增变化归待发布（v0.8.3之后）。不擅定下一版本号。
- 系统钥匙串、原生验收/安装体验仍暂缓；工程应用仍为13:59包，不包含最后隐藏搜索收藏的调整。下方未提交等描述为历史阶段记录。


## 最新补充（2026-11-25，侧栏环境标签与暂停顶部 Tab）

- 按用户反馈移除顶部环境展示条，环境改为侧栏连接行「图标 / 状态点 / D、T、P / 名称」，15×15px圆角小标签，悬停显示完整中英文环境，未指定不显示。生产保护逻辑不变。
- 顶部连接 Tab 暂不开放：隐藏设置入口并停止标签栏渲染，即使旧配置开启也不显示；保留配置与组件便于后续优化。
- 深浅色标签尺寸/顺序、生产保护、连接图标专项浏览器回归及lint/build/diff检查通过。
- 已按要求重新打包 `build/bin/LiteRedis.app`，标识 `0.8.2-local.sidebar-env.20261125.135913`，macOS arm64，本地深度严格签名校验通过，主程序SHA256 `a9fa690726d7cdc7ed4fa7f0302d536877c078158772d3dcaffbf5b756d3c8af`。旧包备份 `/tmp/literedis-sidebar-env-build.6lUbD2/LiteRedis.app`。
- 未启动新包、未替换已安装应用、未操作真实Redis、未提交推送或打tag。下方13:40等包为历史记录。

## 最新补充（2026-11-25，P0 → P2 六项增量功能）

- 用户授权按优先级开发六项功能，不实施钥匙串及原生验收/安装体验，后续另问。计划和边界见 `FEATURE_ROADMAP.md`。
- P0：连接环境字段（未指定/开发/测试/生产），表单/导入导出/复制保留、顶部持续显示连接和 DB。生产批量删除输入 DELETE，TTL/前缀/导入复制目标/Console 非明确只读命令输入 CONFIRM；原只读/CAS/未知结果保护不变。Console 命令识别与后端引号拼接规则一致，防 GET"DEL" 等绕过。
- P0：String/Hash CAS 冲突支持主动三列对比、草稿复制；固定原 DB，只读有界快照，不提供无保护覆盖。Hash 字段 >64KiB 仅返回大小信息（不调用 Lua HGET 复制巨型字段）；缺权限/旧Redis无HSTRLEN明确失败，不做无界降级。
- P1：命名搜索收藏按连接保存 DB/pattern，最近使用排序，一键切库查询受草稿保护；本机localStorage仅记录查询条件，不存凭据。
- P1：大String可按字节跳转/64KiB分页只读查看，跨UTF8界限或二进制用Base64完整保留字节；前后长度/类型检查但不是一致快照。
- P1：连接表单实际分阶段诊断，独立socket/15s上限/按requestID取消；SSH仅用已信任指纹，不自动新增信任；代理DNS与SSH联合阶段如实展示；Cluster仅首seed认证/PING，不宣称全部拓扑健康；复制报告不包含地址/凭据/raw error。
- P2：命令台本机模板、独立占位符参数、安全引用、预览填入不自动执行；拒绝CR/LF/NUL参数，不持久化参数值；保留目标提示及原执行保护。
- Wails绑定已生成，待发布中英日志已同步；135项前端单测、111项浏览器回归、lint/build、Go race/vet、diff检查通过。早期两次运行曾因窗口消失超时失败，停止改动后全量重跑通过。
- 后续按用户要求于13:40打包工程应用 `build/bin/LiteRedis.app`，版本 `0.8.2-local.p0-p2.20261125.134020`，macOS arm64，已含六项功能。本地深度严格签名校验通过，主程序 SHA256 `4fc303fd2a868dd176bbcd40f7f2f605af08d917128b7b8cea567519537e4c00`；旧工程包在 `/tmp/literedis-p0-p2-build.LFBVAn/LiteRedis.app`。
- 打包后重新执行 `go test -race -count=1 ./...` 和 `go vet ./...` 通过；使用测试临时服务/模拟Redis，未找到本机 redis-server，未作真实Redis服务端联调或原生交互验收。未启动新包、未操作真实用户Redis、未替换已安装应用、未提交推送/tag。

## 最新补充（2026-11-25，版本新功能与双语发布说明）

- 用户要求将设置内「安装说明」入口替换为「版本新功能 → 查看新功能」，安装须知保留在安装材料与文档。已接入内置日志，按当前安装版本和界面语言查看，可重复打开；保留快捷键二级菜单。
- `frontend/src/changelog.js` 新增双语待发布日志与 v0.8.2 双语概览；dev/-local 使用待发布预览，正式版按精确版本查找，未知版不冒用最新日志。
- 推送前必须主动维护中英文 changelog，见 `AGENTS.md`。待发布内容定版时登记准确 tag 版本；Release CI 校验该 tag 中英文条目均非空，缺失阻断构建，未改变仅 tag 发布的触发方式。
- 验证：112 项前端单测、83 项 Wails mock Playwright、lint、前端生产构建、Go 测试及 diff 检查通过；发布校验的成功/缺失日志阻断两条路径已验证。原生交互未验收。
- 后续按用户要求于 10:27 重新打包 `build/bin/LiteRedis.app`，标识 `0.8.2-local.release-features.20261125.102758`，macOS arm64，已包含版本新功能入口和双语待发布说明；深度严格本地签名校验通过，主程序 SHA256 `f51a9099e8e016a991ae2fe6e2a543632ed9cdb030b36f8c16284302375608e3`。
- 旧工程包备份在 `/tmp/literedis-release-features-build.nz4WC9/LiteRedis.app`。未启动新包、未替换 `/Applications`、未操作真实配置、未提交/推送或打 tag。下方 09:55 包为历史记录，已由本次新包取代。

## 最新补充（2026-11-25，快捷键二级菜单与工程打包）

- 设置页移除平铺快捷键列表，底部改为「软件说明」，包含安装说明和快捷键指示；「查看快捷键」进入设置内二级页面，顶部/底部返回按钮或 Esc 返回主设置，保留未保存表单。
- 已通过 104 项前端单测、80 项 Wails mock Playwright、lint/build、Go 测试及 diff 检查；原生交互仍待人工验收。
- 按用户要求打包 `build/bin/LiteRedis.app`，标识 `0.8.2-local.software-guide.20261125.095532`，macOS arm64，深度严格 ad-hoc 签名验证通过。主程序 SHA256 `64cb8e2639665e6e8dc43dc75fdb485738c66d14b4b51d475946b22e44878385`。
- 旧工程包与源码 diff 备份在 `/tmp/literedis-software-guide-build.mpIo1e`。未启动新包、未替换 `/Applications`、未操作用户配置、未提交/推送或打 tag。


## 最新补充（2026-11-20，更新检查改用网页重定向）

- 正式版检查更新不再调用 GitHub API：请求 `releases/latest` 后只读取 302 的 `Location`，校验 `github.com` 主机与 `/releases/tag/<tag>` 路径，再做 semver 比较，因此不受每 IP 60 次/小时限制，也不需要 token。
- 更新包下载也不再依赖 release JSON：按发布工作流固定产物名拼接 `releases/download/<tag>/<asset>`，`SHA256SUMS` 直连下载；进度使用 `Content-Length`，完整性仍用 SHA-256 校验。
- 预览版仍走 release 列表；`-local` 工程测试包按稳定通道处理，便于验证新端点；`changelog.js` 内置说明仍优先，避免不必要的网络请求。
- 验证：新增 5 项 Go 测试覆盖跳转解析、404、稳定版无 API 路径、平台资产名和通道选择；`go test -race ./...`、`go vet ./...`、104 项前端单测、80 项 Playwright、lint/build、`git diff --check` 通过。本机访问 github.com 超时，真实端点未联网验证。尚未提交或推送。
- 已打包工程测试包 `build/bin/LiteRedis.app`（`0.7.5-local.update-redirect2.20261120.141236`，SHA256 `759bdb3f8423d75fa84cb7cee58d4227ab9cbb7d694c13986dda0b4d2036f900`），快照与旧包备份位于 `/private/var/folders/hb/xgj26dls1c91mc9fp63lfkm40000gn/T/opencode/literedis-update-redirect2-build-20261120-141236`；未启动、未替换 `/Applications`。

## 最新补充（2026-11-18，超大 String 只读预览）

- 超过 5MB 的 string 不再先拉取 5MB：后端只读取前 8001 字节，按 UTF-8 字符边界截成前 2000 个字符返回，避免打开几十 MB 的 key 时界面长时间转圈甚至卡死。
- 超大值标记为 `truncated` / `read_only_reason=truncated` 且不生成 `edit_token`；前端 textarea 只读，保存、复制、JSON 格式化和树形预览按钮均禁用，提示明确说明只读，避免复制或保存不完整内容。
- 8KB 到 5MB 之间的值仍走原有完整读取，保持可编辑；5MB 以内的普通值不受影响。
- 验证：新增 1 项后端中文超大值测试，并加强截断/二进制/大小变化预览的浏览器回归；`go test -race ./...`、`go vet ./...`、104 项前端单测、80 项 Playwright、lint/build、`git diff --check` 通过。尚未提交或推送。
- 已打包工程测试包 `build/bin/LiteRedis.app`（`0.7.5-local.large-key-readonly.20261118.152552`，SHA256 `5b2b6b7db9f9239ba7c59d3e539d2bb3086c38ae8e4fb5654e255ee8251febae`），快照与旧包备份位于 `/private/var/folders/hb/xgj26dls1c91mc9fp63lfkm40000gn/T/opencode/literedis-large-key-readonly-build-20261118-152552`；未启动、未替换 `/Applications`。

## 最新补充（2026-11-18，应用包更新代理默认地址）

- 设置页「应用包更新代理」的地址为空时默认填入 `http://127.0.0.1:1081`：首次打开、点击重置、启用代理后都直接带默认值，不需要手动输入；已经保存过的地址不会被覆盖。
- 默认值仅用于表单回填，仍由用户保存后才写入 `proxy_url`；原有 http / socks5 地址格式校验不变。
- 验证：`go test -race ./...`、`go vet ./...`、104 项前端单测、80 项 Playwright 浏览器测试（新增 2 项设置页默认值回归）、lint/build、`git diff --check` 通过。尚未提交或推送。
- 已打包工程测试包 `build/bin/LiteRedis.app`（`0.7.5-local.proxy-default.20261118.104614`，SHA256 `f787b8a9f812983edc40a378eb071b28e73c3f9ab73026524a83c5f5fd9fef6c`），快照与旧包备份位于 `/private/var/folders/hb/xgj26dls1c91mc9fp63lfkm40000gn/T/opencode/literedis-proxy-default-build-20261118-104614`；未启动、未替换 `/Applications`。

## 最新补充（2026-11-18，连接图标自定义字符）

- 连接图标内字符可自定义，最多 4 个字符；按每行 2 个显示，2 个字符一行、3-4 个字符两行。留空仍使用连接名称首字。
- 图标底图尺寸不变：侧栏 20px、折叠栏 28px、编辑弹窗预览 34px；自定义多字符自动缩小字号，中文四字两行不会溢出。
- 新增后端 `icon_text` 字段，保存和配置导入时统一清理并截断到 4 个字符；复制连接、配置导出/导入会自动带上该字段。
- 验证：`go test -race ./...`、`go vet ./...`、102 项前端单测、78 项 Playwright 浏览器测试（新增 3 项连接图标）、lint/build、`git diff --check` 通过。尚未提交或推送。
- 已打包工程测试包 `build/bin/LiteRedis.app`（`0.7.5-local.icon-text.20261118.102902`，SHA256 `8ac9d9f4bc3b47da451dd373277f44fe23a0da4c3a6d612fd4d5efa0ececc924`），快照与旧包备份位于 `/private/var/folders/hb/xgj26dls1c91mc9fp63lfkm40000gn/T/opencode/literedis-icon-text-build-20261118-102902`；未启动、未替换 `/Applications`。

## 最新补充（2026-11-17，第三方平台导入实现）

- 第三方平台导入已从入口样式预览升级为可用功能：设置页「第三方平台数据导入」支持 AnotherRedisDesktopManager `.ano`（Base64 JSON，兼容新旧 `{connections, groups}` 与旧连接数组）和 Tiny RDM `.zip`（压缩包内 `connections.yaml`）。
- 流程为选择文件 → 后端解析并在内存暂存（15 分钟、单次令牌）→ 应用内预览确认（文件、可导入数量、跳过明细、注意事项）→ 确认后只新增连接、同组重名自动编号；取消或预览失效会清理暂存，不写配置。
- 按地址去重：`host:port` 相同的连接直接跳过，并在预览中说明“不会覆盖现有连接”；同文件内重复只保留第一条。启用 SSH 时把跳板机路由一并纳入判断，同一 Redis 地址经不同跳板机不会被误判；Cluster 连接缺少单独 host 时使用首个节点地址；预览和确认时都会再次检查。
- 支持普通连接、ACL、只读、DB、分组、Cluster、SSH 账号、基础 TLS 和自定义代理；Sentinel、Unix Socket、双向 TLS、自定义 SNI、SSH Agent、系统代理明确跳过并说明原因。ARDM TLS 保持其原本的跳过证书校验；私钥和 CA 只保存路径，跨机器导入需重新指定。
- 验证：Go 全量竞态与 vet、97 项前端单测、75 项浏览器测试（新增 4 项第三方导入）、lint/build、`git diff --check` 通过。
- 已按用户要求于 17:44 打包至 `build/bin/LiteRedis.app`，构建标识 `0.7.5-local.thirdparty-import2.20261117.174429`，macOS arm64，本地 ad-hoc 签名校验通过，可执行文件 SHA256 `f1e800adbe95c4efca648e7696c222b0668e295614bd3931cbdc8447601acd7d`。源码快照与旧包备份位于 `/private/var/folders/hb/xgj26dls1c91mc9fp63lfkm40000gn/T/opencode/literedis-thirdparty-import2-build-20261117-174429`。未启动、未替换 `/Applications`、未操作用户配置、未提交或推送；真实导出文件和原生文件选择器仍待人工验收。

## 最新补充（第三方导入入口样式预览）

- 最新工程包（16:51）已按要求更新为 `0.7.5-local.import-preview2.20261117.165133`，包含已移除「平台 1」小标题的第三方导入预览入口，导入按钮仍禁用。该包现已被 17:44 的 `0.7.5-local.thirdparty-import2.20261117.174429` 取代；当时未启动、未替换 `/Applications`、未操作用户配置或提交/推送。

- 用户仅授权首版入口及样式，未授权实际导入实现。设置页在快捷键说明之后、安装说明之前新增「第三方平台数据导入」，每个平台行只显示平台名称、文件类型提示和禁用的「导入配置文件」按钮（已按反馈移除「平台 1」小标题，后续平台同样不加），明确标注仅样式预览；没有文件读取、解析或配置写入。
- 中英文/深浅色设置页 6 项浏览器回归、lint/build、diff 检查通过，开发者/API 仍默认隐藏且解锁后位于安装说明之后。本次入口未重新打包，当前工程包仍为 `0.7.5-local.ssh-compact.20261117.161449`；待用户确认样式再继续。

## 最新补充（2026-11-17，SSH 账号管理）

- 最新工程包（16:14）：`build/bin/LiteRedis.app` 已更新为 `0.7.5-local.ssh-compact.20261117.161449`，包含 SSH 选择框与 DB Index 等高的微调及之前全部功能。macOS arm64，本地签名校验通过，旧包已备份。未启动、未替换 `/Applications`、未操作真实配置、未提交/推送。下方未打包/旧包说明均为历史阶段记录。

- 16:02 打包后的最新反馈：SSH 选择框 40px 太高，已取消固定高度并与 DB Index 输入框共用尺寸样式；深浅色高度对比回归及 lint 通过。这次微调未重新打包，当前工程包仍为 16:02 版。

- 最新打包状态（16:02）：按用户要求已将界面优化及连接复制打包到 `build/bin/LiteRedis.app`，标识 `0.7.5-local.ssh-copy.20261117.160217`（macOS arm64，本地签名校验通过）。旧工程包已备份，详情见 `ACCEPTANCE.md`。未启动、未替换 `/Applications`、未修改真实配置、未提交/推送；下方“未重新打包”仅为之前阶段记录。

- 最新界面优化：返回 Redis 按钮绿色+返回箭头，SSH 下拉框 40px 高并移除下方管理按钮。主侧栏/连接管理右键均可复制连接：全部配置和分组保留、独立 ID、名称追加 copy，重名递增，成功后打开副本但不自动连接；SSH 账号仍共享。
- 最新验证为 97 项前端单测、71 项浏览器测试、lint/build、Go 竞态测试和 vet 通过。本次优化未重新打包；当前 `build/bin/LiteRedis.app` 为 15:46 的初版 SSH 包，不含界面优化及复制功能。源码仍未提交/推送。

- 新增独立 SSH 账号管理子页面、Redis 隧道账号选择与下拉框新建跳转；旧内联配置迁移、共享账号引用更新、删除保护、独立 SSH 认证测试及导入导出兼容已实现，详见 `ACCEPTANCE.md`。
- 当前前端 95 项单测、63 项浏览器测试和 Go 全量竞态测试通过；新增 10 项 SSH 浏览器回归及配置/本地临时 SSH 服务测试。Wails 绑定已重新生成。
- 本轮未使用真实用户凭据、未打包、安装、提交或推送。`build/bin/LiteRedis.app` 仍为旧工程测试包；不要将源码测试通过视为原生新功能已验收。
- 后续用户要求打包：工程包现已更新为 `0.7.5-local.ssh.20261117.154640`（macOS arm64，本地签名校验通过），旧包已备份；未安装到 `/Applications`、未启动或操作用户配置，尚未提交/推送。构建身份与待测范围见 `ACCEPTANCE.md`。
- 上一轮设置快捷键与隐藏开发者排序、Hash 编辑保存成功静默关闭已推送到 `main` 的 `060695f`，本轮完整浏览器测试确认未回归。

## 最新补充（2026-11-11）

- 安全修复、大 key 导出及 REL-001～008 可靠性改进已完成代码与自动回归，详见 `SECURITY_REVIEW.md`、`RELIABILITY_PLAN.md`。
- 当前验证：95 项前端单测、43 项浏览器测试、lint/build 通过，Go 竞态测试/vet/build 已通过；新增模糊搜索分页显示及加载动画修复，真实用户 Cluster 只读查询三页找到目标 key，修复已打包至工程 `build/bin/LiteRedis.app`，未替换 `/Applications`。真实 Redis 和部分原生实测结果及未完成门禁见 `ACCEPTANCE.md`，不能将下面历史 API 计划的完成状态理解为所有发布验收已完成。
- 正常名称 macOS 本地包已备份安装，用户连接配置未改；本次更新日志补充尚未重新打包。
- 用户授权当前全部改动提交并推送 `origin/main`，不打标签、不发布安装包。后续继续保留工作区修改，不强推或覆盖远端历史。

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
- 后端：Go `1.26.11+`（标准库安全修复基线，包含 GO-2026-6859 修复），Redis 客户端为 `go-redis/v9`。
- 前端：Vue 3、Pinia、Vite。
- 连接能力：普通 Redis、Cluster、SSH 隧道、HTTP/SOCKS Proxy、多 DB。
- 数据能力：Key 扫描和搜索，String/Hash/List/Set/ZSet 等类型查看与编辑，分页加载，内置 Redis Console。
- 发布能力：GitHub tag 作为版本号，GitHub Actions 只在推送 tag 时构建：test job 先跑 lint / vet / build / 单测及依赖安全检查，通过后出 macOS/Linux/Windows 包并发布 GitHub Releases。按用户要求关闭发布 CI 的 Playwright 冒烟测试，保留本地 `npm run e2e`。push main 不触发任何流水线。

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

## 代码风格

- ESLint（`frontend/eslint.config.js`，flat config + eslint-plugin-vue essential）与 Prettier（`frontend/.prettierrc.json`）已接入，发版流程在构建前跑 `npm run lint`，有报错就失败。
- 格式化暂不设门禁：`npm run format` / `npm run format:check` 手动使用，避免一次性重排几十个文件盖掉业务 diff；新文件建议先格式化再提交。
- 交互冒烟用 Playwright（`frontend/e2e`，`npm run e2e`）：跑 Vite dev + 注入假的 `window.go`，覆盖树窗口化滚动、键盘导航、标签拖拽/中键关闭、命令台补全、命令面板搜 key。默认用系统 Chrome，可设 `PW_CHANNEL=bundled` 使用已安装的 Playwright chromium；发布 CI 不再安装浏览器或运行这些测试。

## 已通过验证

```bash
GOCACHE=/tmp/literedis-go-cache go test ./...
GOCACHE=/tmp/literedis-go-cache go test -race ./...
GOCACHE=/tmp/literedis-go-cache go vet ./...
npm --prefix frontend run lint
npm --prefix frontend run build
node --test frontend/src/utils/*.test.js frontend/src/stores/*.test.js
npm --prefix frontend run e2e
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

## 本地调试 API（127.0.0.1）

设置 → 本地数据提取 API（需先在标题连点 5 次进开发者模式）里有三级开关：

| 开关 | 作用 |
|---|---|
| 启用本地 API 服务 | 起 `127.0.0.1:<port>` 的 HTTP 服务，默认关 |
| 允许通过 API 读取 Redis | 打开 `/api/v1/*`，直接查实例概览 / key / 只读命令，默认关 |
| 关闭 Token 校验 | 纯本机自用时可关掉 `Authorization: Bearer`，默认开启校验 |

接口（全部 POST + JSON body，响应统一 `{ok,data,error,elapsed_ms}`）：

- `POST /api/v1/connections`：已连接实例列表（其它接口的 `conn` 用它）
- `POST /api/v1/overview`：`{conn, db?}`
- `POST /api/v1/keys`：`{conn, pattern, cursor, count}`，count 上限 500
- `POST /api/v1/key`：`{conn, key, type?, cursor?, offset?, count?, zset_sort?, stream_start?}`
- `POST /api/v1/command`：`{conn, command}`，只放行只读命令
- `GET /docs`：自带的调试页面（开发者模式下可在设置里一键打开），`GET /openapi.json` 是它的 spec

约束：只监听回环地址、校验 `Host` 必须是 127.0.0.1/localhost/[::1]、拒绝跨源 `Origin`、不下发任何 CORS 头、请求体上限 64KB、并发上限 8、单请求 30s 超时。`db` 参数只用于校验：和连接当前 DB 不一致时返回 409，避免 API 偷偷切库影响界面。

## 更新内容（what's new）维护约定

版本更新后第一次打开应用会弹出「更新内容」弹窗，文案来源有两级：

1. **本地** `frontend/src/changelog.js` 的 `RELEASE_NOTES`：key 是不带 `v` 的版本号（如 `0.7.0`），发布前在文件末尾补一条即可，优先级最高，不依赖网络。
2. **在线兜底**：本地没有对应版本时，读 GitHub Release 说明；说明里只有 `Full Changelog` 时，自动改用「上一个发布 → 当前发布」的提交列表。

约定：

- 只有在明确要求时才更新 `changelog.js`，不要每轮改动都自动追加。
- **开发者功能不写进更新日志**：本地调试 API（`/api/v1`、`/docs`、调试开关等）这类只有开发者自己用的能力，一律不写进 `changelog.js`。
- 开发环境（`AppVersion=dev`）每次启动都会弹，并用 `RELEASE_NOTES` 里最新加的一条做预览，不写入"已读"标记。
- 正式版本弹过一次会写入配置里的 `last_seen_version`，同一版本不再出现。

## 工作区注意事项

- 当前改动尚未提交，包含约 27 个已修改文件和多组新增测试；这是本轮优化成果，不是待清理文件。
- `frontend/wailsjs/go/main/App.js`、`App.d.ts`、`models.ts` 已由 Wails 重新生成，包含 `RenameSetMember`、`RenameZSetMember`、三参数 `SetString`、六参数 `GetValue`、`KeyValue.next_stream_id` 和 `OperationResult.disconnected`，不得回退。
- 本轮范围明确不调整前端视觉样式；前端修改仅用于接口调用、请求守卫、分页和状态同步。
- Keychain 和 Windows Credential Manager 尚未接入。当前只完成文件权限、风险说明和日志脱敏；系统凭据存储属于后续独立迁移任务。
- SSH、Proxy、Cluster、断网和三平台安装器仍建议在真实平台做发布前手工冒烟测试，自动化测试不能完全替代真实网络环境。
