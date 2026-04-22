# README

## About

This is the official Wails Vue template.

You can configure the project by editing `wails.json`. More information about the project settings can be found
here: https://wails.io/docs/reference/project-config

## Live Development

To run in live development mode, run `wails dev` in the project directory. This will run a Vite development
server that will provide very fast hot reload of your frontend changes. If you want to develop in a browser
and have access to your Go methods, there is also a dev server that runs on http://localhost:34115. Connect
to this in your browser, and you can call your Go code from devtools.

## Building

To build a redistributable, production mode package, use `wails build`.


## notice

当前都是ai写的代码，还没有优化完呢，备份中...


## package

```bash
在项目根目录（/yourRoot/LiteRedis）执行：                                                                                                         
                                                                                                                                                                  
macOS（Apple Silicon）                                                                                                                                          
wails build -platform darwin/arm64                                                                                                                              
                                                                                                                                                              
macOS（Intel）                                                                                                                                                  
wails build -platform darwin/amd64                                                                                                                              
                                                                                                                                                              
macOS（同时支持 Intel + Apple Silicon，通用包）
wails build -platform darwin/universal                                                                                                                          
                                                        
打包产物在 build/bin/LiteRedis.app。                                                                                                                            
                                                                                                                                                              
---
常用附加参数：                                                                                                                                                  
                                                        
# 开发调试模式（热重载，浏览器访问）
wails dev                                                                                                                                                       

# 生产打包 + 压缩体积                                                                                                                                           
wails build -platform darwin/arm64 -upx                   
                                                                                                                                                              
# 打包并跳过前端重新安装依赖（加速二次构建）                                                                                                                    
wails build -platform darwin/arm64 -s
                                                                                                                                                              
# 查看所有可用参数                                        
wails build --help
```


在 Windows 上编译 Wails 项目，步骤如下：

前置依赖

┌─────────────────────────────┬──────────────────────────────────────────────────────────┐                                                                      
│            工具             │                           说明                           │                                                                      
├─────────────────────────────┼──────────────────────────────────────────────────────────┤                                                                      
│ Go 1.18+                    │ 官网下载安装即可                                         │
├─────────────────────────────┼──────────────────────────────────────────────────────────┤                                                                      
│ Node.js 15+                 │ 前端构建需要                                             │                                                                      
├─────────────────────────────┼──────────────────────────────────────────────────────────┤                                                                      
│ GCC（TDM-GCC 或 MinGW-w64） │ Windows 上必须，Go 的 CGo 需要它                         │                                                                      
├─────────────────────────────┼──────────────────────────────────────────────────────────┤                                                                      
│ WebView2                    │ Win10/11 已内置，无需额外安装                            │                                                                      
├─────────────────────────────┼──────────────────────────────────────────────────────────┤                                                                      
│ Wails CLI                   │ go install github.com/wailsapp/wails/v2/cmd/wails@latest │                                                                      
└─────────────────────────────┴──────────────────────────────────────────────────────────┘

▎ GCC 推荐用 TDM-GCC：https://jmeubank.github.io/tdm-gcc/

验证环境

wails doctor

全部绿色才能正常编译。

编译命令

# 普通编译
wails build

# 生产用（隐藏控制台黑窗口 + 打包成安装程序）
wails build -windowsgui -nsis -clean

输出位置

build/bin/LiteRedis.exe

加了 -nsis 还会生成一个 .exe 安装包。
                                                                                                                                                                  
---                                                                                                                                                             
最常见的坑：GCC 没装或没加入 PATH，wails doctor 会报红，按提示补上就行。