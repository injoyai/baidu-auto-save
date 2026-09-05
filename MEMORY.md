# MEMORY.md — 项目记忆

## 项目概况

百度网盘自动转存服务（baidu-auto-save）：Go + Vue 3 的 Web 服务，定时自动转存百度网盘分享链接，支持过滤、去重、通知、开放 API 推送。内网单管理员部署，明确不做防爆破限速、Cookie 加密等（避免过度设计）。

## 技术栈

- 后端：Go 1.25.0 / Gin / robfig/cron/v3 / modernc.org/sqlite（免 CGO）/ golang-jwt/v5
- 前端：Vue 3 + Vite + Element Plus + Pinia + Vue Router，构建产物 Go embed 嵌入
- 部署：Docker 多阶段构建（node:22-alpine → golang:1.25.0 → alpine:3.20 含 tzdata），CGO_ENABLED=0，数据目录 /data（SQLite）

## 关键架构决策（2026-09-05 审查确认）

1. **上游 BaiduPCS-Go 的 `internal/pcscommand` 包不可导入**（Go internal 规则），转存编排逻辑（RunShareTransfer 等）需自研；仅复用可导入的 **`baidupcs` 包（模块根目录下，非 `pcs/baidupcs`）**（分享页访问、提取码验证、目录列举、批量转存、重命名）。
2. 重命名采用两步实现：转存成功后调用文件管理重命名接口（上游转存接口不支持逐文件 newname）。
3. 去重两层：task_files 表 MD5 主判据（任务内）+ 转存请求 ondup=skip 兜底（含跨任务）。
4. Cookie 失效修复路径：`PUT /accounts/:id` 更新（必须有，否则死锁）。
5. transfer_logs.id 即对外 run id（数据模型无独立 run_id 字段）。
6. 开放推送 `POST /open/links` 创建任务后立即执行一次。

## 模块结构（已实现，构建+测试通过）

- `internal/config`：config/config.yaml 全量配置（server.addr/auth.password/storage.data_dir）；不存在时自动生成注释模板并退出；环境变量 APP_PASSWORD/DATA_DIR/ADDR 可覆盖（env > yaml）
- `internal/db`：SQLite 迁移 + Account/Task/TransferLog/TaskFile/Setting DAO
- `internal/baidu`：baidupcs 适配器（Client 接口 + 自研 ListShareDir 遍历 /share/list）
- `internal/engine`：转存编排（walkShare/ensureDir/renameTransferred/classifyErr/withRetry），`BuildCookieStr` 已导出供 API 层复用；有完整单测（mock Client）
- **目录结构保留（2026-09-05 修复）**：百度 Transfer(fsids, dest) 不会按 fs_id 保留分享内路径——整批落到 dest 平铺。destDirFor 必须把子目录层级显式编进 dest：文件落盘目录 = 保存目录/<改名B>/<相对最深改名命中目录的子路径>（无改名命中时 = 保存目录/<分享内相对路径>）。子目录分组后 ensureDir 负责逐级建目录（运行内 mkdirCache 缓存，父路径不重复请求）。
- **文件夹改名（folder_renames）**：tasks 表 JSON 列，map[勾选目录]→新名字；destDirFor 取「命中且配置了改名的最深勾选目录」为基准（级联勾选产生的无改名子级选中项不覆盖父级改名）。
- **转存错误处理**：百度「文件重复」响应 ErrNo 非 4，需同时匹配 ErrMsg contains 文件重复/已存在；同一分享不同目录可能存在同 MD5 文件，运行内需 seenMD5 map 去重（仅 DB 历史去重不够）。
- **性能特征**：全量 3744 文件 ≈ 180 目录分组，瓶颈是 Mkdir 逐级串行请求 + 批次间 500ms 限速；首次慢属正常，后续增量只转新增 MD5 + 目录已存在（Mkdir 快速失败），显著加快。
- `internal/scheduler`：动态 cron 管理（AddTask/RemoveTask/RunNow），启动时恢复 running→idle
- `internal/notify`：4 渠道通知（企业微信/Server酱/Telegram/自定义 webhook）
- `internal/api`：Gin 路由（JWT 认证 + 开放推送 X-API-Token）+ embed SPA 静态托管
- `main.go`：加载 config/config.yaml（自动生成模板）→ env 覆盖 → 默认值校验；数据目录来自配置
- `web/`：Vue 3 SPA（7 视图），构建产物 dist 由 Go embed（占位 web/dist/index.html 在无前端构建时兜底）。前端设计语言为「极光 Aurora」token（App.vue 定义：深空侧栏 + 云白内容区，主色极光青 #14b8a6，渐变 --grad 青→天蓝→紫），页面统一用 Reveal 组件做入场瀑布动画；Settings 页为左主列（双列 CSS grid 表单）+ 右侧粘性操作卡布局；列表页表格列宽统一约定（2026-09-06）：标签列 width=96、时间列 width=170 固定、数字列 width=84 居中、Run ID width=90，仅文本列（名称/目录/摘要）用 min-width 弹性伸展
- `Dockerfile` / `docker-compose.yml` / `.dockerignore`

## 上游 API 关键事实

- go.mod 伪版本：`v0.0.0-20260821135237-225bdd3b6cb2`（对应 v4.0.2 commit）
- `NewPCSWithCookieStr(appID int, cookieStr string)`；**PCS 接口（quota/mkdir/rename/cp-mv）必须用 app_id=266719**：网页版 250528（PanAppID）已被百度对纯 Cookie 请求封禁（报 31030 pcs token not exist），266719 实测可用（2026-09-05 真实账号验证）；share/* 网页接口（verify/transfer/list）仍用 250528
- HTTPClient 用 `Req(...)` 方法（无 Do）；`jsonResult.Raw()` 是方法
- AccessSharePage / PostShareQuery / SetStoken / Transfer / Rename / Mkdir / QuotaInfo / FilesDirectoriesList
- `/share/list` 目录遍历上游只有注释代码，自研实现于 internal/baidu
- **`/share/list` 返回的 fs_id/size/isdir 是字符串**（非数字），Go 结构体必须用 `json.Number` 或 string 接收，用 int64 会 unmarshal 失败导致列表为空（2026-09-05 实测踩坑）
- 纯 Cookie 可用性参考：pan.baidu.com/api/quota、/api/user/getinfo、/api/gettemplatevariable（bdstoken）均正常；pcs.baidu.com/rest/2.0/pcs/* 走 266719 正常

## 设计文档

- `docs/superpowers/specs/2026-09-05-baidu-auto-save-design.md`（v2，已确认，含完整 API/数据模型/流程/风险表）

## 坑点

- 上游锁定 go.mod 版本；v4.0.2（2026-08）为当前版本基线。
- **Windows PowerShell 管道操作 UTF-8 中文文件会双重编码损坏**（Get-Content/Set-Content 默认 GBK），禁止用 PowerShell 读写 UTF-8 文档；一律用 Read/Write 工具。
- **Go 重命名模板不得用 regexp.ReplaceAllString 展开 `\1`**（`$1` 会被二次解释导致后缀重复如 "EP01.mp4.mp4"）；用 ReplaceAllStringFunc 手动展开（engine.renameTemplate 已修复）。
- Server酱等 webhook 参数必须用 `url.Values{}.Encode()` 做 URL 编码，不能用 JSON 转义。
- 前端 `web/src/router/index.js` 引用视图须用 `../views/*`（router 在 src/router/ 子目录）。
- Git 仓库：github.com/injoyai/baidu-auto-save（公开）。**2026-09-05 安全事件**：初版提交误推 `data/`（SQLite WAL，可能含百度 Cookie）、`config/config.yaml`（登录密码）、`tmp_clear/`；补救：用户已删除远程仓库重建（同名空仓库），本地需删 `.git` 重新 init（或 orphan 分支）生成干净历史后推送；**泄露过的百度 Cookie 必须重登轮换**。`data/ config/ docs/` 均已在 .gitignore，不得再入库
- 用户本机 git 代理 127.0.0.1:12345（旧 9876 已失效）；拉取上游库时可用 `GIT_CONFIG_COUNT` 环境变量临时注入。
- **前端改动验证链路**：npm build → 重新编译 Go（embed 旧 dist 则无效）→ 杀掉旧进程再启动（Windows 下旧进程不释放端口则新 exe 启动失败或端口仍被占）。曾出现新 exe 多次启动但 8080 仍被旧 pid 占用，导致服务器一直返回旧 chunk——浏览器「强刷也无效」时先查 `Get-NetTCPConnection -LocalPort 8080` 的进程启动时间是否早于 dist 编译时间。
- **Element Plus el-tree 懒加载**：容器用 v-show 隐藏时不会触发 load 回调（含根节点 level=0），必须用 v-if；根节点（level=0）也会调用 load，需 resolve 根列表（分享根目录），不能 resolve([])。
- 前端缓存策略：index.html 响应 no-cache；资产路径用 `/static/`（vite assetsDir），换命名空间可强制刷新旧缓存的 `/assets/` chunk。
- **后端 `*time.Time` JSON 序列化为 RFC3339**（如 `2026-09-05T15:04:05+08:00`），前端统一用 `web/src/utils/time.js` 的 `fmtTime()` 格式化为 `YYYY-MM-DD HH:mm:ss` 显示（Tasks/Accounts/Dashboard/Logs 均已接入）；后端 cron 预览接口已直接返回格式化字符串。
- **上游 `baidupcs.QuotaInfo()` 返回顺序是 (总量, 已用)**，与本项目 `Client.Quota()` 的 (used, total) 约定相反，client.go 已做适配；直接透传会导致进度条永远 100%。db.go migrate 里有启动时自动对调 `quota_used > quota_total` 反常行的数据修复（幂等）。
- **App.vue 全局渐变按钮选择器须排除 plain/text/link 变体**：`.el-button--primary` 的极光渐变背景必须用 `:not(.is-plain):not(.is-text):not(.is-link)` 限定，否则 plain 按钮（如「立即运行」「删除」）浅底被渐变覆盖导致文字看不清。
