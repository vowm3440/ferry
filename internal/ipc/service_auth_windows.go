//go:build windows

package ipc

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"golang.org/x/sys/windows"
)

var hex64Pattern = regexp.MustCompile(`^[0-9a-f]{64}$`)

// ServiceMeta is written by the elevated installer to an admin-only directory
// and binds the FerryCore service to one user SID, binary SHA-256, and token.
type ServiceMeta struct {
	AuthorizedUserSID string `json:"authorizedUserSid"`
	GUIExePath        string `json:"guiExePath"`
	GUIExeSHA256      string `json:"guiExeSha256"`
	CoreExeSHA256     string `json:"coreExeSha256"`
	Token             string `json:"token"`
}

// ClientAuth is readable only by SYSTEM, Administrators, and the authorized GUI user.
type ClientAuth struct {
	CoreExeSHA256 string `json:"coreExeSha256"`
	Token         string `json:"token"`
	GUIExePath    string `json:"guiExePath,omitempty"`
	GUIExeSHA256  string `json:"guiExeSha256,omitempty"`
}

func GenerateServiceToken() (string, error) {
	var raw [32]byte
	if _, err := rand.Read(raw[:]); err != nil {
		return "", errors.New("无法生成服务通信密钥")
	}
	return hex.EncodeToString(raw[:]), nil
}

// HashFileLocked opens path with FILE_SHARE_READ only so the file cannot be
// replaced or modified while the returned release handle remains open.
func HashFileLocked(path string) (digest, finalPath string, release func() error, err error) {
	clean := filepath.Clean(path)
	if !filepath.IsAbs(clean) {
		return "", "", nil, errors.New("程序路径必须为绝对路径")
	}
	utf16, err := windows.UTF16PtrFromString(clean)
	if err != nil {
		return "", "", nil, errors.New("程序路径无效")
	}
	handle, err := windows.CreateFile(utf16, windows.GENERIC_READ, windows.FILE_SHARE_READ, nil, windows.OPEN_EXISTING, windows.FILE_ATTRIBUTE_NORMAL, 0)
	if err != nil {
		return "", "", nil, errors.New("无法锁定读取目标程序")
	}
	file := os.NewFile(uintptr(handle), clean)
	h := sha256.New()
	if _, err := io.Copy(h, io.LimitReader(file, 256<<20)); err != nil {
		_ = file.Close()
		return "", "", nil, errors.New("无法计算程序完整性摘要")
	}
	resolved, err := finalHandlePath(handle)
	if err != nil {
		_ = file.Close()
		return "", "", nil, errors.New("无法解析目标程序真实路径")
	}
	return hex.EncodeToString(h.Sum(nil)), resolved, file.Close, nil
}

func finalHandlePath(handle windows.Handle) (string, error) {
	buf := make([]uint16, 1024)
	n, err := windows.GetFinalPathNameByHandle(handle, &buf[0], uint32(len(buf)), 0)
	if err != nil || n == 0 || int(n) >= len(buf) {
		return "", errors.New("无法解析最终路径")
	}
	return strings.TrimPrefix(windows.UTF16ToString(buf[:n]), `\\?\`), nil
}

func (m ServiceMeta) Validate() error {
	if _, err := parseUserSID(m.AuthorizedUserSID); err != nil {
		return err
	}
	if !filepath.IsAbs(m.GUIExePath) || !hex64Pattern.MatchString(m.GUIExeSHA256) || !hex64Pattern.MatchString(m.CoreExeSHA256) || !hex64Pattern.MatchString(m.Token) {
		return errors.New("服务认证元数据无效")
	}
	return nil
}

func (a ClientAuth) Validate() error {
	if !hex64Pattern.MatchString(a.CoreExeSHA256) || !hex64Pattern.MatchString(a.Token) {
		return errors.New("服务客户端认证凭据无效")
	}
	if a.GUIExePath != "" && !filepath.IsAbs(a.GUIExePath) {
		return errors.New("服务客户端认证凭据无效")
	}
	if a.GUIExeSHA256 != "" && !hex64Pattern.MatchString(a.GUIExeSHA256) {
		return errors.New("服务客户端认证凭据无效")
	}
	return nil
}

func WriteServiceAuthFiles(paths InstallPaths, meta ServiceMeta) error {
	if err := meta.Validate(); err != nil {
		return err
	}
	if err := ProtectDataDir(paths.ServiceDir); err != nil {
		return err
	}
	if err := ProtectClientAuthDir(paths.ClientAuthDir, meta.AuthorizedUserSID); err != nil {
		return err
	}
	metaBytes, err := json.Marshal(meta)
	if err != nil {
		return errors.New("无法编码服务元数据")
	}
	if err := os.WriteFile(paths.ServiceMetaPath, metaBytes, 0o600); err != nil {
		return errors.New("无法写入服务元数据")
	}
	clientBytes, err := json.Marshal(clientAuthForMeta(meta))
	if err != nil {
		return errors.New("无法编码服务客户端凭据")
	}
	if err := os.WriteFile(paths.ClientAuthPath, clientBytes, 0o600); err != nil {
		return errors.New("无法写入服务客户端凭据")
	}
	return nil
}

func clientAuthForMeta(meta ServiceMeta) ClientAuth {
	return ClientAuth{
		CoreExeSHA256: meta.CoreExeSHA256,
		Token:         meta.Token,
		GUIExePath:    meta.GUIExePath,
		GUIExeSHA256:  meta.GUIExeSHA256,
	}
}

func ReadServiceMeta(path string) (ServiceMeta, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return ServiceMeta{}, errors.New("无法读取服务认证元数据")
	}
	var meta ServiceMeta
	if err := json.Unmarshal(data, &meta); err != nil {
		return ServiceMeta{}, errors.New("服务认证元数据格式无效")
	}
	if err := meta.Validate(); err != nil {
		return ServiceMeta{}, err
	}
	return meta, nil
}

func ReadClientAuth(path string) (ClientAuth, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return ClientAuth{}, errors.New("无法读取服务客户端凭据")
	}
	var auth ClientAuth
	if err := json.Unmarshal(data, &auth); err != nil {
		return ClientAuth{}, errors.New("服务客户端凭据格式无效")
	}
	if err := auth.Validate(); err != nil {
		return ClientAuth{}, err
	}
	return auth, nil
}
