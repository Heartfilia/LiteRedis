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
