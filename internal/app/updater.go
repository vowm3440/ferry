package app

import (
	"context"
	"crypto/ed25519"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/base64"
	"encoding/hex"
	"errors"
	"ferry/internal/release"
	"ferry/internal/update"
	"io"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"
)

type UpdateInfo struct {
	Available  bool   `json:"available"`
	Version    string `json:"version"`
	Channel    string `json:"channel"`
	Configured bool   `json:"configured"`
}

type UpdateState struct {
	Phase    string `json:"phase"` // idle | checking | available | downloading | staged | installing | error | unconfigured
	Version  string `json:"version"`
	Progress int    `json:"progress"`
	Error    string `json:"error,omitempty"`
}

type UpdateSource struct {
	ManifestURL string
	PublicKey   ed25519.PublicKey
	Client      *http.Client
	StageDir    string
}

// UpdateInstallerFunc launches a verified staged artifact and reports whether
// the running desktop application should exit immediately so files can be replaced.
type UpdateInstallerFunc func(stagedPath string) (shouldQuit bool, err error)

type updateRuntime struct {
	source    *UpdateSource
	state     UpdateState
	manifest  *update.Manifest
	staged    string
	installer UpdateInstallerFunc
	quit      func()
}

type Update struct{ session *Session }

func (s *Session) Update() *Update { return &Update{session: s} }

func (s *Session) SetUpdateSource(src *UpdateSource) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.updater.source = src
}

func (s *Session) SetUpdateInstaller(installer UpdateInstallerFunc, quit func()) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.updater.installer = installer
	s.updater.quit = quit
}

func (u *Update) GetState(context.Context) (UpdateState, error) {
	u.session.mu.Lock()
	defer u.session.mu.Unlock()
	if u.session.updater.state.Phase == "" {
		if _, err := u.session.resolveUpdateSource(); err != nil {
			return UpdateState{Phase: "unconfigured"}, nil
		}
		return UpdateState{Phase: "idle"}, nil
	}
	return u.session.updater.state, nil
}

func (s *Session) resolveUpdateSource() (UpdateSource, error) {
	if s.updater.source != nil {
		if s.updater.source.ManifestURL == "" || len(s.updater.source.PublicKey) != ed25519.PublicKeySize {
			return UpdateSource{}, release.ErrUpdateUnconfigured
		}
		return *s.updater.source, nil
	}
	feed, err := release.RequireUpdateFeed()
	if err != nil {
		return UpdateSource{}, err
	}
	return UpdateSource{
		ManifestURL: feed.ManifestURL,
		PublicKey:   feed.PublicKey,
		StageDir:    filepath.Join(s.configDir, "updates"),
	}, nil
}

func (s *System) CheckUpdate(ctx context.Context, channel string) (UpdateInfo, error) {
	if channel != "stable" && channel != "beta" {
		return UpdateInfo{}, errors.New("更新通道无效")
	}
	s.session.mu.Lock()
	src, err := s.session.resolveUpdateSource()
	s.session.mu.Unlock()
	if err != nil {
		s.session.setUpdateState(UpdateState{Phase: "unconfigured", Error: err.Error()})
		return UpdateInfo{Channel: channel, Configured: false}, err
	}
	s.session.setUpdateState(UpdateState{Phase: "checking"})
	raw, sig, err := fetchSignedManifest(ctx, src, channel)
	if err != nil {
		s.session.setUpdateState(UpdateState{Phase: "error", Error: err.Error()})
		return UpdateInfo{Channel: channel, Configured: true}, err
	}
	manifest, err := update.VerifyManifest(raw, sig, src.PublicKey, release.Version, channel)
	if err != nil {
		if strings.Contains(err.Error(), "newer") {
			s.session.setUpdateState(UpdateState{Phase: "idle", Version: release.Version})
			return UpdateInfo{Available: false, Version: release.Version, Channel: channel, Configured: true}, nil
		}
		s.session.setUpdateState(UpdateState{Phase: "error", Error: "更新清单签名或内容验证失败"})
		return UpdateInfo{Channel: channel, Configured: true}, errors.New("更新清单签名或内容验证失败")
	}
	s.session.mu.Lock()
	s.session.updater.manifest = &manifest
	s.session.mu.Unlock()
	s.session.setUpdateState(UpdateState{Phase: "available", Version: manifest.Version()})
	return UpdateInfo{Available: true, Version: manifest.Version(), Channel: channel, Configured: true}, nil
}

func (s *System) ApplyUpdate(ctx context.Context) error {
	s.session.mu.Lock()
	src, err := s.session.resolveUpdateSource()
	manifest := s.session.updater.manifest
	s.session.mu.Unlock()
	if err != nil {
		return err
	}
	if manifest == nil {
		return errors.New("请先检查并验证可用更新")
	}
	if src.StageDir == "" || !filepath.IsAbs(src.StageDir) {
		return errors.New("更新暂存目录无效")
	}
	if err := os.MkdirAll(src.StageDir, 0o700); err != nil {
		return errors.New("无法创建更新暂存目录")
	}
	target := filepath.Join(src.StageDir, stagedFilename(*manifest))
	if verifyStagedArtifact(target, *manifest) != nil {
		_ = os.Remove(target)
		s.session.setUpdateState(UpdateState{Phase: "downloading", Version: manifest.Version(), Progress: 10})
		if err := update.NewDownloader(src.Client).Download(ctx, *manifest, target); err != nil {
			s.session.setUpdateState(UpdateState{Phase: "error", Version: manifest.Version(), Error: "更新包下载或完整性校验失败"})
			return errors.New("更新包下载或完整性校验失败")
		}
	}
	s.session.mu.Lock()
	s.session.updater.staged = target
	s.session.mu.Unlock()
	s.session.setUpdateState(UpdateState{Phase: "staged", Version: manifest.Version(), Progress: 100})
	return nil
}

func (s *System) InstallUpdate(_ context.Context) error {
	s.session.mu.Lock()
	manifest := s.session.updater.manifest
	staged := s.session.updater.staged
	installer := s.session.updater.installer
	quit := s.session.updater.quit
	s.session.mu.Unlock()
	if manifest == nil || staged == "" {
		return errors.New("尚未下载并暂存可用更新包")
	}
	if err := verifyStagedArtifact(staged, *manifest); err != nil {
		s.session.setUpdateState(UpdateState{Phase: "error", Version: manifest.Version(), Error: err.Error()})
		return err
	}
	if installer == nil {
		installer = defaultLaunchUpdate
	}
	s.session.setUpdateState(UpdateState{Phase: "installing", Version: manifest.Version(), Progress: 100})
	shouldQuit, err := installer(staged)
	if err != nil {
		s.session.setUpdateState(UpdateState{Phase: "error", Version: manifest.Version(), Error: "无法启动更新安装程序"})
		return errors.New("无法启动更新安装程序")
	}
	if shouldQuit && quit != nil {
		quit()
	}
	return nil
}

func stagedFilename(m update.Manifest) string {
	ext := ".bin"
	if parsed, err := url.Parse(m.URL()); err == nil {
		lower := strings.ToLower(parsed.Path)
		if strings.HasSuffix(lower, ".exe") {
			ext = ".exe"
		} else if strings.HasSuffix(lower, ".zip") {
			ext = ".zip"
		}
	}
	return "Ferry-" + m.Version() + ext
}

func verifyStagedArtifact(path string, m update.Manifest) error {
	file, err := os.Open(filepath.Clean(path))
	if err != nil {
		return errors.New("未找到已暂存的更新包")
	}
	defer file.Close()
	info, err := file.Stat()
	if err != nil || !info.Mode().IsRegular() || info.Size() != m.Size() {
		return errors.New("更新包大小与签名清单不一致，请重新下载")
	}
	h := sha256.New()
	if _, err := io.Copy(h, io.LimitReader(file, m.Size()+1)); err != nil {
		return errors.New("无法读取更新包校验完整性")
	}
	digest := hex.EncodeToString(h.Sum(nil))
	if subtle.ConstantTimeCompare([]byte(digest), []byte(m.SHA256())) != 1 {
		return errors.New("更新包 SHA-256 校验失败，拒绝安装")
	}
	return nil
}

func defaultLaunchUpdate(path string) (bool, error) {
	clean := filepath.Clean(path)
	if !filepath.IsAbs(clean) {
		return false, errors.New("invalid path")
	}
	if strings.HasSuffix(strings.ToLower(clean), ".exe") {
		return true, launchInstallerExecutable(clean)
	}
	return false, exec.Command("explorer.exe", "/select,"+clean).Start()
}

func (s *Session) setUpdateState(st UpdateState) {
	s.mu.Lock()
	s.updater.state = st
	s.mu.Unlock()
	s.emit("update:state", st)
}

func fetchSignedManifest(ctx context.Context, src UpdateSource, channel string) ([]byte, []byte, error) {
	client := src.Client
	if client == nil {
		client = &http.Client{Timeout: 15 * time.Second}
	}
	manifestURL := src.ManifestURL + "?channel=" + channel
	if strings.Contains(src.ManifestURL, "{channel}") {
		manifestURL = strings.ReplaceAll(src.ManifestURL, "{channel}", channel)
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, manifestURL, nil)
	if err != nil {
		return nil, nil, errors.New("无法创建更新请求")
	}
	resp, err := client.Do(req)
	if err != nil {
		return nil, nil, errors.New("无法连接更新服务器")
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, nil, errors.New("更新服务器返回异常状态")
	}
	raw, err := io.ReadAll(io.LimitReader(resp.Body, update.MaxManifestBytes+1))
	if err != nil || len(raw) == 0 || len(raw) > update.MaxManifestBytes {
		return nil, nil, errors.New("更新清单大小无效")
	}
	sigText := strings.TrimSpace(resp.Header.Get("X-Ferry-Signature"))
	if sigText == "" {
		sigText, err = fetchCompanionSignature(ctx, client, manifestURL)
		if err != nil {
			return nil, nil, err
		}
	}
	sig, err := base64.StdEncoding.DecodeString(sigText)
	if err != nil || len(sig) != ed25519.SignatureSize {
		return nil, nil, errors.New("更新清单缺少有效的 ed25519 签名")
	}
	return raw, sig, nil
}

func fetchCompanionSignature(ctx context.Context, client *http.Client, manifestURL string) (string, error) {
	sigURL := strings.SplitN(manifestURL, "?", 2)[0] + ".sig"
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, sigURL, nil)
	if err != nil {
		return "", errors.New("更新清单缺少有效的 ed25519 签名")
	}
	resp, err := client.Do(req)
	if err != nil {
		return "", errors.New("更新清单缺少有效的 ed25519 签名")
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return "", errors.New("更新清单缺少有效的 ed25519 签名")
	}
	body, err := io.ReadAll(io.LimitReader(resp.Body, 512))
	if err != nil {
		return "", errors.New("更新清单缺少有效的 ed25519 签名")
	}
	return strings.TrimSpace(string(body)), nil
}
