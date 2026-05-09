# LiteRedis

一个基于 [Wails](https://wails.io) + Vue 3 构建的轻量级 Redis 桌面客户端。

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

产物路径：`build/bin/LiteRedis.app`

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
├── wails.json               # Wails 项目配置
├── go.mod / go.sum          # Go 模块依赖
│
├── backend/                 # Go 后端包
│   ├── config/              # 配置与持久化
│   │   ├── types.go         # 所有共享数据结构（ConnectionConfig、KeyValue 等）
│   │   ├── store.go         # 连接配置的读写（connections.json）
│   │   └── window.go        # 窗口位置/尺寸持久化（window.json）
│   ├── redis/               # Redis 操作
│   │   ├── client.go        # 连接管理器（连接池、DB 切换、集群支持）
│   │   ├── keys.go          # Key 级别操作（SCAN、TTL、重命名、删除）
│   │   ├── value.go         # Value 读写（String/Hash/List/Set/ZSet/Stream）
│   │   └── search.go        # Value 内容搜索（HSCAN/SSCAN/ZSCAN glob）
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
│   │   │   ├── keys/        # Key 列表与树形结构
│   │   │   ├── editor/      # 各数据类型的 Value 编辑器
│   │   │   ├── connections/ # 连接管理弹窗与表单
│   │   │   └── settings/    # 设置弹窗
│   │   └── utils/
│   │       ├── keyTree.js   # Key 路径前缀树构建（按 : 分隔）
│   │       ├── typeColors.js # 各数据类型的颜色方案
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
