# LiteRedis

一个基于 [Wails](https://wails.io) + Vue 3 构建的轻量级 Redis 桌面客户端。

---

## 功能特性

**连接能力**

- 单机、Cluster 两种连接模式，支持多 DB 切换
- SSH 隧道跳板机（密码 / 私钥 / 私钥口令）
- 连接级 HTTP / SOCKS5 代理（与 SSH 互斥）
- TLS / SSL，可选自定义 CA 证书与跳过校验
- Redis 6+ ACL 用户名认证，兼容旧式 `AUTH <password>`
- 连接分组、拖拽排序、颜色标记、心跳保活
- **保护模式**：连接级只读开关，后端拦截所有写操作，Console 写命令需二次确认

**浏览与编辑**

- `SCAN` 游标分页，不使用 `KEYS`，大库也不阻塞
- 树形（按 `:` 分组）/ 平铺两种展示，glob 模式搜索，支持多会话保留
- String / Hash / List / Set / ZSet / Stream 全类型编辑，大集合分段加载
- 字符串 JSON 一键格式化，格式化后可一键恢复为库里的原始值；支持树形折叠查看
- 重命名、TTL 修改、原子创建（Lua 脚本保证不覆盖已有 key）

**编辑可靠性**

- String 预览最多读取 5MiB+1 字节；截断、非 UTF-8 或检测到读取变化时只读，不能把预览误存回去。
- String / 展开编辑器的 JSON 格式化只调整空白，保留大整数、指数、负零、重复属性和字符串转义原文；树形数字与复制也不经 JavaScript Number 转换。重复属性只支持文本查看，避免树形视图静默丢失属性；预览设有输入、深度和格式化大小限制。
- String 保存与 Hash 单字段编辑/新增使用 Lua 原子比较，冲突不会覆盖，也不会重建已删除的 key，保留原 TTL。需允许 EVAL 及脚本内命令；不支持或权限不足时不会降级为无保护覆盖。
- String、Hash、List/Set/ZSet 新增/编辑、展开编辑器和发布消息输入有未保存提醒；Console 未知结果保留命令并在关闭前确认。切换连接/DB/Key、关闭标签或应用前确认，保存中阻止离开，断线不主动清除草稿。草稿仅在内存中，不能防止进程崩溃或强制退出。
- 确认使用应用内异步对话框，不依赖 WebView 的 `window.confirm/alert`；默认取消，支持 Esc 和键盘焦点循环。
- 普通编辑、删除、TTL、批量操作、Console、发布消息及数据任务使用固定 DB 的独立客户端，禁用自动重试。写入超时/连接中断或 Lua 可能部分执行时显示“结果未知”，保留输入并阻止盲目重复提交。普通编辑按连接/key 拦截，重命名、批量、Console、导入/复制和消息发布按目标连接拦截（保守覆盖该连接全部 DB）；刷新读取不会自动解锁，需核实后显式确认。
- Console 的 GET/TTL 等核实命令可在拦截期间使用；每条命令使用一次性连接，不支持 MULTI/EXEC/WATCH、订阅、AUTH/HELLO、MONITOR 等会话状态命令，SELECT 由连接管理器切换并同步界面。订阅请使用发布订阅面板。Cluster 的 MOVED/ASK 不自动重放，会明确提示刷新，必要时重新连接。
- 只有 String / Hash 单字段保存具备上述 CAS 冲突保护；List/Set/ZSet、删除/TTL、批量操作和任意 Console 写入不是跨客户端冲突保护或回滚事务。Lua 运行期错误可能已执行部分命令，不能直接重试。
- String 冲突检查是内容摘要比较，不能识别相同内容的删除重建（ABA）；不是分布式锁或一致性快照。

**批量操作**

- 列表左下角勾选图标进入多选模式
- 支持整目录勾选（父目录显示半选状态）与 `Shift` 连选
- 批量删除（`UNLINK` 分片执行）、批量设置 TTL / 永久、批量改前缀
- **导出 / 导入**：导出为 JSON 或可直接 `redis-cli` 执行的命令文件，导出的 JSON 可以再导回去（已存在的 key 默认跳过）
- **跨实例复制**：把选中的 key 连同 TTL 复制到另一个已连接的实例（已存在的 key 默认跳过）
- 导入/复制前显示目标连接、DB 和限制；当前界面固定为跳过已有 key，不提供覆盖开关。
- 切换连接或 DB 会自动清空选中，避免误删

导出采用分段读取、逐 key 写文件，不在内存里保留全部 key 和整个文件。String 先检查长度，再按 256KiB 读取；集合每批约 200 项，Hash/Set 使用 SCAN 并去重。

- 单次最多 5000 个 key；导出单 key 原始数据最多 32MiB、最多 10 万项（Stream 同时计入字段）；编码后单文件最多 64MiB，与 JSON 导入文件限制一致。
- JSON 导入/跨连接复制采用更小的安全构建上限：**每 key 5MiB / 1 万项**（含 key 名；Stream 计入消息和字段，每条消息最多 1000 个字段）。超过时拒绝该 key，不截断，也不会先删目标；因此较大的导出 key 不能直接导回，应拆分数据或使用 Redis 原生工具。同实例跨 DB 复制仍使用 Redis 6.2+ 原生 COPY。
- 导入/跨连接复制先在随机、同槽临时 key 上构建完整内容并设置 TTL，最后原子 RENAME 替换目标；默认不覆盖模式在同一脚本内检查目标是否存在。构建/提交失败会尝试清理，清理失败明确返回临时 key 并尝试设置 10 分钟过期。需要允许 EVAL、目标及 `{slot-tag}:__literedis_import:*` 临时 key、构建命令、EXISTS、PEXPIRE、EXPIRE/PERSIST、RENAME 和 DEL 的权限；Hash 构建使用 HMSET 兼容旧版本。
- 导出、导入、同实例跨 DB 复制和跨连接复制统一显示任务进度、成功/跳过/失败数，支持取消及复制失败清单。全局同时一个数据任务，10 分钟超时；仅在内存保留最近 10 个任务、每任务前 100 条失败信息。
- 取消停止后续操作，**不会回滚已执行写入，也不能终止已发出的 Lua 脚本**。传输错误可能发生在写入之后，任务会标记“结果未知”并停止，不自动重试。导出超限、取消或失败不会替换目标文件，也不会留下部分导出文件。
- 临时构建在单个 Lua 脚本内完成，集合内部每命令约 200 项，但不是服务端分段任务，仍可能阻塞 Redis 并额外占用内存；RENAME 替换大目标也可能阻塞。不承诺 maxmemory 淘汰、Redis 崩溃/故障下原数据完全不受影响，临时 key 也可能因权限不足无法清理/过期。建议仅在开发环境、低负载和内存充足时使用，不作为生产备份恢复工具。
- **不是一致性快照**：分页期间的删除、类型和数量变化会尽量检测，但等长改写仍可能无法识别。需要一致备份时请暂停写入，或使用 Redis 原生备份方案。
- 大小预算限制已累积的数据，不等于进程 RSS 上限。Redis SCAN 的 COUNT 是提示值，单个字段/元素也可能很大；单次 Redis 响应、JSON 转义和 Go 容器仍有额外内存开销。

**运维与排障**

- **内存体检**：可独立选择 DB（不切换主界面），扫描大 key、无过期 key、按前缀与类型聚合内存占用，可导出 Markdown 报告；汇总展示 Redis 整体已用内存和 DB 内存占比估算
- **实时监控**：QPS / 命中率 / 连接数曲线 + 内存水位，窗口失焦自动暂停
- **慢查询**：面板内直接读 `SLOWLOG`，按耗时排序，点击可跳到对应 key
- 连接概览（版本、角色、客户端数、运行时间）
- 内置 Redis Console，带历史命令

内存体检占比采用所选 DB 的 key `MEMORY USAGE` 合计 ÷ Redis `INFO memory` 的 `used_memory`，不是 Redis 配置的最大内存或主机物理内存占比。集合大小采用默认采样，扫描不是一致性快照，Redis 整体内存还包含连接、缓冲区等开销。按 pattern 筛选、扫描截断或部分读取失败时，明确显示“已扫描 Key 占比”；缺少 INFO / MEMORY USAGE 权限或降级为序列化长度、元素数时，占比显示 `—` 而不是 0%。Cluster 仅支持 DB 0，分母为全部主节点已用内存之和，不含副本。

占比不可用时区分命令不支持/被重命名、权限不足、命令禁用及采样失败。Redis 3.0.7 等 4.0 之前版本没有 `MEMORY USAGE`；即使可以读取 `DEBUG OBJECT` 的序列化长度，也不会将其冒充实际内存占比。

**其他**

- 快捷键：`⌘R` / `Ctrl+R` 刷新当前 key；`⌘W` / `Ctrl+W` 关闭当前 key（保留连接与列表，有草稿先确认，保存中禁止关闭）；`⌘Q` / `Ctrl+Q` 或关闭窗口退出时，仅在仍有已连接 Redis 时确认，无连接则直接退出（保存中仍禁止退出）；`Esc` 关闭最上层弹窗或菜单
- 中英文双语、深浅色主题、自定义水印
- 本地数据提取 API：把当前 key/元素发布到 `127.0.0.1` 的 HTTP 接口，方便浏览器侧工具读取
- 应用内检查更新、下载、校验与自动替换重启（详见下方「应用内更新」）

---

## 可靠性验收状态

2026-11-11：94 项前端单测、37 项浏览器测试及 lint/build、Go 竞态测试/vet/build 通过。已用独立临时 Redis 7.2.6 / 3.0.7 验证类型内容与 TTL、CAS、导入与 COPY 兼容性，并验证 Redis 7 的 ACL 拒绝/清理、真实写后丢回复、三节点 Cluster 同槽及 MOVED/ASK 禁重放。

浏览器测试使用 Wails mock，**不等于原生交互验收**。2026-11-11 已备份并更新 `/Applications/LiteRedis.app` 和 `build/bin/LiteRedis.app`，本地构建标识 `0.7.0-local.20261111.152730`；实际安装版已重新打开。隔离原生实测已验证无连接直接退出、有连接确认/取消/显式退出、无草稿 Cmd+W 只关闭 key。原生草稿关闭、保存中关闭、文件对话框和任务取消仍待人工验收，尚不能据此宣布发布就绪。可复现命令、备份位置及清单见 [ACCEPTANCE.md](ACCEPTANCE.md)。代码按用户授权提交推送至 main，不打版本标签、不发布安装包，不做发行签名或公证。

## 环境要求

| 工具 | 版本 | 说明 |
|------|------|------|
| **Go** | 1.26.10+ | 后端语言（包含标准库安全修复） |
| **Node.js** | 18+ | 前端构建 |
| **npm** | 随 Node 附带 | 前端依赖管理 |
| **Wails CLI** | v2.x | 项目构建工具 |

> **macOS** 无需额外安装 C 编译器（Xcode CLT 即可）。  
> **Windows** 还需安装 GCC，见下方 [Windows 打包](#windows-打包) 章节。

---

## 快速开始

### 1. 克隆代码

```bash
git clone git@github.com:Heartfilia/LiteRedis.git
cd LiteRedis
```

### 2. 安装 Wails CLI

```bash
go install github.com/wailsapp/wails/v2/cmd/wails@latest
```

安装后确认 `wails` 命令可用：

```bash
wails version
```

### 3. 验证开发环境

```bash
wails doctor
```

所有检查项显示绿色 ✔ 后再继续。

### 4. 启动开发模式

```bash
wails dev
```

启动后会自动打开应用窗口，前端代码修改后**热重载**即时生效。  
同时会在 `http://localhost:34115` 启动一个 Web 调试页面，可在浏览器 DevTools 中直接调用 Go 方法。

---

## 首次运行说明

### macOS

首次打开从 GitHub Release 下载的 `.app` / `.dmg` 时，macOS 可能会提示“无法验证开发者”或“已阻止打开”。如果遇到这类提示，可按下面方式处理：

方法一(推荐)：
打开终端，输入下面的命令并执行即可

`xattr -dr com.apple.quarantine /Applications/LiteRedis.app`

方法二：

1. 在 Finder 中找到 `LiteRedis.app`
2. 右键应用，选择“打开”
3. 在系统弹窗中再次点击“打开”

如果仍被拦截，可前往：

`系统设置 -> 隐私与安全性`

在页面底部找到被阻止的应用提示，然后点击“仍要打开”。

### Windows

首次运行时如果出现 SmartScreen 提示，可点击“更多信息”后选择“仍要运行”。

### 本地凭据

Redis、SSH 和代理凭据保存在当前系统用户的 LiteRedis 配置目录中。macOS 和 Linux 上，`connections.json` 会自动设置为仅当前用户可读写（`0600`）；Windows 上由当前用户配置目录的系统 ACL 负责隔离。

配置内容仍是本地明文，不应放入代码仓库、共享目录或公开备份。Debug 日志不会记录 Redis/SSH 密码，带认证信息的代理 URL 也会脱敏。后续如迁移到 Keychain 或 Credential Manager，需要保留对现有配置的兼容读取。

---

## 应用内更新

设置面板底部点版本号即可检查更新；发现新版本后点「执行更新」，会自动下载、校验、替换并重启，不需要去 GitHub 手动下载。

| 环节 | 行为 |
|---|---|
| 校验 | 安装包对照 Release 里的 `SHA256SUMS` 做 SHA-256 校验，不匹配直接拒绝安装 |
| macOS | 挂载 dmg → 复制 `.app` → 进程退出后原子替换 → 自动重新打开（必须从 `.app` 运行，在 dmg 里直接跑无法替换） |
| Windows | 原地替换 `LiteRedis.exe`（允许重命名运行中的 exe），随后自动重启 |
| Linux | 解压 `tar.gz` 替换二进制后重启 |
| 失败兜底 | 任一步骤失败都会回退成「打开安装包」由你手动完成，不会把应用留在半更新状态 |

> 目前只发布 macOS Apple Silicon 包，Intel 机器会提示到发布页手动下载。

### 应用包更新代理

设置 → 应用包更新代理，**只对下面两类请求生效**，数据库连接完全不受影响：

1. 检查最新版本（访问 `api.github.com`）
2. 下载安装包与校验文件

两个容易踩的坑：

- 未认证的 GitHub API **每个 IP 每小时只有 60 次**配额。走代理时用的是代理节点出口 IP，共享节点很容易被用光；提示"IP 访问限制"时等待约 1 小时再试
- 浏览器能打开 `github.com` 不代表 API 可用——网页浏览不消耗 API 配额，两者是分开的

请求失败时错误信息会注明"本次经代理 … 访问"，方便确认代理是否真的生效。

---

## 打包发布

### macOS 打包

在项目根目录执行：

```bash
# Apple Silicon (M 系列)
wails build -platform darwin/arm64

# Intel
wails build -platform darwin/amd64

# 通用包（同时支持 Intel + Apple Silicon，体积较大）
wails build -platform darwin/universal
```

---

### Windows 打包

#### 在 Windows 本机打包

**额外前置依赖：**

| 工具 | 说明 | 下载 |
|------|------|------|
| **TDM-GCC / MinGW-w64** | Go CGo 需要 C 编译器 | [TDM-GCC](https://jmeubank.github.io/tdm-gcc/) |
| **WebView2** | Win10/11 已内置，否则运行时自动安装 | — |

安装 GCC 后将其 `bin` 目录加入系统 `PATH`，然后验证：

```bash
wails doctor
```

编译：

```bash
# 普通编译
wails build

# 生产包（隐藏控制台窗口 + 生成 NSIS 安装程序）
wails build -nsis -clean
```

产物路径：`build/bin/LiteRedis.exe`（加 `-nsis` 还会生成安装包）

> 最常见的坑：GCC 没加入 PATH，`wails doctor` 会报红，按提示修复即可。

---

#### 在 macOS（Apple Silicon）上交叉编译 Windows x64

```bash
# 安装 Windows 交叉编译工具链
brew install mingw-w64

# 交叉编译
GOOS=windows GOARCH=amd64 CGO_ENABLED=1 \
  CC=x86_64-w64-mingw32-gcc \
  wails build -platform windows/amd64 -clean
```

产物同样在 `build/bin/LiteRedis.exe`，拷贝到 Windows 机器即可运行。

> ⚠️ 交叉编译时 `-nsis`（NSIS 安装包生成）不可用，需要在 Windows 上执行才能生成安装程序。

---

## 常用构建参数

| 参数 | 说明 |
|------|------|
| `-clean` | 构建前清理缓存，保证产物干净 |
| `-upx` | 使用 UPX 压缩可执行文件体积（需单独安装 upx） |
| `-s` | 跳过前端依赖重新安装（加速二次构建） |
| `-windowsgui` | ~~Windows 专用，隐藏控制台黑窗口~~ （v2.9+ 已移除，控制台窗口默认隐藏） |
| `-nsis` | Windows 专用，生成 NSIS 安装程序 |

查看所有参数：

```bash
wails build --help
```

---

## 项目架构

### 技术栈

| 层次 | 技术 | 说明 |
|------|------|------|
| **框架** | [Wails v2](https://wails.io) | 将 Go 后端与 Web 前端打包为单一原生桌面应用 |
| **后端** | Go 1.26.10+ | 业务逻辑、Redis 连接管理、文件持久化 |
| **前端** | Vue 3 + Pinia + Vite | Composition API，无 UI 组件库，所有样式手写 |
| **Redis 客户端** | go-redis/v9 | 支持单机模式与 Cluster 模式 |
| **SSH 隧道** | golang.org/x/crypto/ssh | 通过 SSH 跳板机连接 Redis |

---

### 目录结构

```
LiteRedis/
├── main.go                  # 入口：启动 Wails，embed 前端静态资源
├── app.go                   # Wails 绑定层：所有暴露给前端的 Go 方法
├── update.go                # 版本检查、下载校验、代理与 GitHub 响应缓存
├── update_install_*.go      # 三平台自动安装（macOS dmg / Windows exe / Linux tar.gz）
├── update_install_unix.go   # 后台替换脚本（等待进程退出后原子替换并重启）
├── wails.json               # Wails 项目配置
├── go.mod / go.sum          # Go 模块依赖
│
├── backend/                 # Go 后端包
│   ├── config/              # 配置与持久化
│   │   ├── types.go         # 所有共享数据结构（ConnectionConfig、KeyValue 等）
│   │   ├── store.go         # 连接配置的读写（connections.json）
│   │   └── window.go        # 窗口位置/尺寸持久化（window.json）
│   ├── redis/               # Redis 操作
│   │   ├── client.go        # 连接管理器（连接池、DB 切换、集群、TLS、代理、SSH）
│   │   ├── keys.go          # Key 级别操作（SCAN、TTL、重命名、批量删除/TTL）
│   │   ├── value.go         # Value 读写（String/Hash/List/Set/ZSet/Stream）
│   │   ├── search.go        # Value 内容搜索（HSCAN/SSCAN/ZSCAN glob）
│   │   ├── overview.go      # INFO 解析、集群聚合、Command Console
│   │   └── memory.go        # 内存体检（大 key、前缀聚合、三级大小降级）
│   ├── localapi/            # 本地数据提取 API（127.0.0.1，带 token）
│   └── ssh/
│       └── tunnel.go        # SSH 隧道拨号器
│
├── frontend/                # Vue 3 前端
│   ├── src/
│   │   ├── main.js          # Vue 应用入口，挂载 Pinia
│   │   ├── App.vue          # 根组件，双栏布局（侧边栏 + 主内容区）
│   │   ├── api/
│   │   │   └── wails.js     # 前后端通信层（封装 window.go.main.App.*）
│   │   ├── stores/
│   │   │   ├── connections.js  # 连接状态管理
│   │   │   ├── workspace.js    # 工作区状态（当前连接、Key 树、搜索会话）
│   │   │   └── settings.js     # 应用设置
│   │   ├── components/
│   │   │   ├── layout/      # 整体布局（Sidebar、MainContent）
│   │   │   ├── keys/        # Key 列表、树形结构、多选与批量操作
│   │   │   ├── editor/      # 各数据类型的 Value 编辑器
│   │   │   ├── connections/ # 连接管理、连接概览、内存体检、实时监控
│   │   │   ├── common/      # 提取菜单、浮动提示等通用组件
│   │   │   └── settings/    # 设置弹窗
│   │   └── utils/
│   │       ├── keyTree.js   # Key 路径前缀树构建（按 : 分隔）
│   │       ├── typeColors.js # 各数据类型的颜色方案
│   │       ├── format.js    # 字节数/数量格式化
│   │       └── clipboard.js # 剪贴板工具（兼容 Wails WebView）
│   ├── wailsjs/             # Wails 自动生成的 JS/TS 绑定
│   └── dist/                # 前端构建产物（编译时 embed 进 Go 二进制）
│
├── build/                   # 平台打包资源
│   ├── darwin/              # macOS Info.plist
│   ├── windows/             # 图标、manifest、NSIS 安装脚本
│   └── appicon.png          # 应用图标源文件
│
└── .github/workflows/
    └── release.yml          # CI/CD：tag 触发，自动构建三平台并发布 Release
```

---

### 架构分层

```
┌─────────────────────────────────────────────────┐
│                    前端（Vue 3）                  │
│                                                   │
│  components/  ←→  stores (Pinia)  ←→  api/wails.js │
└───────────────────────┬─────────────────────────┘
                        │  Wails 桥接层
                        │  window.go.main.App.*
┌───────────────────────┴─────────────────────────┐
│                  app.go（绑定层）                  │
│         连接管理 / Key 操作 / Value CRUD / 设置    │
└──────┬──────────────┬──────────────┬────────────┘
       │              │              │
  backend/config  backend/redis  backend/ssh
  （持久化）       （Redis 操作）  （SSH 隧道）
       │              │
  connections.json  go-redis/v9  ←→  Redis Server
  window.json
```

---

### 数据持久化

运行时数据存储在系统用户配置目录：

| 平台 | 路径 |
|------|------|
| macOS | `~/Library/Application Support/LiteRedis/` |
| Windows | `%APPDATA%\LiteRedis\` |

| 文件 | 内容 |
|------|------|
| `connections.json` | 所有连接配置 + 应用设置，`sync.RWMutex` 保护 |
| `window.json` | 窗口位置与尺寸，退出时保存，下次启动还原 |

---

### 前后端通信

Wails 在运行时将所有 `App` 的公开方法注入到 `window.go.main.App.*`，前端通过 `src/api/wails.js` 统一封装调用，所有调用均返回 `Promise`。

**示例：点击 Key 到渲染 Value 的完整链路**

```
用户点击 Key
  → workspaceStore.selectKey()
    → wails.js: getValue(connID, key)
      → window.go.main.App.GetValue(connID, key)   // Wails 桥接
        → app.go: GetValue()
          → backend/redis/value.go: GetValue()
            → Redis Server（go-redis）
          ← config.KeyValue{}
      ← Promise<KeyValue>
    ← 存入 workspaceStore.keyValue
  → KeyEditor 按类型渲染对应编辑器组件
```

### 构建须知
```bash
本软件没有做安全相关的处理
如：记录连接信息的配置缓存文件，里面的密码信息为明文，如果有特殊要求，可以二次开发处理。
（避免本人忘记）
```
