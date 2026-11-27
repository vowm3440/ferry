//go:build windows

package ipc

import (
	"errors"
	"os"
	"path/filepath"
	"unsafe"

	"golang.org/x/sys/windows"
)

const unsafeWriteMask uint32 = windows.GENERIC_ALL | windows.GENERIC_WRITE |
	windows.FILE_WRITE_DATA | windows.FILE_APPEND_DATA | windows.FILE_WRITE_ATTRIBUTES |
	windows.FILE_WRITE_EA | windows.DELETE | windows.WRITE_DAC | windows.WRITE_OWNER

const (
	sidSystem           = "S-1-5-18"
	sidAdministrators   = "S-1-5-32-544"
	sidCreatorOwner     = "S-1-3-0"
	sidTrustedInstaller = "S-1-5-80-956008885-3418522649-1831038044-1853292631-2271478464"
)

// VerifyInstallTree ensures the directory and every ancestor has no reparse
// points, is owned by a privileged principal, and grants no write/delete/ACL
// rights to non-privileged users.
func VerifyInstallTree(dir string) error {
	clean := filepath.Clean(dir)
	if !filepath.IsAbs(clean) {
		return errors.New("受保护目录必须为绝对路径")
	}
	if err := checkNoReparseChain(clean); err != nil {
		return err
	}
	sd, err := windows.GetNamedSecurityInfo(clean, windows.SE_FILE_OBJECT, windows.OWNER_SECURITY_INFORMATION|windows.DACL_SECURITY_INFORMATION)
	if err != nil || sd == nil {
		return errors.New("无法读取受保护目录安全描述符")
	}
	return verifySecurityDescriptor(sd)
}

func checkNoReparseChain(target string) error {
	current := target
	for {
		utf16, err := windows.UTF16PtrFromString(current)
		if err != nil {
			return errors.New("目录路径无效")
		}
		attrs, err := windows.GetFileAttributes(utf16)
		if err != nil {
			return errors.New("受保护目录不存在或不可访问")
		}
		if attrs&windows.FILE_ATTRIBUTE_REPARSE_POINT != 0 {
			return errors.New("拒绝包含符号链接或目录联接的受保护路径")
		}
		parent := filepath.Dir(current)
		if parent == current {
			return nil
		}
		current = parent
	}
}

func verifySecurityDescriptor(sd *windows.SECURITY_DESCRIPTOR) error {
	owner, _, err := sd.Owner()
	if err != nil || owner == nil || !isPrivilegedSID(owner.String()) {
		return errors.New("受保护目录所有者不是受信任的系统或管理员账户")
	}
	dacl, _, err := sd.DACL()
	if err != nil || dacl == nil {
		return errors.New("受保护目录缺少有效访问控制列表")
	}
	for i := uint32(0); i < uint32(dacl.AceCount); i++ {
		var ace *windows.ACCESS_ALLOWED_ACE
		if err := windows.GetAce(dacl, i, &ace); err != nil || ace == nil {
			return errors.New("无法解析受保护目录访问控制项")
		}
		if ace.Header.AceType != windows.ACCESS_ALLOWED_ACE_TYPE {
			continue
		}
		sid := (*windows.SID)(unsafe.Pointer(&ace.SidStart))
		sidStr := sid.String()
		if sidStr == sidCreatorOwner && (ace.Header.AceFlags&windows.INHERIT_ONLY_ACE) != 0 {
			continue
		}
		if !isPrivilegedSID(sidStr) && (uint32(ace.Mask)&unsafeWriteMask) != 0 {
			return errors.New("受保护目录对非特权账户开放了写入或修改权限")
		}
	}
	return nil
}

func isPrivilegedSID(sid string) bool {
	return sid == sidSystem || sid == sidAdministrators || sid == sidTrustedInstaller
}

// ProtectDataDir creates a service data directory if absent (rejecting reparse
// points) and applies a protected DACL allowing only SYSTEM and Administrators.
func ProtectDataDir(dir string) error {
	return applyProtectedDirSDDL(dir, "D:P(A;OICI;GA;;;SY)(A;OICI;GA;;;BA)")
}

// ProtectClientAuthDir grants SYSTEM/Administrators full control and the
// single authorized GUI user read-only access to retrieve the service IPC token.
func ProtectClientAuthDir(dir, authorizedUserSID string) error {
	sid, err := parseUserSID(authorizedUserSID)
	if err != nil {
		return err
	}
	return applyProtectedDirSDDL(dir, "D:P(A;OICI;GA;;;SY)(A;OICI;GA;;;BA)(A;OICI;GR;;;"+sid+")")
}

// ProtectClientReadFile applies a protected DACL granting SYSTEM/Administrators
// full control and the single authorized GUI user read-only access to the file.
func ProtectClientReadFile(filePath, authorizedUserSID string) error {
	clean := filepath.Clean(filePath)
	if !filepath.IsAbs(clean) {
		return errors.New("目标文件路径无效")
	}
	if err := checkNoReparseChain(clean); err != nil {
		return err
	}
	sid, err := parseUserSID(authorizedUserSID)
	if err != nil {
		return err
	}
	sddl := "D:P(A;;GA;;;SY)(A;;GA;;;BA)(A;;GR;;;" + sid + ")"
	return setProtectedDACL(clean, sddl, "无法应用受保护文件访问控制列表")
}

func applyProtectedDirSDDL(dir, sddl string) error {
	clean := filepath.Clean(dir)
	if !filepath.IsAbs(clean) {
		return errors.New("数据目录路径无效")
	}
	if err := os.MkdirAll(clean, 0o700); err != nil {
		return errors.New("无法创建受保护数据目录")
	}
	if err := checkNoReparseChain(clean); err != nil {
		return err
	}
	if err := setProtectedDACL(clean, sddl, "无法应用受保护目录访问控制列表"); err != nil {
		return err
	}
	if adminSID, err := windows.StringToSid(sidAdministrators); err == nil {
		_ = windows.SetNamedSecurityInfo(clean, windows.SE_FILE_OBJECT, windows.OWNER_SECURITY_INFORMATION, adminSID, nil, nil, nil)
	}
	return nil
}

func setProtectedDACL(target, sddl, applyErrMsg string) error {
	sd, err := windows.SecurityDescriptorFromString(sddl)
	if err != nil {
		return errors.New("无法构造受保护路径权限")
	}
	dacl, _, err := sd.DACL()
	if err != nil || dacl == nil {
		return errors.New("无法提取受保护路径权限")
	}
	flags := windows.SECURITY_INFORMATION(windows.DACL_SECURITY_INFORMATION | windows.PROTECTED_DACL_SECURITY_INFORMATION)
	if err := windows.SetNamedSecurityInfo(target, windows.SE_FILE_OBJECT, flags, nil, nil, dacl, nil); err != nil {
		return errors.New(applyErrMsg)
	}
	return nil
}
