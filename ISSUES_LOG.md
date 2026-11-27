# Issues Log

## 2026-11-27 — v2rayN 复制的 VLESS Reality 链接导入失败（`含暂不支持的参数 spx`）

- **症状（Symptom）**：
  从 v2rayN 复制 VLESS + Reality + Vision 分享链接粘贴至「订阅 → 粘贴内容」导入时，界面提示：
  `没有可导入的节点：第 1 行含暂不支持的参数 spx`。
- **复现步骤（Reproduction）**：
  粘贴标准 v2rayN 导出的 VLESS Reality 链接（含 `&security=reality&...&pbk=...&sid=...&spx=%2F&type=tcp&headerType=none`）并点击「导入」。
- **根因（Root Cause）**：
  1. Xray-core / v2rayN / 3x-ui 导出的 VLESS Reality 链接默认附带 `spx`（`spiderX`，Xray 客户端鉴权失败时的伪装爬虫起始路径），而 `sing-box` 的 `OutboundRealityOptions` 仅需 `enabled`、`public_key`、`short_id`，不使用 `spiderX`（不影响握手或转发语义）。`internal/convert/links.go` 的 `parseVLESS` 白名单未包含 `spx`，导致被 `unsupportedParam` 拦截。
  2. 同类互操作细节中，v2rayN 新版还可能导出 `type=raw`（纯 TCP 别名）、gRPC `mode=gun`，部分面板导出的 `pbk` 含 Base64 填充或标准字符集、`sid` 为奇数位十六进制；Clash Reality YAML 也常携带 `flow` 与 `reality-opts.spider-x`。
- **解决方案（Solution）**：
  1. `internal/convert/query.go` & `internal/convert/links.go`：允许 Reality 节点携带合法非控制字符的 `spx`（非 Reality 启用时仍严格拒绝），兼容 `type=raw` 与 gRPC `mode=gun`，并将合法 32 字节 Base64 `pbk` 与奇数位十六进制 `sid` 规范化为 `sing-box` 所需格式。
  2. `internal/convert/clash.go` & `internal/convert/export_clash.go`：支持 VLESS `flow` 与 `reality-opts.spider-x` 导入及无损导出，并在缺省 `client-fingerprint` 时自动补齐 Reality 必需的 `utls`（`chrome`）。
  3. 补充 `protocols_test.go`、`links_boundary_test.go`、`documents_boundary_test.go`、`export_test.go` 与真实 `sing-box` 内核构造测试 `internal/core/compile/protocols_test.go`。
- **失败尝试（Failed Attempts）**：
  无（TDD 先行复现红灯后一次修复通过）。
- **预防措施（Prevention）**：
  在协议白名单测试中保留第三方主流客户端（v2rayN / Clash.Meta / Mihomo）真实分享链接样例，确保客户端专属非协议面参数在严格边界校验下安全兼容。

## 2026-11-27 — 安装 UAC 服务后内核重启失败、手动结束内核后 TUN 仍提示安装服务

- **症状（Symptom）**：
  1. 在界面通过 UAC 安装 `FerryCore` 系统服务后，重启/启停内核失败并提示 `无法连接内核，请重新启动` / `内核连接已断开`。
  2. 手动在任务管理器结束 `ferry-core.exe` 进程后重新打开 Ferry，设置页显示已安装 UAC 服务，但概览页「TUN 模式」与开启 TUN 时仍提示安装服务。
- **复现步骤（Reproduction）**：
  1. 在普通用户权限启动 `Ferry.exe`，点击安装系统服务并确认 UAC。
  2. 服务启动后尝试重启内核；或手动结束 `ferry-core.exe` 后重新打开 `Ferry.exe` 并切换 TUN。
- **根因（Root Cause）**：
  1. `internal/ipc/peer_windows.go` 中 `WinPeerVerifier.VerifyServer` 由普通用户权限的 `Ferry.exe` 调用，去对 `LocalSystem` 权限的 `FerryCore` 服务执行 `OpenProcess(PROCESS_QUERY_LIMITED_INFORMATION)` / `OpenProcessToken(TOKEN_QUERY)`，同时读取受 `SY/BA` 限制的 `C:\ProgramData\Ferry\service\ferry-core.exe`，在 Windows 上双双触发 `ERROR_ACCESS_DENIED (5)`；`Router.serviceClient()` 又仅凭 SCM 状态 `Running` 盲目缓存并关闭用户态子进程，未做管道健康探测，导致内核调用全部失败。
  2. `FerryCore` 服务未配置 SCM 故障自动恢复（`SetRecoveryActions`）与授权用户的 `SERVICE_START` 权限，结束 `ferry-core.exe` 后服务停留在 `STOPPED (1067)`；同时概览页 `ProxyTiles.svelte` 与 `System.SetTun` 仅检查 `installed && running`，未运行即误导用户“安装服务”；且 `winsvc.Install` 遇到服务已存在时直接拒绝。
  3. 安装/修复服务后缺少停止旧用户态内核及自动重启 `Ferry.exe` 与服务态内核的闭环（直接重启也会被单实例互斥锁拦截）。
- **解决方案（Solution）**：
  1. `internal/ipc/install_tree_windows.go`、`peer_windows.go`、`service_auth_windows.go`、`service_status_windows.go`、`router_windows.go`：新增 `ProtectClientReadFile` 授予授权用户对 `ProgramData\Ferry\service\ferry-core.exe` 的只读 ACL；`VerifyServer` 改为基于命名管道内核 `GetNamedPipeServerProcessId == expectedPID` 与锁定 SHA-256 校验（不再对 `LocalSystem` 服务调用 `OpenProcess`）；`ClientAuth` 绑定 GUI 指纹；`Router` 增加 800ms 管道握手健康探测、冷却与免 UAC 唤醒；`InspectService` 准确区分 `running`、`stopped` 与 `needsRepair`。
  2. `internal/core/winsvc/winsvc_windows.go`：将 `Install` 改为幂等覆盖安装，配置 SCM 崩溃/非零退出自动重启，并授予授权用户 `SERVICE_START` 权限。
  3. `internal/instance/lock_windows.go`、`internal/app/service.go`、`service_windows.go`、`cmd/ferry/main_windows.go`：新增 `AcquireWithTimeout`、`SetTun` 自动唤醒已安装服务、以及 UAC 安装/修复服务前先停旧内核、成功后自动释放互斥锁并重启 `Ferry.exe` 与服务态内核。
  4. 前端 `ProxyTiles.svelte`、`TunRows.svelte`、`HeroCard.svelte`、`UsageCard.svelte`、设置页各子组件与订阅页：修正已安装服务未运行/需更新时的 TUN 卡片展示与一键启动/修复，并精简界面冗余小字。
- **失败尝试（Failed Attempts）**：
  1. 初版 `TestClientAuthRoundTripWithGUIHash` 在非提权单测中直接调用 `WriteServiceAuthFiles` 向受 `SY/BA` 保护的目录写文件触发拒绝访问，改为序列化 `clientAuthForMeta` 后在用户临时目录验证 `ReadClientAuth` 往返解析与向后兼容。
  2. `TestServiceAndTunStatusAndIsolatedRegistryAutoStart` 在已安装 `FerryCore` 的宿主机上暴露出 `ctrl == nil` 时 `ServiceStatus` 透传宿主机 SCM 的状态隔离缺陷，已修正为无 `ServiceController` 时返回未安装状态。
- **预防措施（Prevention）**：
  1. 所有普通用户态与 `LocalSystem` 服务的 IPC 校验必须仅依赖内核管道对象属性（`GetNamedPipeServerProcessId`）与显式授予只读 ACL 的受保护文件，禁止跨权限边界 `OpenProcess`/`OpenProcessToken`。
  2. 保持服务态与用户态内核的路由探测与回退单测、互斥锁交接单测以及前端多分辨率冒烟测试常绿。

## 2026-11-27 — 工程自动化与实机稳定性闭环加固

- **症状（Symptom）**：
  1. 实机链路隐患：服务模式下退出 `Ferry.exe` 未停止 `FerryCore` 内核导致端口/TUN 残留；服务异常重启后 `Router.Exited()` 未通知上层恢复配置且日志游标重置卡死；托盘驻留时二次启动弹出“无法加载配置”；应用内更新以 `CreateProcess` 启动管理员安装包易报 740 错误；NSIS 覆盖升级未停旧进程且未同步更新服务模式哈希。
  2. 工程自动化断层：版本号多处硬编码；`scripts/test.ps1` 未跑前端单测与无头冒烟；缺少实机自检脚本与 GitHub Actions CI/Release 流水线。
- **解决方案（Solution）**：
  1. `internal/release/version.go` 统一版本号来源，由 `scripts/build.ps1` 通过 `-ldflags -X` 注入全二进制。
  2. `internal/ipc/router_windows.go` & `router_lifecycle_windows.go` & `exited_windows.go`：实现 `Router.Close` 主动停机服务内 `sing-box`、服务 PID 切换/断开检测、`Exited()` 触发自动恢复，以及跨模式/重启单调递增的日志游标重定基。
  3. `internal/instance/deeplink_windows.go` & `cmd/ferry/{main,session}_windows.go` & `internal/app/connections.go`：支持 `ActivateToken` 二次启动唤醒主窗口、友好提示框、以及停止态跳过无效遥测轮询并在停机时同步清理连接缓存。
  4. `internal/store/store.go` 增加瞬时文件锁重试；`internal/app/updater_windows.go` 使用 `ShellExecuteW` 拉起管理员更新包；`internal/core/winsvc/winsvc_windows.go` + `cmd/ferry-core` + `build/installer.nsi` 实现覆盖安装自动停进程与 `refresh-installed-service` 服务同步。
  5. 升级 `scripts/build.ps1`、`scripts/test.ps1`、`scripts/package.ps1`，新增 `scripts/verify-machine.ps1` 与 `.github/workflows/{ci,release}.yml`。
- **预防措施（Prevention）**：
  通过 `scripts/test.ps1`、`scripts/verify-machine.ps1` 与 GitHub Actions CI 强制门禁，确保每次改动同时通过前端单测/无头冒烟、Go 双标签测试、真实 `ferry-core.exe` 子进程压力恢复测试与打包校验。
