# P2 服务安全实施依据（2026-11-13）

本地对照源码：`github.com/sagernet/sing-box@v1.14.4/experimental/boxdd/{peer_windows.go,authenticode_windows.go,security_windows.go}`。这是只读研究，不代表 Ferry 已实现系统服务。

当前随机管道 + 当前用户 DACL + stdin bearer 仅适用于用户态子进程，不能直接作为 LocalSystem 服务协议。正式服务仍禁用，TUN 请求仍拒绝。

## 必须先固定的身份

- 服务以 LocalSystem 运行；安装由已提权进程执行；被授权使用服务的是原 GUI 用户 SID。三者不同，使用其它管理员凭据进行 UAC 时不能把安装进程 SID 当作 GUI 用户。
- 更新清单的 ed25519 公钥与 Authenticode 发布者身份是两套独立身份，目前都没有生产配置。
- 发行签名校验须 WinVerifyTrust 成功且 signer certificate 的 SPKI SHA-256 命中编译内置允许列表。任意系统信任的签名不足以证明是 Ferry；不使用 Subject/CN 模糊匹配。
- 缺失发行身份时，安装、服务运行、服务连接和 TUN 必须在产生副作用前拒绝。禁止环境变量、CLI、IPC 或用户配置绕过签名门禁。

## 本地上游核对结论

boxdd 不只检查文件名：客户端 PID → 进程句柄 → 镜像文件句柄 → 最终路径、文件 ID、签名；句柄保持到连接关闭。上游还支持特定自签名且关闭吊销检查，Ferry 不照搬这两个例外。

服务安装目录使用 Windows KnownFolder 的 ProgramFiles/Ferry 与 ProgramData/Ferry，不硬编码系统盘。打开文件时仅 FILE_SHARE_READ，拒绝共享写/删除；通过 GetFinalPathNameByHandle、GetFileInformationByHandle 对比真实路径与文件身份。拒绝 reparse points、不安全所有者和低权限写/删除/改 ACL。不能把 Go 的 0700 当作 Windows 受保护 DACL。

管道客户端授权掩码应移除 FILE_CREATE_PIPE_INSTANCE：`(FILE_GENERIC_READ|FILE_GENERIC_WRITE)&^FILE_APPEND_DATA` = `0x12019b`，客户端通过 DialPipeAccess 使用同一掩码；禁止给被授权普通用户 GA/通用写权限。go-winio 0.6.2 已带 FILE_PIPE_REJECT_REMOTE_CLIENTS，此外还需独占首次创建、身份握手/连接数上限及双向对端认证。

GUI 验证服务 PID 与 SCM 返回一致，并锁定、验证固定 ferry-core.exe 身份后才发配置。服务验证管道对端 PID、授权用户 SID、固定 Ferry.exe 和同一文件句柄签名。认证状态放内部连接上下文，不能由 HTTP header 自报。

WinVerifyTrustEx 状态中的 signer 可通过 WTHelperProvDataFromStateData、WTHelperGetProvSignerFromChain、WTHelperGetProvCertFromChain 提取；在 WTD_STATEACTION_CLOSE 前复制 DER，解析 x509 并验证 code-signing EKU 和 SPKI pin。有效时间戳交给 WinVerifyTrust，不能直接按证书当前过期时间误拒绝。

## 后续实现与验收

1. 发行身份门禁与无副作用失败测试。
2. 固定路径、句柄文件锁、DACL/reparse检查、真实签名提取与wrong-signer拒绝。
3. 用户/SYSTEM隔离的双向命名管道认证、连接限额和取消清理。
4. SCM 生命周期与安装器授权用户 SID 传递。
5. 有正式发行身份的受控环境测试真实安装/TUN/卸载和低权限攻击输入。

本地 x/sys/windows/testdata/ev-signed-file.exe 可作为只读签名提取测试样例，不能执行或安装证书。当前未测试 Authenticode 原生提取、SCM 或真实 TUN；不把测试替身通过当作这些能力可用。

2026-11-14更新：`internal/wintrust`已实现真实原生提取/信任+SPKI验证，x/sys样例在amd64与386实际通过，错误pin拒绝；未执行样例、未装证书。此处原生提取已验收，SCM、安装树ACL、管道双向认证、TUN仍未实现。当前`internal/release` pins为空，服务相关调用在无副作用门禁处拒绝。

2026-11-27更新（用户决策）：按用户要求采用 Clash Verge Rev 方式，移除对商业 Authenticode SPKI 证书的强制依赖，改为：
1. 一次性 UAC（`ShellExecuteExW("runas")`）调用 `ferry-core.exe install-service --user-sid <guiSID> --gui-exe <Ferry.exe>`，将服务二进制复制到管理员独占目录 `ProgramData\Ferry\service\ferry-core.exe`（防止便携目录被替换）。
2. 安装时写入管理员独占的 `service-meta.json` 与仅允许授权 GUI 用户只读的 `service-client\auth.json`，保存 256 位随机 IPC 密钥及 `Ferry.exe`/`ferry-core.exe` 的 SHA-256。
3. 管道仍坚持 `0x12019b` 受限 DACL（移除 `FILE_CREATE_PIPE_INSTANCE`）、`PIPE_REJECT_REMOTE_CLIENTS`、双向 PID + 用户 SID + `FILE_SHARE_READ` 锁定句柄 SHA-256 + Bearer Token 校验，且服务仅接收受限 RunSpec；若未来编译时配置了 `release.SignerPins()`，则自动叠加 WinVerifyTrust SPKI 校验。
