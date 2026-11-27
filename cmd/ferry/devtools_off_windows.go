//go:build windows && !devtools

package main

import "github.com/wailsapp/wails/v3/pkg/application"

// 正式构建：不读取 FERRY_DEVTOOLS_PORT，永远不打开 WebView2 调试端口。
func devToolsOptions() application.WindowsOptions {
	return application.WindowsOptions{DisableQuitOnLastWindowClosed: true}
}
