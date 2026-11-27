# 前后端接口契约（Wails 绑定 + 事件）

> 状态：P0 联调契约，已按确认方案开始实施。**后端实现以本文件为接口规范**；如需变更，先在本文件改，再改代码。P0 实现范围及服务全名见 `P0.md`。
> 前端不直连 Clash API，不接触密钥；所有数据经 Go 侧。

## 1. 约定

- 绑定：Wails v3 服务结构体的导出方法，`ctx context.Context` 为首参，返回 `(T, error)`。
- 错误：`error` 的字符串直接展示给用户，必须是中文且不含内部路径/密钥。
- 命名：方法 `PascalCase`，领域前缀由结构体承担（如 `Nodes.List()`）。
- 时间：`int64` Unix 毫秒。速率：字节/秒（`int64`）。总量：字节（`int64`，JS 侧按 number 处理，超过 2^53 不现实）。
- 列表分页：连接与日志用「快照 + 差量」，不分页；节点/订阅/规则全量返回（量级 < 10k）。

## 2. 类型

```ts
type CoreStatus = 'stopped' | 'starting' | 'running' | 'error'
type Mode = 'rule' | 'global' | 'direct'
type Action = 'direct' | 'block' | 'proxy'
type LatencyStatus = 'idle' | 'testing' | 'ok' | 'timeout'

interface CoreState {
  status: CoreStatus
  mode: Mode
  uptimeMs: number
  memoryBytes: number
  version: string          // Ferry 版本
  coreVersion: string      // sing-box 版本
  serviceMode: boolean     // 是否由系统服务承载内核
  error?: string
}

interface Traffic {                  // 1 Hz
  up: number; down: number           // 当前速率 B/s
  activeConns: number
  totalUp: number; totalDown: number
  memoryBytes: number
  /** 最近 60 秒速率，定长 60，索引 0 为最旧 */
  upHistory: number[]; downHistory: number[]
}

interface Node {
  id: string; tag: string; name: string; groupId: string
  type: string                       // ss | vmess | vless | trojan | hysteria2 | tuic ...
  server: string; port: number       // server 仅用于展示
  region?: string                    // 从节点名解析出的地区，可空
  latencyMs: number                  // -1 表示未测
  latencyStatus: LatencyStatus
  selected: boolean                  // 所属分组当前选中项
  favorite: boolean
}

interface Group {
  id: string; name: string; type: 'select' | 'urltest'
  members: string[]                  // nodeId 或子分组 id
  selected?: string                  // select 专用
  testUrl: string; intervalMin: number
}

interface Subscription {
  id: string; name: string
  urlMasked: string                  // 形如 https://example.com/api/****
  nodeCount: number; groupCount: number
  usedBytes: number; totalBytes: number   // totalBytes 为 0 表示未知
  expiresAt: number                       // 0 表示未知
  lastUpdatedAt: number
  intervalHours: number; autoUpdate: boolean
  userAgent: string
  status: 'ok' | 'updating' | 'error'
  error?: string
  skipped?: { line: number; reason: string }[] // 仅导入结果；line=0 表示与已有节点重复
  warnings?: string[]                // 例如 HTTP 订阅未加密警告
}

interface Rule {
  id: string; name: string; enabled: boolean
  apps: string[]                     // 'chrome.exe' | 'C:\..\chrome.exe' | '*\Games\*.exe' | '@group:games'
  hosts: string[]                    // 'example.com' | '*.x.com' | '10.0.0.1-10.0.0.99' | 'geosite:cn'
  ports: string[]                    // '443' | '8000-9000'
  network: 'any' | 'tcp' | 'udp'
  action: Action
  targetId?: string                  // action=proxy 时的分组或节点 id
  resolveBefore: boolean             // 默认 true
  pinned?: boolean                   // 首条/末条固定规则不可删改
}

interface Connection {
  id: string
  process: string                    // 展示名，如 chrome.exe；未知时为 ''
  processPath: string
  host: string; port: number
  ruleName: string; ruleId?: string
  action: Action
  outboundTag: string                // 'DIRECT' | 'REJECT' | 节点 tag
  chain: string[]                    // 多级代理链，空即为单跳
  up: number; down: number
  startAt: number
}

interface LogEntry { seq: number; time: number; level: 'trace'|'debug'|'info'|'warn'|'error'|'fatal'; message: string }

interface ServiceStatus { installed: boolean; running: boolean; startType: 'auto'|'manual'|'disabled'; version: string }
interface TunStatus { enabled: boolean; running: boolean; stack: 'mixed'|'system'|'gvisor'; strictRoute: boolean; excludes: string[]; error?: string }
interface SysProxyStatus { enabled: boolean; address: string; bypass: string[] }
interface InboundStatus { mixedPort: number; allowLan: boolean }
```

## 3. 方法

### 3.1 Core（内核生命周期）
| 方法 | 说明 |
|---|---|
| `Status() CoreState` | 当前状态快照 |
| `Start() error` / `Stop() error` / `Restart() error` | 幂等 |
| `SetMode(m Mode) error` | 走 Clash API，不重启内核 |
| `SelectNode(groupId, nodeId string) error` | 同上 |
| `LatencyTest(nodeIds []string) (map[string]int, error)` | 触发测速；结果也通过 `nodes:latency` 事件推送 |
| `Snapshot() (Traffic, error)` | 首屏用，避免等第一次推送 |

### 3.2 Nodes / Subscriptions
| 方法 | 说明 |
|---|---|
| `Nodes.List() (NodeList, error)` | `NodeList{ groups: Group[], nodes: Node[] }`，前端自行组树 |
| `Nodes.SetFavorite(nodeId string, on bool) error` | |
| `Subs.List() ([]Subscription, error)` | |
| `Subs.AddFromURL(name, url string, autoUpdate bool) (Subscription, error)` | url 仅在此处出现，落盘前加密 |
| `Subs.AddFromText(name, text string) (Subscription, error)` | 粘贴的 base64/URI 列表 |
| `Subs.AddFromFile(path string) (Subscription, error)` | |
| `Subs.ImportFromQR(imageBase64 string) ([]Subscription, error)` | 剪贴板/文件图片 |
| `Subs.CaptureScreenQR() ([]Subscription, error)` | 需用户先在界面上确认 |
| `Subs.Update(id string) error` / `UpdateAll() error` | 进度用 `subs:progress` 事件 |
| `Subs.Remove(id string)` / `Rename(id, name string)` / `SetAutoUpdate(id string, on bool, hours int)` | |
| `Subs.Export(id string, format 'clash'|'singbox'|'base64'|'uri') (string, error)` | |
| `Subs.ExportNode(nodeId string, format ...) (string, error)` | |
| `Subs.QRSvg(payload string, size int) (string, error)` | 返回 SVG 字符串 |
| `Subs.ShareDeepLink(payload string) ([]DeepLink, error)` | `DeepLink{ scheme, label, url }`，前端用系统浏览器打开 |
| `Subs.LanShareStart() (LanShare, error)` / `LanShareStop() error` / `LanShareStatus() LanShare` | `LanShare{ url, expiresAt, active }`；令牌仅含在 URL，默认 10 分钟/一次成功读取，分享全部节点及分组的 sing-box JSON，不含订阅访问地址 |

当前 P1 子集只实现 `Subs.List`、`Subs.AddFromText`、`Subs.Remove`，服务全名为 `ferry/internal/app.Subs`；其余方法仍是后续契约，前端不得提前调用。

- `AddFromText` 接受逐行 URI 或 base64 包装的 URI 列表；VLESS/Trojan 支持 TCP、WS 和受限 TLS，SS 支持三种 AEAD 方法，SOCKS 固定为 v5。未知参数的非空值、重复参数、损坏的 URL 编码、冲突的 TLS 参数明确拒绝，不静默更改节点行为。
- `skipped` 仅在导入返回值中包含；`List` 不重复保存导入报告。跳过原因不回显链接或凭据；跨订阅重复节点不新增，也不转移原订阅的所有权。
- 导入与删除须先停止内核。首次链接导入替换开发配置的节点与分组，保留入站/DNS 等设置；后续导入追加；`Core.Configure` 则清除分享链接订阅记录。
- 每次输入不超过 1 MiB，合并后的内核配置也不超过 1 MiB，最多 1024 个节点。失败时保留之前的订阅与配置。
- 当前记录仅在本次会话有效；手动导入的 `urlMasked` 为空，`autoUpdate=false`，流量配额与到期时间为未知（0）。不宣称持久化或自动更新已经实现。

### 3.3 Rules
| 方法 | 说明 |
|---|---|
| `Rules.List() ([]Rule, error)` | 含固定首末条 |
| `Rules.Save(r Rule) (Rule, error)` | id 为空即新建 |
| `Rules.SaveMany(rules []Rule) error` | 导入用：整批新建为一个事务，只应用一次；任一条无效则全部不写；传入的 id 不沿用 |
| `Rules.Remove(id string) error` | |
| `Rules.Reorder(ids []string) error` | 全量顺序 |
| `Rules.SetEnabled(id string, on bool) error` | |
| `Rules.Test(app, host string, port int, network string) (TestResult, error)` | `TestResult{ matchedRuleId, matchedRuleName, action, outboundTag, chain, stage }` |
| `Rules.CreateFromConnection(connId string) (Rule, error)` | 返回未保存的预填规则 |
| `Rules.Import(kind 'clash'|'singbox'|'ppx', content string) (ImportPreview, error)` | `ImportPreview{ rules []Rule, warnings []string }` |
| `Rules.Apply(ids []string) error` | 按完整 ID 列表重新应用；当前 Save/Remove/Reorder/SetEnabled 已事务保存并立即应用，失败恢复原配置 |
| `RuleSets.List(refresh bool) ([]RuleSet, error)` | `RuleSet{ id, name, url, updatedAt, sizeBytes, status }` |
| `RuleSets.Update(ids []string) error` | |
| `AppGroups.List() ([]AppGroup, error)` / `Save(g AppGroup) (AppGroup,error)` / `Remove(id string) error` | `AppGroup{ id, name, apps []string }`，空 ID 新建并返回生成 ID；引用 `@group:id`；保存事务应用，引用中的组不可删除 |

### 3.4 Connections / Logs
| 方法 | 说明 |
|---|---|
| `Connections.Pause(on bool) error` | 暂停推送 |
| `Connections.Close(id string) error` / `CloseAll() error` | |
| `Connections.Snapshot() ([]Connection, error)` | |
| `Logs.SetLevel(l string) error` | |
| `Logs.Pause(on bool) error` | |
| `Logs.Snapshot(limit int) ([]LogEntry, error)` | 环形缓冲，最多 5000 条 |
| `Logs.Export(path string) error` | 明文导出，需界面确认 |

### 3.5 Settings / System
| 方法 | 说明 |
|---|---|
| `Settings.Get() (Settings, error)` / `Settings.Set(s Settings) error` | 见下 |
| `System.SetSysProxy(on bool) error` | WinINet；退出时恢复原值 |
| `System.SetTun(on bool) error` | 失败返回可读原因 |
| `System.ServiceStatus() (ServiceStatus, error)` / `System.InstallService() error` / `System.UninstallService() error` / `System.RepairService() error` | 需提权时由主进程触发一次 UAC |
| `System.TestInbound() (int, error)` | 探测混合端口是否可用 |
| `System.OpenConfigDir() error` / `System.OpenLogDir() error` | |
| `System.ExportDiagnostics(path string) error` | 自动脱敏订阅 URL、密钥、内网 IP |
| `System.ExportBackup(path string) error` / `RestoreBackup(path string) error` | Windows 当前账户 DPAPI 加密备份；导出不覆盖既有文件，恢复事务应用；跨账户不支持 |
| `System.CheckUpdate(channel 'stable'|'beta') (UpdateInfo, error)` / `System.ApplyUpdate() error` | |
| `Window.SetLightweight(on bool) error` | 轻量模式：隐藏时销毁 WebView |
| `Update.GetState() (UpdateState, error)` | `UpdateState{ phase, version, progress, error }` |

```ts
interface Settings {
  autoStart: boolean
  startMinimized: boolean
  lightweight: boolean
  language: 'zh-CN' | 'en-US'
  theme: 'system' | 'dark' | 'light'
  mixedPort: number
  allowLan: boolean
  sysProxyOnStart: boolean
  tun: { stack: 'mixed'|'system'|'gvisor'; strictRoute: boolean; excludes: string[]; mtu: number; fakeIp: boolean }
  dns: { remote: string; direct: string; strategy: string }
  ruleSetAutoUpdate: boolean
  subscriptionUA: string
  subscriptionUpdateHours: number
  logLevel: 'debug'|'info'|'warn'|'error'
  updateChannel: 'stable'|'beta'
  clashApiMode: 'random'    // 固定为随机端口 + 随机密钥
}
```

## 4. 事件（Go → 前端）

| 事件名 | 负载 | 频率 |
|---|---|---|
| `core:state` | `CoreState` | 状态变化时 |
| `traffic` | `Traffic` | 1 Hz（窗口不可见时降到 0.2 Hz） |
| `connections:snapshot` | `{ items: Connection[]; added: Connection[]; updated: Connection[]; removed: string[] }` | 1 Hz，Go 侧做差量 |
| `logs:entry` | `LogEntry[]` | 批量，最多 20 条/次 |
| `nodes:latency` | `{ [nodeId]: { latencyMs: number; status: LatencyStatus } }` | 测速过程中 |
| `subs:progress` | `{ id: string; phase: 'fetch'|'parse'|'apply'|'done'|'error'; error?: string }` | 更新订阅时 |
| `rules:applied` | `{ version: number }` | 规则生效后（前端据此刷新） |
| `update:state` | `UpdateState` | 更新流程中 |

前端订阅方式：Wails v3 的 events API；页面不可见时取消订阅以省内存。

## 5. 生命周期与边界

1. 前端启动：调 `Status()` + `Snapshot()` 渲染首屏，再订阅事件。
2. 窗口关闭 → 隐藏到托盘（默认）；轻量模式下销毁 WebView，事件暂停推送，`Settings.Get()` 之外无调用。
3. 退出应用：停内核 → 恢复系统代理 → 停托盘。
4. 内核崩溃：后端自动重启（最多 3 次/5 分钟），每次通过 `core:state` 的 `error` 告知原因。
5. 前端严禁：直接 HTTP 调 Clash API、读订阅 URL 明文、访问 `C:\ProgramData\Ferry` 路径——这些都由 Go 侧封装。

## 6. 2026-11-13 规则接口实现范围

- Wails 服务名 `ferry/internal/app.Rules`，已注册 List/Save/Remove/Reorder/SetEnabled/Apply/Test/Import/CreateFromConnection。`System.Capabilities()` 增加 `rules`；前端接线尚未验收。
- 固定规则 ID：`builtin-lan`、`builtin-default`，只能位于首末且不可编辑、禁用、删除；旧空规则配置自动补齐。
- `Rule.TargetID="proxy"` 优先命中实际同名分组；没有同名分组时指向首个分组/合成默认组。内建默认规则始终指首个分组。
- `Rules.Test` 使用独立内核中的 sing-box 匹配器，测试规则模式，不发送网络请求。`stage="matched"` 为确定命中；`stage="dns-required"` 表示候选规则还需真实 DNS 地址，action/outboundTag 为空，不能渲染为确定命中。
- 默认局域网规则匹配已有 IP，不主动为全部域名做 DNS；用户 IP 规则可用 resolveBefore 请求解析。`*.example.com` 只匹配子域名，Clash DOMAIN-SUFFIX 导入同时保留根域名和子域名。
- Import 仅返回预览，不写入；不兼容字段整条跳过并附 warnings。Clash JSON/YAML/文本、sing-box 简单规则、Proxifier PPX RuleList 的受限子集可用；2026-11-14增加固定目录的Clash GEOIP/GEOSITE，复杂 logical/代理链等暂不支持。Ferry 应用分组可用，组内不允许再引用组；每规则展开最多1024应用、全配置最多16384。
- Save/Remove/Reorder/SetEnabled 完成后发送 `rules:applied`。重建失败或持久化失败会恢复旧配置与旧选中节点；无法确认内核状态时拒绝提交。

## 7. 2026-11-14 规则集接口

- `RuleSets.List(ctx, refresh bool)` 返回 `[{id,name,url,updatedAt,sizeBytes,status}]`，始终重新检查本地缓存，refresh不触发网络下载。
- `RuleSets.Update(ctx, ids []string)` 显式按固定目录下载；id仅限 `geosite-cn/geosite-non-cn/geoip-cn/geoip-private`，一批1–4项，无重复。失败保留旧缓存；每项原子更新，不承诺整批同时提交。
- 状态：missing/ready/updating/error/invalid；error且sizeBytes>0可能仍有可用旧缓存。
- 缓存更新不修改已运行内核，用户重启/重新应用规则后加载。规则测试使用当前校验后的本地缓存，可能反映比运行实例更新的快照。
- `System.Capabilities` 新增 `rulesets`。未公布system.service/system.tun，调用相应方法返回当前构建无受信任发行身份，不触发提权。

## 8. 2026-11-27 收尾接口补充

- `Subs.CaptureScreenQR(ctx)`：截取当前虚拟桌面并解码二维码（订阅 URL 返回来源确认提示，不自动拉取）；能力名 `qr.screen`。
- `Subs.ShareDeepLink(ctx, payload)` 与 `System.ParseDeepLink(ctx, raw)`：生成与解析 `ferry://`、`clash://`、`sing-box://` 深链；单实例转发触发 `deeplink:received` 事件（`DeepLinkPreview`），前端必须先向用户展示来源域名并确认；能力名 `deeplink`。
- `System.ServiceStatus(ctx)` / `System.TunStatus(ctx)` / `System.InstallService(ctx)` / `System.UninstallService(ctx)` / `System.RepairService(ctx)` / `System.SetTun(ctx, on)`：采用 Clash Verge Rev 风格的一次性 UAC（`ShellExecuteExW("runas")`）安装 `FerryCore` 服务，并以 ProgramData 管理员保护目录 + 二进制 SHA-256 + 256 位随机密钥 + 用户 SID/PID 双向管道校验替代强制 SPKI；桌面端公布 `system.service`、`system.tun`。
- `System.TestInbound(ctx)`、`System.OpenConfigDir(ctx)`、`System.OpenLogDir(ctx)` 与 `Window.SetLightweight(ctx, on)`：本地入站探测、目录打开与轻量模式；能力名 `system.tools`、`settings.automation`。
- `Update.GetState(ctx)`、`System.CheckUpdate(ctx, channel)`、`System.ApplyUpdate(ctx)` 与 `update:state` 事件：封装 ed25519 验签与原子暂存；未配置生产公钥/地址时返回未配置提示，仅配置生产源时公布 `update.online`。
