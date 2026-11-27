//go:build windows && devtools

package main

import (
	"os"
	"strconv"

	"github.com/wailsapp/wails/v3/pkg/application"
)

// devToolsOptions 是只供本机联调使用的开关：设置 FERRY_DEVTOOLS_PORT 后，
// WebView2 会打开本机 CDP 调试端口，让 frontend/tests/e2e/real-app.mjs 能驱动真实界面。
// 不设置时返回零值，行为与正式运行一致。WebView2 只监听 127.0.0.1，且调试端口仅用于本机联调。
//
// 调试端口没有任何鉴权：连上它的本机进程可以调用界面能调用的一切（包括导出节点凭据）。
// 所以这段代码只在带 devtools 构建标签时编译（scripts/build.ps1 -DevTools 生成
// bin/Ferry-devtools.exe），正式构建里不存在这个开关，见 devtools_off_windows.go。
func devToolsOptions() application.WindowsOptions {
	port, err := strconv.Atoi(os.Getenv("FERRY_DEVTOOLS_PORT"))
	if err != nil || port < 1024 || port > 65535 {
		return application.WindowsOptions{DisableQuitOnLastWindowClosed: true}
	}
	return application.WindowsOptions{
		DisableQuitOnLastWindowClosed: true,
		AdditionalBrowserArgs:         []string{"--remote-debugging-port=" + strconv.Itoa(port)},
	}
}
