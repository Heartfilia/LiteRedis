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
wails build -windowsgui -nsis -clean
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
  wails build -platform windows/amd64 -windowsgui -clean
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
| `-windowsgui` | Windows 专用，隐藏控制台黑窗口 |
| `-nsis` | Windows 专用，生成 NSIS 安装程序 |

查看所有参数：

```bash
wails build --help
```
