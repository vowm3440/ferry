package app

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"

	"ferry/internal/store"
)

// Backups use the same Windows-account-bound DPAPI envelope as state.dat.
// They contain credentials, but can never be silently exported as plaintext.
func (s *System) ExportBackup(ctx context.Context, path string) error {
	if ctx.Err() != nil {
		return errors.New("备份已取消")
	}
	if !filepath.IsAbs(path) {
		return errors.New("备份路径须为绝对路径")
	}
	s.session.mu.Lock()
	data, err := json.Marshal(s.session.snapshot())
	s.session.mu.Unlock()
	if err != nil {
		return errors.New("备份无法编码")
	}
	staging, err := os.MkdirTemp(filepath.Dir(path), ".ferry-backup-")
	if err != nil {
		return errors.New("无法创建备份临时目录")
	}
	defer os.Remove(staging)
	temporary := filepath.Join(staging, "backup.dat")
	defer os.Remove(temporary)
	if err = store.New(temporary).Save(data); err != nil {
		return err
	}
	if ctx.Err() != nil {
		return errors.New("备份已取消")
	}
	// Hard-link creation fails atomically if the destination already exists.
	if err = os.Link(temporary, path); err != nil {
		return errors.New("无法创建备份，请选择同一磁盘上尚不存在的文件")
	}
	return nil
}

func (s *System) RestoreBackup(ctx context.Context, path string) error {
	if ctx.Err() != nil {
		return errors.New("恢复已取消")
	}
	if !filepath.IsAbs(path) {
		return errors.New("备份路径须为绝对路径")
	}
	data, err := store.New(path).Load()
	if err != nil {
		return err
	}
	if len(data) == 0 {
		return errors.New("备份为空或不存在")
	}
	next, err := decodeSession(data)
	if err != nil {
		return err
	}
	if err = validateImportedConfig(next.Config); err != nil {
		return errors.New("备份配置超过内核支持范围")
	}
	s.session.mu.Lock()
	defer s.session.mu.Unlock()
	return s.session.reconfigure(ctx, next)
}

// Diagnostics deliberately include only a small allowlist of non-identifying
// facts. No arbitrary error/log text, nodes, labels, paths, URLs or IPs enter it.
func (s *System) ExportDiagnostics(ctx context.Context, path string) error {
	if ctx.Err() != nil {
		return errors.New("诊断导出已取消")
	}
	s.session.mu.Lock()
	state := s.session.state(ctx)
	report := map[string]any{"schema": 1, "version": state.Version, "coreVersion": state.CoreVersion, "status": state.Status, "mode": state.Mode, "serviceMode": state.ServiceMode, "nodeCount": len(s.session.config.Nodes), "groupCount": len(s.session.config.Groups), "subscriptionCount": len(s.session.subs), "ruleCount": len(s.session.config.Rules), "persistent": s.session.storage != nil, "platform": "windows"}
	s.session.mu.Unlock()
	data, err := json.MarshalIndent(report, "", "  ")
	if err != nil {
		return errors.New("无法编码诊断信息")
	}
	if !filepath.IsAbs(path) {
		return errors.New("诊断路径须为绝对路径")
	}
	file, err := os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0600)
	if err != nil {
		return errors.New("无法创建诊断文件，请选择尚不存在的文件")
	}
	defer file.Close()
	if _, err = file.Write(data); err != nil {
		return errors.New("诊断写入失败")
	}
	if err = file.Sync(); err != nil {
		return errors.New("诊断保存失败")
	}
	return nil
}
