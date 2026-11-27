package app

import (
	"context"
	"errors"
	"ferry/internal/convert"
	"ferry/internal/spec"
	"io"
	"os"
	"path/filepath"
	"slices"
)

func (s *Subs) Export(_ context.Context, id, format string) (string, error) {
	s.session.mu.Lock()
	defer s.session.mu.Unlock()
	index := slices.IndexFunc(s.session.subs, func(sub subscription) bool { return sub.view.ID == id })
	if index < 0 {
		return "", errors.New("订阅不存在")
	}
	sub := s.session.subs[index]
	nodes := []spec.Node{}
	groups := []spec.Group{}
	for _, node := range s.session.config.Nodes {
		if slices.Contains(sub.nodeIDs, node.ID) {
			nodes = append(nodes, node)
		}
	}
	for _, group := range s.session.config.Groups {
		if slices.Contains(sub.groupIDs, group.ID) {
			groups = append(groups, group)
		}
	}
	return convert.Export(nodes, groups, format)
}

func (s *Subs) ExportNode(_ context.Context, id, format string) (string, error) {
	s.session.mu.Lock()
	defer s.session.mu.Unlock()
	for _, node := range s.session.config.Nodes {
		if node.ID == id {
			return convert.Export([]spec.Node{node}, nil, format)
		}
	}
	return "", errors.New("节点不存在")
}

func (s *Subs) AddFromFile(ctx context.Context, path string) (Subscription, error) {
	file, err := os.Open(path)
	if err != nil {
		return Subscription{}, errors.New("无法读取节点文件")
	}
	defer file.Close()
	data, err := io.ReadAll(io.LimitReader(file, convert.MaxBytes+1))
	if err != nil || len(data) > convert.MaxBytes {
		return Subscription{}, errors.New("节点文件读取失败或超过 1 MiB")
	}
	return s.AddFromText(ctx, filepath.Base(path), string(data))
}
