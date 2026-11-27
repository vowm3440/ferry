# Ferry 实施状态与待办盘点（2026-11-27）

## 已完成能力概览

1. **双进程安全与瘦身内核（P0 / P2）**
   - `Ferry.exe`（Wails 3.1 + Svelte 5）与 `ferry-core.exe`（内嵌 `sing-box` 1.14.4）独立编译运行；`internal/core/boxinclude` 仅注册客户端白名单协议，剔除服务端入站、Tor、SSH、Snell、ACME 与 gRPC 守护服务，便携包体积控制在 ≤ 16 MB。
   - 支持用户态子进程模式（随机命名管道 + 匿名 stdin 传递 256 位令牌）与 `FerryCore` Windows 服务模式（一次性 UAC 安装至管理员独占 `ProgramData\Ferry\service` 目录，双向核验进程 PID、授权用户 SID、句柄锁定 SHA-256 与 256 位令牌，支持可选 SPKI 签名叠加校验）。
2. **订阅、自定义策略组与分享互操作（P1 / P4）**
   - 支持 VLESS、VMess、Trojan、Shadowsocks、Hysteria2、TUIC、AnyTLS、WireGuard、SOCKS、HTTP 节点的受限白名单解析、编译与双向导出（URI / Base64 / Clash YAML / sing-box JSON / SIP008）。
   - 支持 URL 订阅定时更新、**单订阅独立 `User-Agent` 覆盖与继承**、**自定义策略分组（`select` / `urltest`）可视化增删改与防环/引用校验**、节点延迟测试、二维码生成与图片/屏幕二维码识别、一次性令牌局域网分享，以及 `ferry://` / `clash://` / `sing-box://` 深链确认导入与**便携版免 UAC 当前用户（`HKCU\Software\Classes\ferry`）深链协议注册开关**。
3. **规则系统、遥测与系统集成（P1 / P3 / P5）**
   - 支持应用/域名/IP/端口/网络匹配规则 CRUD、批量事务导入（Clash / sing-box / Proxifier PPX）、应用分组（`@group:id`）、离线规则测试器，以及 4 个内置 `geosite` / `geoip` SRS 规则集下载校验与后台自动刷新。
   - 支持设置页 **`tun.excludes`（IPv4/IPv6 CIDR）可视化增删、常用局域网预设填充与校验**。
   - 支持实时流量、连接差量快照与关闭、日志环形缓冲与明文导出、系统代理读写与异常恢复、开机自启、最小化启动、DPAPI 加密备份恢复与脱敏诊断包导出。
4. **真轻量 WebView2 销毁重建、应用内更新闭环与全量双语国际化（P5）**
   - **轻量模式（`Lightweight`）**：支持 `DisableQuitOnLastWindowClosed` 下的窗口控制器生命周期管理；最小化启动且开启轻量模式时不提前创建 WebView2，关闭主窗口或隐藏状态下开启轻量模式时真实销毁 WebView2 进程释放内存，托盘点击或深链唤醒时按需重建。
   - **应用内更新闭环**：支持 `ed25519` 签名清单校验、SHA-256 校验下载暂存（保留 `.exe` / `.zip` 扩展名），以及 `System.InstallUpdate` 启动前二次大小与常量时间 SHA-256 防篡改复核、静默拉起安装器（`/S`）与优雅退出。
   - **全量中英文双语（`zh-CN` / `en-US`）**：覆盖全部 7 个页面组件、对话框、纯 TS 视图格式化辅助模块以及 Windows 系统托盘右键菜单，切换语言即时联动生效。

## 发行配置与后续实机验收说明

1. **在线更新源与签名公钥已配置**：
   - `internal/release/update.go` 已配置 GitHub Releases 更新源 `https://github.com/vowm3440/ferry/releases/latest/download/manifest-{channel}.json` 与项目专属 `ed25519` 公钥（`Nw03fbVL3Yyy1b5YFUZUCJNUa6re558iL0NFjj+aSdU=`）。
   - 配套签名脚本 `scripts/sign-release.go` 支持一键生成 `manifest-<channel>.json` 与 `.sig` 分离签名文件；对应私钥仅离线保存在本地 `.keys/`（已排除于 Git 与源码归档之外）。
2. **安装包、实机稳定性闭环与自动化流水线已就绪**：
   - `scripts/build.ps1` 统一通过 `-ldflags` 向 `internal/release.Version` 注入版本号；`scripts/test.ps1` 完整串联前端检查/单测/无头冒烟测试、Go 源码规范、双标签单测与真实子进程崩溃恢复验证；`scripts/verify-machine.ps1` 提供实机一键自检。
   - `Router` 支持退出时优雅停机服务态内核、服务 PID 变更触发自动恢复与跨模式连续日志游标；二次启动直接唤醒已驻留托盘窗口；应用内更新通过 `ShellExecuteW` 拉起管理员安装包，NSIS 安装脚本自动停旧进程并调用 `ferry-core.exe refresh-installed-service` 同步服务二进制与 GUI 哈希。
   - `.github/workflows/ci.yml` 与 `.github/workflows/release.yml` 已配置 `windows-latest` 自动构建、全量测试、打包与 `ed25519` 签名发布流水线。
3. **日常发版人工核对**：
   - 每次大版本发布前建议在实机核对 `FerryCore` 系统服务 + TUN 网卡接管及第三方客户端配置互导。
