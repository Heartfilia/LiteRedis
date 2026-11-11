# 代码与安全检查记录

检查日期：2026-11-09。范围为 Redis 写入与导入导出、保护模式、代理/TLS、SSH 信任与凭据处理、依赖及现有测试。不是完整渗透测试，也不代表没有其他漏洞。

## 已修复

| 问题 | 处理方式 | 主要文件 |
|---|---|---|
| 跨实例复制未校验目标连接保护模式 | 在后端入口调用 `ensureWritable`，添加回归测试 | `app.go`、`security_test.go` |
| 自定义 Redis Dialer 绕过 go-redis 默认 TLS 握手 | 自定义拨号后显式握手、验证证书，并设置握手超时；覆盖普通及 Cluster 选项 | `backend/redis/client.go`、`tls_test.go` |
| HTTPS 代理按明文发送 CONNECT 及代理凭据 | 先与代理完成 TLS 和证书验证，再发送 CONNECT | `backend/redis/client.go`、`proxy_test.go` |
| 代理握手取消后连接泄漏或长时间等待 | HTTP CONNECT 响应取消并关闭连接，SOCKS5 使用带 context 的原生拨号；添加取消测试 | `backend/redis/client.go`、`proxy_test.go` |
| SSH 超时/取消后迟到的连接无人关闭 | 使用 SSH 库的 `DialContext`，避免多层带缓冲结果通道遗留连接 | `backend/ssh/tunnel.go` |
| 不完整代理/SSH 配置可能静默直连，Cluster 可绕过 SSH/代理互斥检查 | 连接和测试连接统一提前校验 | `backend/redis/client.go` |
| 批量改名 EXISTS + RENAME 有覆盖竞态 | 改为原子 `RENAMENX`，减少一次 pipeline 往返，正确统计部分成功数；添加确定性并发测试 | `backend/redis/keys.go`、`keys_test.go` |
| 导入覆盖先删旧值，随后才发现内容无效 | 先校验类型、空集合、Stream ID、score、TTL；后续可靠性收口改为同槽临时 key 完整构建、设置 TTL 后原子提交，默认跳过检查也在同一脚本内 | `backend/redis/export.go`、`import_staged.go`、对应测试 |
| TTL 为 0 的导入变成永久 key | 保留立即过期语义，TTL -2 的不存在条目跳过 | `backend/redis/export.go` |
| 命令导出注释中的 key 可通过换行注入命令 | 对注释元数据及 Stream ID 转义 | `backend/redis/export.go`、`export_test.go` |
| 数据导出默认其他用户可读，覆盖旧配置导出不收紧权限 | 数据与连接导出统一原子写入，使用 `0600`（Windows 仍依赖 ACL） | `app.go` |
| 保护模式放行会修改诊断状态的 LATENCY RESET，解析器与执行逻辑不一致 | 删除 RESET 白名单；与执行共用参数解析器 | `backend/redis/readonly.go`、`readonly_test.go` |
| SSH 密码及私钥口令被 TrimSpace 改写 | 保留凭据原始字节；添加含首尾空格的加密私钥测试 | `backend/ssh/tunnel.go`、`tunnel_test.go` |
| SSH 指纹文件损坏时静默重新信任主机 | 改为拒绝连接，保留损坏文件供恢复；更新回归测试 | `backend/ssh/hostkey.go`、`hostkey_test.go` |
| 已知依赖及 Go 标准库安全问题 | 更新 Vue/Vite/插件、Go 网络和加密依赖；Go 基线提升为 1.26.10；发布流水线读取 go.mod 并增加安全审计 | `go.mod`、`frontend/package*.json`、`.github/workflows/release.yml` |

## 验证结果

- `go test -race -count=1 ./...`：通过。
- `go vet ./...`、`go build ./...`：通过。
- 前端 `npm test`：78 项通过；`npm run lint`、`npm run build`：通过。
- `npm run e2e`：10 项通过（包括导出取消；浏览器及模拟 Wails 接口，并非真实桌面端到端测试）。
- `npm audit`：0 漏洞。
- `govulncheck`：0 个可达漏洞、0 个已导入包漏洞；仍报告 4 个依赖模块级漏洞，当前代码未调用相关功能。
- 前端测试仍有 Wails 自动生成 runtime 的 ESM 元数据警告，不影响本次测试结果。

Go 工具链会按项目要求自动下载（未替换全局 Go 安装）。因 `main.go` 嵌入 `frontend/dist`，应先构建前端，再运行 Go 检查，不能同时重建 dist 和编译 Go。

## 后续优化与边界

1. **大 key 导出已优化**：String 预检 STRLEN，按 256KiB GETRANGE；Hash/Set 使用 SCAN 并去重，List/ZSet/Stream 分页（约 200 项），Stream 固定末尾 ID。逐 key 编码写入同目录临时文件，成功才替换目标。单 key 原始数据 32MiB / 10 万项、编码后单文件 64MiB，支持取消、10 分钟超时及全局单任务。跨实例复制复用有界读取。仍不是一致快照，需暂停源数据写入；COUNT 不是硬上限，单个集合元素/字段可能很大，Redis 响应、Go 容器及 JSON 编码开销不包含在原始字节预算内。
2. **凭据存储**：连接配置仍是受文件权限保护的本地明文。建议后续接入 Keychain / Credential Manager，保留旧配置兼容迁移。
3. **更新供应链**：SHA256SUMS 能检查下载内容一致性，但不是独立发布者签名。建议引入签名清单及应用签名/公证，制定可信公钥轮换策略。
4. **导入脚本边界**：当前使用同槽临时 key 构建并原子提交，需要 EVAL、临时 key 和脚本内命令权限；Lua 运行时失败不保证回滚，清理失败会返回临时 key 并尝试设置过期。取消不能终止已发送脚本，额外内存及阻塞风险仍需评估；详见 `RELIABILITY_PLAN.md` 与 `ACCEPTANCE.md`。
5. **SSH 首次连接**：仍采用 TOFU，首次信任不能抵御首次连接时的中间人攻击。重要环境应通过可信渠道核对指纹。
6. **验证范围**：尚未进行真实 SSH + Redis Cluster 联调、真实三平台桌面启动及更新替换测试；发布前应在测试实例上验证。此次没有连接或修改生产 Redis 数据。
