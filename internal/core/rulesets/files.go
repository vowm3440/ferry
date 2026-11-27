package rulesets

import (
	"context"
	"crypto/rand"
	"errors"
	"io"
	"os"
	"path/filepath"
	"strings"
)

func openCacheRoot(path string, create bool) (*os.Root, error) {
	if path == "" || !filepath.IsAbs(path) || filepath.Clean(path) != path {
		return nil, errors.New("invalid rule-set cache directory")
	}
	volume := filepath.VolumeName(path) + string(os.PathSeparator)
	root, err := os.OpenRoot(volume)
	if err != nil {
		return nil, errors.New("cannot open rule-set cache volume")
	}
	for _, name := range strings.Split(strings.TrimPrefix(path, volume), string(os.PathSeparator)) {
		if name == "" {
			continue
		}
		next, stepErr := openDirectory(root, name, create)
		root.Close()
		if stepErr != nil {
			return nil, stepErr
		}
		root = next
	}
	return root, nil
}

func openDirectory(root *os.Root, name string, create bool) (*os.Root, error) {
	info, err := root.Lstat(name)
	if errors.Is(err, os.ErrNotExist) && create {
		if err = root.Mkdir(name, 0700); err != nil && !errors.Is(err, os.ErrExist) {
			return nil, errors.New("cannot create rule-set cache directory")
		}
		info, err = root.Lstat(name)
	}
	if err != nil {
		return nil, localError(err)
	}
	if !info.IsDir() || isReparse(info) {
		return nil, errors.New("rule-set cache directory must not be a link")
	}
	next, err := root.OpenRoot(name)
	if err != nil {
		return nil, errors.New("cannot open rule-set cache directory")
	}
	opened, err := next.Stat(".")
	if err != nil || !os.SameFile(info, opened) {
		next.Close()
		return nil, errors.New("rule-set cache directory changed while opening")
	}
	return next, nil
}

func safeDestination(root *os.Root, name string) error {
	info, err := root.Lstat(name)
	if errors.Is(err, os.ErrNotExist) {
		return nil
	}
	if err != nil || !info.Mode().IsRegular() || isReparse(info) {
		return errors.New("rule-set cache entry must be a regular file")
	}
	return nil
}

func readCacheFile(root *os.Root, name string) ([]byte, os.FileInfo, error) {
	before, err := root.Lstat(name)
	if err != nil {
		return nil, nil, localError(err)
	}
	if !before.Mode().IsRegular() || isReparse(before) || before.Size() > maxCompressed {
		return nil, nil, errors.New("rule-set cache entry is unsafe or oversized")
	}
	file, err := root.Open(name)
	if err != nil {
		return nil, nil, errors.New("cannot read rule-set cache")
	}
	defer file.Close()
	opened, err := file.Stat()
	if err != nil || !os.SameFile(before, opened) || !opened.Mode().IsRegular() || isReparse(opened) {
		return nil, nil, errors.New("rule-set cache changed while opening")
	}
	data, err := io.ReadAll(io.LimitReader(file, maxCompressed+1))
	if err != nil || len(data) > maxCompressed {
		return nil, nil, errors.New("cannot read bounded rule-set cache")
	}
	return data, opened, nil
}

func publish(ctx context.Context, root *os.Root, name string, data []byte) (err error) {
	temporary := ".ferry-ruleset-" + rand.Text() + ".tmp"
	file, err := root.OpenFile(temporary, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0600)
	if err != nil {
		return errors.New("cannot stage rule-set cache")
	}
	defer func() {
		file.Close()
		if removeErr := root.Remove(temporary); removeErr != nil && !errors.Is(removeErr, os.ErrNotExist) {
			err = errors.Join(err, errors.New("cannot remove staged rule-set cache"))
		}
	}()
	if _, err = file.Write(data); err != nil {
		return errors.New("cannot write staged rule-set cache")
	}
	if err = file.Sync(); err != nil {
		return errors.New("cannot flush staged rule-set cache")
	}
	if err = file.Close(); err != nil {
		return errors.New("cannot close staged rule-set cache")
	}
	if err = ctx.Err(); err != nil {
		return err
	}
	if err = safeDestination(root, name); err != nil {
		return err
	}
	// Root.Rename is the confined equivalent of os.Rename; destination replacement
	// is atomic and never truncates the old inode or follows a destination link.
	if err = root.Rename(temporary, name); err != nil {
		return errors.New("cannot atomically replace rule-set cache")
	}
	return nil
}

func localError(err error) error {
	if errors.Is(err, os.ErrNotExist) {
		return errors.Join(errors.New("rule-set cache is missing"), os.ErrNotExist)
	}
	return errors.New("cannot access rule-set cache")
}
