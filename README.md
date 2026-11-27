# Ferry（渡）

Windows 专用轻量代理客户端，采用 Go 1.26 + Wails 3.1 + Svelte 5 架构，通过独立内核宿主进程内嵌 `sing-box` 1.14.4。本项目按 `GPL-3.0-or-later` 开源，与 `sing-box` 官方无隶属关系。

完整架构设计见 [PLAN.md](docs/plan/PLAN.md)，前后端绑定契约见 [api.md](docs/plan/api.md)，当前实施与验收状态见 [STATUS.md](docs/plan/STATUS.md)。

## 构建与验证

环境要求：Windows 10/11 x64、Go 1.26.2、Node.js 24、pnpm 11，以及系统 WebView2（无需 CGO / C 编译器）。

```powershell
go mod download
.\scripts\build.ps1
.\scripts\test.ps1
.\bin\Ferry.exe
```

- `scripts\build.ps1`：构建前端静态资源（自动剔除仅浏览器调试使用的演示 chunk），注入版本号（`-Version`）并编译精简版 `bin\ferry-core.exe`（仅注册客户端所需协议）与 `bin\Ferry.exe`（零内核依赖）。
- `scripts\test.ps1`：一键执行构建、前端类型检查 / 单元测试 / 无头浏览器全页面冒烟测试、Go 源码尺寸检查（单文件 ≤ 800 行、单函数 ≤ 50 行）、`go vet`、全标签与默认标签测试（含真实 `ferry-core.exe` 子进程崩溃恢复与端口释放压力测试），以及 GUI 零 `sing-box` 依赖断言。
- `scripts\verify-machine.ps1`：非破坏性实机自检脚本，核对二进制版本与体积、WinINet 系统代理状态、`FerryCore` 系统服务故障恢复与指纹一致性，并运行真实子进程握手与端口释放闭环检测。
- `scripts\package.ps1 -Version 0.2.0`：执行全量测试门禁后自动调用 NSIS（`makensis`）编译 `Ferry-0.2.0-windows-amd64-setup.exe` 安装程序（覆盖升级时自动停旧进程并同步刷新已安装的 `FerryCore` 服务），同时生成便携版压缩包、GPL 对应源码归档与 SHA-256 校验文件。
- `go run .\scripts\sign-release.go` 与 `.github\workflows\{ci,release}.yml`：支持本地签名与 GitHub Actions 自动 CI 验证及 tag 自动打包签名发布。

## 核心特性

- **双进程隔离与真轻量模式**：`Ferry.exe` 运行于普通用户权限，通过命名管道控制 `ferry-core.exe`（`cmd/ferry` 不链接任何 `sing-box` 代码）；轻量模式下关闭窗口真实销毁 WebView2 释放渲染进程内存，托盘/深链唤醒时按需重建。
- **协议、订阅与自定义分组**：支持 VLESS（含 Reality / uTLS / WS / gRPC / HTTPUpgrade）、VMess、Trojan、Shadowsocks AEAD、Hysteria2、TUIC、AnyTLS、WireGuard、SOCKS、HTTP；支持单订阅独立 `User-Agent`、自定义 `select` / `urltest` 策略组增删改、二维码图片/屏幕识别、一次性局域网分享，以及便携版免 UAC `ferry://` 深链协议注册与确认导入。
- **规则与分流**：Proxifier 风格规则编辑与排序、应用分组（`@group:id`）、离线规则测试器、Clash / sing-box / PPX 规则导入预览，以及内置 `geosite` / `geoip` SRS 缓存管理。
- **系统代理与服务模式 TUN**：WinINet 系统代理接管与崩溃自动恢复；支持一次性 UAC 安装 `FerryCore` 系统服务（ProgramData 受保护目录 + 句柄锁定 SHA-256 + 256 位随机密钥 + 用户 SID/PID 双向管道校验），免 UAC 切换 TUN 并支持可视化编辑 `tun.excludes` 排除网段。
- **安全存储、应用内更新与双语界面**：配置与凭据使用当前 Windows 账户 DPAPI 加密落盘；支持 `ed25519` + SHA-256 双重校验的应用内更新安装闭环、同账户加密备份恢复、脱敏诊断导出，以及全界面与系统托盘的简体中文 / English 即时切换。
