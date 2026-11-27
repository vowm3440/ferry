# Ferry 技术方案与实现计划

> 代号 Ferry（渡）。Windows 专用、基于 sing-box 内核的代理客户端。
> 本文是给实现者（Codex）的唯一上下文来源；UI 以 `docs/design/mockup.html` 为准。
> 状态：**用户已于 2026-11-11 确认，P0 已验收，P1 分享链接导入已接续**。前端由 Fable 编写，Go 后端由 Codex 编写；当前实施边界见 `P1.md`。

## 0. 已实测的事实（2026-11-09，本机，`.spike/` 下）

| 项 | 结果 |
|---|---|
| sing-box 最新稳定版 | v1.14.4（预发布 v1.15.0-alpha.13） |
| 内嵌构建（`CGO_ENABLED=0`，tags `with_gvisor,with_quic,with_utls,with_clash_api,with_wireguard`） | 43.3 MB exe，xz 后 11.0 MB；冷编译 18 s，热编译 13 s |
| 内嵌启动耗时 | `box.New`+`Start` 共 68 ms（mixed 入站 + Clash API） |
| 内核常驻内存 | 工作集 27.9 MB（有流量后） |
| Wails | v3.1.0 已是正式版；Svelte+TS 模板空壳 10.7 MB，xz 后 3.6 MB |
| 开发磁盘 | Go 模块缓存 +0.85 GB，构建缓存约 1.5 GB（可 `go clean -cache`），`node_modules` 74 MB |
| wintun | sing-tun 以 `go:embed` 内置 wintun 0.14.1 并内存加载，无需随包 DLL |
| `process_name` | 区分大小写的 basename 精确匹配 → Windows 上必须编译成 `(?i)` 正则 |
| 进程查找 | 对 TUN 与本机 mixed 入站都生效 |
| Clash API | `/connections`、`/logs`、`/traffic` 可用；providers 是空实现 |
| 弃用项 | 距移除一个小版本内的弃用配置直接报错 → 编译器只产出当前写法 |
| 官方参考 | 1.14 起有 `experimental/boxdd`（Windows 服务 + 命名管道对端校验 + Authenticode），作为服务安全设计的对照实现 |

未完成的调研（代理中断，实现前由 Codex 补做并写入 `docs/research/`）：
Clash Verge Rev 服务历史漏洞的逐条复盘；各分享链接格式的字段级规范。

## 1. 技术选型

- **语言**：全栈 Go 1.26，无 CGO。sing-box 作为库内嵌，不走子进程转发，数据面零额外开销。
- **GUI 壳**：Wails v3.1（系统 WebView2）。不选 Tauri（Rust target 目录数 GB 起）、不选 Electron（体积）。
- **前端**：Svelte 5 + TypeScript + Vite，手写 CSS token，无 UI 框架；图标内联 SVG。
- **安装包**：NSIS（LZMA solid），预计 15 MB 左右；另出便携版 zip。不使用 UPX（杀软误报）。
- **不启用的 tag**：`with_tailscale`、`with_naive_outbound`（需 libcronet.dll）、`with_acme` 等服务端功能。

## 2. 进程与权限架构

```
Ferry.exe（用户权限，GUI + 托盘 + 业务逻辑，不链接 sing-box）
   │  HTTP over 命名管道 \\.\pipe\ferry-core   → 控制面（RunSpec）
   │  HTTP 127.0.0.1:<随机端口> + 256 位密钥    → Clash API 实时数据
   ▼
ferry-core.exe（内嵌 sing-box）
   ├─ 服务模式：LocalSystem Windows 服务（TUN 免提权）
   └─ 子进程模式：用户权限（未装服务时，仅系统代理）
```

- 服务一次性安装（安装器或应用内按钮，一次 UAC），之后开关 TUN 不再弹窗。
- 服务二进制固定在 `C:\Program Files\Ferry\`，数据在 `C:\ProgramData\Ferry\`（受保护 DACL）。
- 服务已安装时内核始终跑在服务里，避免两套运行态。

### 2.1 服务安全边界（硬性要求）

1. 管道 DACL 只放行 SYSTEM、Administrators、安装者 SID；`PIPE_REJECT_REMOTE_CLIENTS`。
2. 对端校验：取管道客户端 PID → 镜像路径必须是安装目录下的 `Ferry.exe`，并校验签名（纵深防御，参考 boxdd `peer_windows.go`）。
3. 服务**不接受原始 sing-box JSON**，只接受 RunSpec（见 §3）。由服务端编译成 `option.Options`。
4. 服务不接受任何文件系统路径：资源一律用逻辑名（`ruleset://geosite-cn`），在服务自己的数据根内解析。
5. 节点的原始出站片段过净化器：按 §2.2 的字段清单拒绝路径类字段、UNC 路径、可执行类字段。
6. 服务永不执行外部程序，不提供“运行任意二进制/更新自身”的接口；更新由安装器完成。
7. 所有请求有大小上限、超时、并发上限；错误信息不回显内部路径。

### 2.2 路径类字段清单（v1.14.4 `option/` 实测）

`path`（文件语义处）、`certificate_path`、`key_path`、`client_certificate_path`、`client_key_path`、
`data_directory`、`wrapper_path`、`usages_path`、`static_key_path`、`crl_path`、`credential_path`、
`config_path`、`cache_file`、`taildrop_directory`、`state_directory`、`secret_path`、`protect_path`、
`private_key_path`、`pid_file`、`mesh_psk_file`、`mca_key_path`、`mca_certificate_path`、
`executable_path`、`directory`、`dhcp_lease_files`、`certificate_directory_path`、
`certificate_authority_path`、`cache_path`。
净化器采用**白名单**：只允许已知出站类型的已知字段；上表用于测试“必须被拒绝”。
升级 sing-box 时重跑清单脚本，差异进入评审。注意 `path` 在 ws/http 传输里是 URL 路径，属合法字段。

## 3. RunSpec（GUI → 内核的中间表示）

```
RunSpec {
  nodes:    [{ id, tag, outbound: <净化后的 sing-box outbound JSON> }]
  groups:   [{ id, name, type: select|urltest, members, testUrl, interval }]
  rules:    [Rule]            // §5
  dns:      { mode: fakeip|redirhost, remote, direct, strategy }
  inbound:  { mixedPort, allowLan, tun: { enabled, stack, strictRoute, excludes[], mtu } }
  mode:     rule|global|direct
  log:      { level }
}
```

编译器（`internal/core/compile`）是纯函数：`RunSpec → option.Options`，golden 测试 + `box.New` 校验。
切换节点、模式走 Clash API，不重启内核；规则/入站变更才重建实例。

## 4. TUN

- sing-box tun 入站：`stack: mixed`、`auto_route`、`strict_route`、`auto_detect_interface`。
- 路由规则前置 `sniff` 与 `hijack-dns` 动作；默认排除局域网/组播网段，可编辑。
- DNS：默认 fakeip（可切真实 IP），直连域名走国内 DNS，其余走代理 DNS，防泄漏。
- 失败回退：TUN 启动失败时保持系统代理模式并给出可读原因（适配器冲突、服务未运行等）。
- 系统代理由 GUI 侧用 WinINet 设置；记录原值，崩溃后下次启动恢复。

## 5. 规则系统（Proxifier 语义）

```
Rule { id, name, enabled, apps[], hosts[], ports[], network: any|tcp|udp, action }
action = direct | block | proxy(groupId|nodeId)
```

- 字段内为“或”，字段间为“且”，空字段表示任意；自上而下首条命中生效。
- 固定首条“本机/局域网”，固定末条“默认规则”。
- 应用分组（如“浏览器”“游戏平台”）可复用；提供场景模板。

编译到 sing-box：

| 写法 | 产物 |
|---|---|
| `chrome.exe` | `process_path_regex: (?i)\\chrome\.exe$` |
| `*\Riot Games\*.exe`、完整路径 | `(?i)` 的 `process_path_regex` |
| `example.com` / `*.example.com` | `domain` / `domain_suffix` |
| 其他通配 | `domain_regex` |
| `10.0.0.1-10.0.0.99`、CIDR | `ip_cidr`（区间拆分） |
| `80`、`8000-9000` | `port` / `port_range` |
| `geosite:x`、`geoip:x` | 本地 `rule_set`（SRS） |

- IP 类规则前自动插入 `resolve` 动作（可按规则关闭）。
- 规则集由内核宿主下载（多镜像、解析校验、原子替换），启动只读本地文件，离线可启动。
- 规则测试器：输入应用/主机/端口，用 sing-box 自身的规则构造器求值，返回命中规则与出站。
- 连接页“据此创建规则”：从一条连接预填应用与主机。
- 导入：Clash rules、Proxifier `.ppx`、sing-box route rules。

## 6. 订阅、二维码、分享

- **导入格式**：Clash/Mihomo YAML、sing-box JSON、base64 链接集、明文 URI 列表、SIP008。
  URI：ss、vmess、vless、trojan、hysteria2、tuic、wireguard、anytls 等。
- 解析响应头 `subscription-userinfo`、`profile-update-interval`、`profile-title`；UA 可配置。
- 防护：响应体大小上限、gzip 炸弹上限、YAML 别名上限、超时；`http://` 订阅给出警告。
- 订阅 URL 用 DPAPI 加密落盘；日志与诊断包中脱敏。
- 深链（`ferry://`、`clash://`、`sing-box://`）导入**必须经用户确认**，显示来源域名。
- **二维码**：剪贴板图片、图片文件、屏幕截取（gozxing + kbinani/screenshot）；生成用 rsc.io/qr → SVG。
- **分享**：导出 Clash YAML / sing-box JSON / base64 / 单节点 URI；二维码；其他客户端深链。
  局域网分享服务默认关闭，开启时带一次性令牌、自动过期、限速。

## 7. 目录结构

```
cmd/ferry/            GUI 入口（Wails）
cmd/ferry-core/       内核宿主（service / run 子命令）
internal/ipc/         命名管道传输、对端校验、客户端
internal/spec/        RunSpec 类型与校验
internal/convert/     订阅与 URI 解析、导出
internal/store/       配置与状态持久化（JSON + 原子写 + DPAPI）
internal/core/{host,compile,policy,rulesets,winsvc}
internal/app/{subs,nodes,rules,share,qr,sysproxy,tray,settings,updater}
frontend/             Svelte 5（Claude 负责）
build/                NSIS、图标、清单
docs/{plan,design,research}/   testdata/   scripts/
```

依赖方向：`cmd/ferry` 不得 import `internal/core/*` 与 sing-box（CI 用 `go list -deps` 断言）。

## 8. 前端（Claude 负责）

- 页面：概览、节点、订阅、规则、连接、日志、设置；深浅色；自绘标题栏。
- 前端只通过 Wails 绑定调用 Go，不直连 Clash API、拿不到密钥。
- 连接列表由 Go 侧 1 Hz 做差量后推送；列表虚拟滚动。
- 轻量模式：窗口隐藏到托盘时销毁 WebView，只留托盘与内核。
- 绑定接口契约放在 `docs/plan/api.md`（P0 由 Codex 起草，Claude 评审），前后端按契约并行。

## 9. 分期

| 阶段 | 内容 | 验收 |
|---|---|---|
| P0 | 仓库脚手架、两个二进制骨架、IPC、RunSpec、编译器最小集、接口契约 | 子进程模式跑通一个节点，`go test ./...` 通过 |
| P1 | 订阅导入与更新、节点与分组、测速、系统代理、概览/日志/连接、托盘 | 代理模式可日常使用 |
| P2 | Windows 服务、对端校验、净化器、TUN、规则集下载 | 标准用户开关 TUN 无 UAC；安全用例全绿 |
| P3 | 规则 IR、编译、测试器、导入器、应用分组 | 规则 golden 测试；`.ppx` 样例导入 |
| P4 | 二维码导入、导出与分享、深链 | 与 Clash Verge Rev、sing-box 官方客户端互导 |
| P5 | 更新器（ed25519 签名校验）、NSIS、便携版、备份、诊断脱敏、中英文 | 安装包 ≤ 20 MB |

## 10. 质量要求

- TDD，覆盖率 ≥ 80%；转换器用表驱动 + fuzz；编译器 golden + `box.New`；净化器按 §2.2 清单。
- 文件 ≤ 800 行、函数 ≤ 50 行；错误显式处理，不吞错；边界处校验输入。
- 约定式提交；不提交密钥；`.spike/` 不入库。
- 许可证：sing-box 为 GPL-3.0，本仓库整体 GPL-3.0；名称不得暗示与 sing-box 官方关联。
