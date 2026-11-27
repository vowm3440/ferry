//go:build windows

package ipc

import (
	"errors"
	"path/filepath"
	"strings"

	"golang.org/x/sys/windows"
)

const (
	ServiceName     = "FerryCore"
	ServicePipeName = `\\.\pipe\ferry-core`
	// (FILE_GENERIC_READ|FILE_GENERIC_WRITE) &^ FILE_APPEND_DATA removes FILE_CREATE_PIPE_INSTANCE.
	ServicePipeAccessMask = 0x12019b
)

// InstallPaths holds the fixed KnownFolder locations used by the privileged service.
// It never consults FERRY_CONFIG_DIR or hardcoded drive letters.
type InstallPaths struct {
	DataDir         string
	RuleSetsDir     string
	ServiceDir      string
	ServiceCorePath string
	ServiceMetaPath string
	ClientAuthDir   string
	ClientAuthPath  string
}

func ServicePaths() (InstallPaths, error) {
	programData, err := windows.KnownFolderPath(windows.FOLDERID_ProgramData, windows.KF_FLAG_DEFAULT)
	if err != nil || !filepath.IsAbs(programData) {
		return InstallPaths{}, errors.New("无法解析 ProgramData 目录")
	}
	return buildInstallPaths(programData), nil
}

func buildInstallPaths(programData string) InstallPaths {
	dataDir := filepath.Join(programData, "Ferry")
	serviceDir := filepath.Join(dataDir, "service")
	clientAuthDir := filepath.Join(dataDir, "service-client")
	return InstallPaths{
		DataDir:         dataDir,
		RuleSetsDir:     filepath.Join(dataDir, "rulesets"),
		ServiceDir:      serviceDir,
		ServiceCorePath: filepath.Join(serviceDir, "ferry-core.exe"),
		ServiceMetaPath: filepath.Join(serviceDir, "service-meta.json"),
		ClientAuthDir:   clientAuthDir,
		ClientAuthPath:  filepath.Join(clientAuthDir, "auth.json"),
	}
}

// ServicePipeSDDL grants SYSTEM and Administrators full control and the single
// authorized GUI user read/write without FILE_CREATE_PIPE_INSTANCE.
func ServicePipeSDDL(authorizedUserSID string) (string, error) {
	sid, err := parseUserSID(authorizedUserSID)
	if err != nil {
		return "", err
	}
	return "D:P(A;;GA;;;SY)(A;;GA;;;BA)(A;;0x12019b;;;" + sid + ")", nil
}

func parseUserSID(raw string) (string, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" || raw == "S-1-5-18" || raw == "S-1-5-19" || raw == "S-1-5-20" || raw == "S-1-1-0" || raw == "S-1-5-11" {
		return "", errors.New("授权用户 SID 无效")
	}
	sid, err := windows.StringToSid(raw)
	if err != nil || sid == nil {
		return "", errors.New("授权用户 SID 格式无效")
	}
	return sid.String(), nil
}
