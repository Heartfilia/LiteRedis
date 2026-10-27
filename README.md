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

**批量操作**

- 列表左下角勾选图标进入多选模式
- 支持整目录勾选（父目录显示半选状态）与 `Shift` 连选
- 批量删除（`UNLINK` 分片执行）、批量设置 TTL / 永久、批量改前缀
- **导出 / 导入**：导出为 JSON 或可直接 `redis-cli` 执行的命令文件，导出的 JSON 可以再导回去（已存在的 key 默认跳过）
- **跨实例复制**：把选中的 key 连同 TTL 复制到另一个已连接的实例（已存在的 key 默认跳过）
- 切换连接或 DB 会自动清空选中，避免误删

**运维与排障**

- **内存体检**：扫描大 key、无过期 key、按前缀与类型聚合内存占用，可导出 Markdown 报告
- **实时监控**：QPS / 命中率 / 连接数曲线 + 内存水位，窗口失焦自动暂停
- **慢查询**：面板内直接读 `SLOWLOG`，按耗时排序，点击可跳到对应 key
- 连接概览（版本、角色、客户端数、运行时间）
- 内置 Redis Console，带历史命令

**其他**

- 快捷键：`⌘R` / `Ctrl+R` 刷新当前 key，`Esc` 关闭最上层弹窗或菜单
- 中英文双语、深浅色主题、自定义水印
- 本地数据提取 API：把当前 key/元素发布到 `127.0.0.1` 的 HTTP 接口，方便浏览器侧工具读取
- 应用内检查更新、下载、校验与自动替换重启（详见下方「应用内更新」）

---

## 环境要求

| 工具 | 版本 | 说明 |
|------|------|------|
| **Go** | 1.22+ | 后端语言 |
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
| **后端** | Go 1.22 | 业务逻辑、Redis 连接管理、文件持久化 |
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
