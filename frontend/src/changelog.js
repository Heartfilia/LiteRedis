// 每个版本「更新内容」弹窗的文案。发布前在这里补一条即可：
//   key 用不带 v 的版本号（与 AppVersion 去掉 v 后一致）
//   新条目提供 { zh, en }；历史中文字符串保留原文，不作为英文说明使用
// dev / local 预览使用独立的 PENDING_RELEASE_NOTES，不代表已有正式版本发布。
// 没有对应条目或语言的版本会退回 GitHub Release 说明（含提交列表兜底）。

const RELEASE_084_NOTES = {
  zh: `## v0.8.4 更新内容（相较 v0.8.3）
* 顶部连接 Tab 暂不开放：隐藏设置入口与标签栏，连接继续通过侧栏切换，保留既有配置以便后续优化
* String / Hash 保存冲突可主动查看打开时内容、当前草稿和 Redis 最新有界快照；不自动覆盖，草稿保留，截断/二进制/删除/类型变化明确提示
* 搜索条件收藏入口暂时隐藏，释放搜索区空间；保留本机收藏数据和功能实现，原有搜索历史与置顶记录不受影响
* 超大 String 支持按原始字节跳转与翻页，每次最多 64KiB，仅供只读查看；跨 UTF-8 边界或二进制段以 Base64 保留字节，不作为一致性快照
* 连接表单新增分阶段诊断与脱敏报告：实际检查传输、代理、已信任 SSH、TLS、Redis 认证/PING；未知 SSH 指纹不会自动信任，Cluster 仅检查种子节点
* 命令台新增本机命令模板与参数预览，安全引用独立参数，填入后需手动执行，继续受生产确认、只读与结果未知保护；不保存参数值
* 连接新增开发、测试、生产环境标识，左侧连接状态点后显示紧凑 D/T/P 标签，移除顶部环境展示条；生产批量删除、TTL、改前缀、导入/复制目标和命令台高风险操作增加输入式确认，环境标签不改变只读保护模式
* 设置中的「安装说明」升级为「软件说明」，快捷键指示移入二级菜单，按全局操作、命令面板和命令台分组展示
* 当前版本的「新功能」说明支持中英文，可随时从设置中重新查看，不必等待首次启动提示
`,
  en: `## What’s new in v0.8.4 (since v0.8.3)
* Top connection tabs are temporarily unavailable: the setting and tab bar are hidden, sidebar switching remains available, and saved preferences are retained for future improvements
* String / Hash save conflicts offer an opt-in comparison of original content, your draft and a bounded current Redis snapshot. Drafts are preserved without automatic overwrite, with explicit truncation/binary/deletion/type-change markers
* The saved-search entry is temporarily hidden to keep the search area compact. Existing local data and implementation are retained; search history and pinned entries are unaffected
* Large Strings support byte-offset navigation and paging, reading at most 64KiB per request in read-only mode. Split UTF-8 and binary ranges use Base64 to preserve bytes; reads are not a consistent snapshot
* Connection forms offer staged diagnosis and sanitized reports for transport, proxies, trusted SSH, TLS and Redis authentication/PING. Unknown SSH fingerprints are never trusted automatically; Cluster checks cover the seed only
* Local command templates provide parameter previews and safe standalone argument quoting. Prepared commands require manual execution and retain production, read-only and uncertain-result safeguards; parameter values are not saved
* Connections can be labeled Development, Staging or Production, with compact D/T/P badges after sidebar status dots instead of a top banner. High-risk production batches, import/copy targets and console operations require typed confirmation, independently of read-only protection
* Installation instructions in Settings become Software guide, with keyboard shortcuts in a submenu grouped by global actions, command palette, and console
* What's New for the current version is available in Chinese and English and can be reopened from Settings at any time, without waiting for a first-launch prompt
`,
}

// 本地工程包预览当前准备发布的说明；下一轮开发再创建独立待发布条目。
export const PENDING_RELEASE_NOTES = { ...RELEASE_084_NOTES }

export const RELEASE_NOTES = {
  '0.8.4': RELEASE_084_NOTES,
  '0.7.0': `## 连接与标签
* 新增顶部连接标签栏：多个连接并排切换，可拖拽排序、中键关闭，每个连接保留自己的浏览状态（默认关闭，在 设置 → 其他设置 里开启）
* 折叠侧栏的按钮只在有已连接的 Redis 时出现；折叠状态下所有连接断开会自动恢复原状
* 收藏和最近访问不再重复显示同一个 key

## 主题
* 主题按钮改为菜单选择：浅色 / 护眼模式 / 暗色 / 极客模式（终端风）/ 跟随系统
* 按钮图标跟随当前选择，后续新主题会继续加进这个菜单

## 命令面板（Cmd/Ctrl+K）
* 新增动作：直接打开命令台、发布订阅、实时监控、内存医生、诊断面板
* 输入即搜当前库里的 key，点一下直达（左侧树会自动展开定位）
* 最近在命令台执行过的命令可以直接重跑
* 需要连接的动作在未连接时置灰并说明原因

## 命令台
* 命令名 / 子命令 / 关键字自动补全（Tab 补全，回车执行）
* 历史命令带命令名高亮
* 命令历史持久化，重启应用后按 ↑ 还能翻到

## 性能
* key 树、Stream 列表、超大 JSON 树形视图改为按需渲染，大数据量下滚动和展开更流畅
\n## 界面与交互
* 中间面板底栏的「加载更多」改成方形图标按钮，宽度够时居中、面板变窄时并入左侧图标组
* 编辑弹窗支持 JSON 格式化 / 树形预览，并从点击的「编辑」按钮缩放进出，弹窗尺寸也放大了
* 树形面板新增「自动换行」开关，右键 key 可直接复制 key 或 value，长文本不再截断
* 界面元素默认不可选中（Cmd/Ctrl+A 不会把按钮和列表文字一起选上），list / set / zset / hash / stream 的值仍可拖选多选
* 格式化、树形、自动换行等按钮增加了触发态高亮
* GitHub 入口移到设置面板左下角，版本检测后不再显示「打开发布页」按钮

## 更新
* 更新状态提升到全局，关掉设置面板再打开仍能看到进度
* 打开设置面板时自动静默检测更新，失败不打扰；手动检测失败才提示
* 新增「更新内容」弹窗：每次版本更新后第一次打开显示一次，说明本次变化
`,
  '0.7.1': `## 内存体检
* 新增「扫描数据库」选择，可独立扫描其他 DB，不改变主界面当前数据库；切换 DB 后清空旧报告，扫描期间禁止切换
* 汇总新增 Redis 整体已用内存与 DB 内存占比估算，Markdown 报告同步包含新增指标及统计口径
* 占比按已扫描 key 的 MEMORY USAGE 合计除以 Redis used_memory 计算；按 pattern 筛选、扫描截断或部分读取失败时，明确标为「已扫描 Key 占比」
* Cluster 仅支持 DB 0，整体内存汇总所有主节点，不包含副本；修复并发扫描可能超出全局扫描上限的问题，并对节点内重复扫描的 key 去重
* 兼容性说明：Redis 3.x 不支持 MEMORY USAGE（需要 Redis 4.0+）；命令不可用、权限不足或降级为序列化长度、元素数时，占比显示 —，不会用降级数据冒充实际内存占比
* 内存大小使用默认集合采样，扫描不是一致性快照；Redis 整体内存还包含连接、缓冲区等开销

## 大 key 导出
* 改为分段读取、逐 key 写入临时文件，不再同时缓存全部选中 key 和完整导出文件，保持现有 JSON 导入兼容性
* String 先检查长度，再按 256KiB 读取；Hash / Set 使用 SCAN 并去重，List / ZSet / Stream 分页读取，跨实例复制复用有界读取逻辑
* 增加单 key 原始数据 32MiB / 10 万项、编码后单文件 64MiB 限制；超限明确报错，不截断数据
* 新增导出取消操作，同时仅允许一个导出任务，10 分钟超时；失败或取消不覆盖目标文件，并清理临时文件
* 分页导出不是一致性快照，会尽量检测删除、类型和数量变化；需要一致备份时应暂停源数据写入或使用 Redis 原生备份方案

## 安全与稳定性
* 连接管理新增独立 SSH 账号管理页，支持账号保存、编辑、删除及单独测试 SSH 握手与认证；Redis 隧道改为选择已保存账号，可从下拉框直接跳转新建并返回选中
* SSH 管理页返回 Redis 的切换按钮使用独立绿色及返回箭头；SSH 账号选择框与 DB Index 等表单控件统一高度和样式，移除下方重复的管理按钮
* Redis 连接右键新增「复制连接」：保留全部配置与分组、名称追加 copy，同组重名自动编号；创建后打开副本供修改，不影响原连接或自动连接
* 兼容迁移旧 Redis 连接中的 SSH 配置，支持多个连接复用账号与配置导入导出；修改共享账号传输参数后相关连接需重连，被引用账号禁止直接删除
* 修复跨实例复制未校验目标连接保护模式、批量改名前缀可能覆盖并发写入 key 的问题
* 导入前校验数据与 TTL，在提交脚本内检查已有目标并默认跳过；修复 TTL 为 0 被误导入为永久 key 的问题
* 修复自定义拨号场景的 TLS 校验、HTTPS 代理握手、代理取消及 SSH 超时后的连接清理问题
* 加强导出文件权限与命令转义、保护模式命令校验及 SSH 主机指纹文件损坏时的处理，保留密码和私钥口令中的首尾空格
* 更新前后端依赖和 Go 工具链安全基线，发布流水线增加依赖安全检查
* Go 最低版本升级至 1.26.11，修复标准库 net/http 漏洞 GO-2026-6859，保留 CI 漏洞扫描门禁

## 编辑可靠性（2026-11-11 补充）
* String 使用有界预览；内容截断、非 UTF-8 或检测到读取变化时只读，防止把不完整预览误存回 Redis
* JSON 格式化保留大整数、指数、负零、重复属性及字符串转义原文；树形查看和复制不再损失数字精度，重复属性保留为文本查看
* String 保存和 Hash 单字段编辑 / 新增增加原子冲突检查；冲突保留草稿，不覆盖其他客户端的新值，成功保存保留原 TTL
* 切换连接、数据库、key 或关闭编辑器时保护未保存输入，保存中阻止离开；断线不主动清除草稿，草稿仅驻留内存，不支持重启恢复
* 写操作固定目标 DB 并禁用自动重试；结果未知或可能部分执行时保留输入、阻止重复提交，需核实后显式解锁；Cluster MOVED / ASK 不自动重放
* 恢复 Hash 字段编辑弹窗保存成功后静默关闭的交互，不再在输入框上方显示并残留「已更新」；保存失败仍保留草稿和错误提示

## 数据任务
* 导出、导入和复制统一接入任务面板，展示进度及成功 / 跳过 / 失败数，支持取消和复制失败清单
* 导入和跨连接复制先在同槽临时 key 构建完整数据，再原子提交；确认框显示目标连接、DB、默认跳过已有 key 及限制，界面不提供覆盖开关
* 导入 / 跨连接复制每 key 限制为 5MiB / 1 万项，超限明确拒绝；取消只停止后续操作，不回滚已完成写入，也不能终止已发送的 Lua 脚本
* 细化内存体检的权限不足、命令不支持 / 禁用和采样失败提示，不将降级估算冒充实际内存占比

## Key 搜索与加载体验（2026-11-12 补充）
* 修复模糊搜索点击「加载更多」后 key 面板空白的问题：追加加载期间保留已有列表和滚动位置，修正虚拟列表重新绑定时的滚动状态
* 后续分页失败会显示错误并保留已加载结果；重试时清除旧错误，SCAN 重复返回的 key 自动去重，空的后续页不清空已有结果
* 修复「加载中」文字跟随圆圈一起旋转、提示挤在面板顶部的问题：首次加载提示居中，仅圆圈旋转，文字保持静止，适配深浅色主题
* 隔离首次加载与「加载更多」按钮的状态样式，按钮不再整块旋转或出现额外圆圈边框；追加加载期间仍可查看已加载的 key

## 退出与快捷键
* 设置新增快捷键说明，按全局操作、命令面板和命令台分组，展示按键、适用场景及关闭/退出保护，支持中英文与深浅色主题
* 调整设置底部顺序：安装说明位于普通设置最后；开发者/API 组仅在开启开发者模式后显示，并始终排在安装说明之后
* 关闭窗口或 Cmd/Ctrl+Q 时，仅在仍有已连接 Redis 时提醒（包含后台连接）；没有连接则直接退出，即使留有断线草稿也不提醒，保存中仍阻止退出
* 退出确认框收窄至 360px，适配深浅色，默认「继续使用」、Esc 取消，丢弃草稿的退出按钮使用红色提醒
* Cmd/Ctrl+W 只关闭当前 key，保留连接、数据库和搜索条件；有草稿先确认，保存中禁止关闭，无 key 时不退出应用；macOS 新增 File → Close Key 菜单

## 第三方平台导入
* 设置页新增「第三方平台数据导入」，支持 AnotherRedisDesktopManager（.ano）和 Tiny RDM（.zip）的连接配置
* 导入前预览可导入数量、跳过明细和注意事项；确认后只新增连接，不覆盖现有连接，同组重名自动编号
* 按地址去重：host:port 已存在的连接直接跳过并说明原因，同文件内重复只保留第一条；启用 SSH 时按跳板机路由区分，不会误判或覆盖
* 支持名称、地址、ACL、只读、DB、分组、Cluster、SSH 账号、基础 TLS 和自定义代理；Sentinel、Unix Socket、双向 TLS、自定义 SNI、SSH Agent、系统代理会明确跳过并说明原因
* ARDM 的 TLS 保持其原本的跳过证书校验行为；私钥和 CA 只保存路径，跨机器导入需要重新指定

## 连接图标
* 图标内字符支持自定义，最多 4 个字符，按每行 2 个显示；留空仍使用连接名称首字，图标底图尺寸保持不变

## 应用包更新代理
* 首次使用时代理地址预填 http://127.0.0.1:1081，不用手动输入；已经保存过的地址保持不变

## 超大 String
* 超过 5MB 的 string 只展示前 2000 个字符，不再先下载 5MB 内容，避免打开大 key 时界面卡住
* 超大 key 为只读预览：无法编辑、复制、格式化和保存，避免复制或保存不完整内容；提示会明确说明只读原因
`,
  '0.8.2': {
    zh: `## v0.7.6 至 v0.8.2 更新概览
* SSH 账号独立管理：保存、编辑、测试和复用账号，兼容旧连接配置迁移；被连接引用的账号不可直接删除
* 连接右键支持「复制连接」，保留配置与分组，同组重名自动编号，创建后打开副本供修改
* 支持导入 AnotherRedisDesktopManager（.ano）和 Tiny RDM（.zip）的连接配置，导入前预览、按地址与 SSH 路由去重，不覆盖已有连接；不支持的配置明确提示
* 连接图标可自定义最多 4 个字符，每行显示 2 个，留空仍使用连接名称首字
* 应用包更新代理默认预填 http://127.0.0.1:1081，已保存的地址保持不变
* 超过 5MB 的 String 只读取并展示前 2000 个字符；超大 key 为只读预览，禁止编辑、复制、格式化和保存，避免误用不完整内容
`,
    en: `## Updates from v0.7.6 through v0.8.2
* Manage SSH accounts separately: save, edit, test, and reuse accounts, with migration of legacy connection settings. Accounts referenced by connections cannot be deleted directly
* Duplicate Connection is available in the connection context menu, preserving settings and groups, numbering duplicate names, and opening the copy for editing
* Import connections from AnotherRedisDesktopManager (.ano) and Tiny RDM (.zip), with a preview and deduplication by address and SSH route. Existing connections are never overwritten, and unsupported settings are reported
* Customize connection icons with up to 4 characters, displayed 2 per row; leaving the field empty uses the first character of the connection name
* The application update proxy defaults to http://127.0.0.1:1081; previously saved addresses are preserved
* Strings larger than 5MB only load and display the first 2000 characters. Large keys are read-only: editing, copying, formatting, and saving are disabled to avoid using incomplete content
`,
  },
  '0.8.3': {
    zh: `## 更新检查与下载
* 正式版检查更新改为读取 GitHub /releases/latest 的网页跳转，安装包与 SHA256SUMS 直接按发布产物名下载，不再依赖 GitHub API 配额或 token
* beta / rc 预览版仍通过 release 列表检查更新；本地工程包按稳定通道检查
`,
    en: `## Update checks and downloads
* Stable updates read the GitHub /releases/latest redirect and download packages and SHA256SUMS directly by asset name, without GitHub API quotas or tokens
* beta / rc previews still check the release list; local engineering builds check the stable channel
`,
  },
}
