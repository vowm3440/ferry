//go:build windows

package main

import (
	"errors"
	"golang.org/x/sys/windows"
	"golang.org/x/sys/windows/svc"
)

// A manually registered SCM service must never gain the user-mode pipe path.
// Authenticated service hosting remains unavailable until P2 is implemented.
func requireUserProcess() error {
	user, err := windows.GetCurrentProcessToken().GetTokenUser()
	if err != nil {
		return errors.New("无法验证内核运行身份")
	}
	service, err := svc.IsWindowsService()
	if err != nil {
		return errors.New("无法验证内核进程类型")
	}
	return validateUserIdentity(user.User.Sid.String(), service)
}

func validateUserIdentity(sid string, service bool) error {
	if service || sid == "S-1-5-18" || sid == "S-1-5-19" || sid == "S-1-5-20" {
		return errors.New("此开发构建未启用受信任系统服务，请由用户界面启动")
	}
	return nil
}
