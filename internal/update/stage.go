package update

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"io"
	"os"
	"path/filepath"
	"strings"
)

type downloadStage struct {
	root                   *os.Root
	file                   *os.File
	temporary, destination string
}

func openStage(path string) (*downloadStage, error) {
	name := filepath.Base(path)
	if name == "." || name == ".." || strings.ContainsAny(name, `\/:`) {
		return nil, errors.New("invalid update destination")
	}
	root, err := os.OpenRoot(filepath.Dir(path))
	if err != nil {
		return nil, errors.New("unable to open update destination directory")
	}
	if _, err = root.Lstat(name); !errors.Is(err, os.ErrNotExist) {
		root.Close()
		return nil, errors.New("update destination already exists or is inaccessible")
	}
	temporary := ".ferry-update-" + rand.Text() + ".tmp"
	file, err := root.OpenFile(temporary, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0600)
	if err != nil {
		root.Close()
		return nil, errors.New("unable to stage update download")
	}
	return &downloadStage{root: root, file: file, temporary: temporary, destination: name}, nil
}

func (s *downloadStage) write(ctx context.Context, body io.Reader, manifest Manifest) error {
	digest := sha256.New()
	count, err := io.Copy(io.MultiWriter(s.file, digest), io.LimitReader(body, manifest.size+1))
	if err != nil {
		return downloadError(ctx, err)
	}
	if count != manifest.size || hex.EncodeToString(digest.Sum(nil)) != manifest.sha256 {
		return errors.New("update download size or SHA-256 verification failed")
	}
	if err = s.file.Sync(); err != nil {
		return errors.New("unable to flush staged update")
	}
	err = s.file.Close()
	s.file = nil
	if err != nil {
		return errors.New("unable to close staged update")
	}
	return nil
}

func (s *downloadStage) publish(ctx context.Context) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	// Link creates the final directory entry atomically and never replaces one.
	// Both names stay confined to the directory handle opened before download.
	if err := s.root.Link(s.temporary, s.destination); err != nil {
		return errors.New("unable to publish update without replacing an existing destination")
	}
	return nil
}

func (s *downloadStage) cleanup() error {
	if s.file != nil {
		s.file.Close()
	}
	err := s.root.Remove(s.temporary)
	s.root.Close()
	if err != nil && !errors.Is(err, os.ErrNotExist) {
		return errors.New("unable to remove temporary update file")
	}
	return nil
}
