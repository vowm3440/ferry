// Package store atomically persists bounded JSON encrypted for the current
// Windows account. It never falls back to plaintext.
package store

import (
	"bytes"
	"encoding/json"
	"errors"
	"io"
	"os"
	"path/filepath"
	"sync"
	"time"
)

const MaxBytes = 8 << 20

var header = []byte("FERRY-STATE\x01")

type File struct {
	mu   sync.Mutex
	path string
}

func New(path string) *File { return &File{path: path} }

func (f *File) Load() ([]byte, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	file, err := os.Open(f.path)
	if errors.Is(err, os.ErrNotExist) {
		return nil, nil
	}
	if err != nil {
		return nil, errors.New("无法读取已保存的配置")
	}
	defer file.Close()
	cipher, err := io.ReadAll(io.LimitReader(file, MaxBytes+(64<<10)+1))
	if err != nil || len(cipher) > MaxBytes+(64<<10) || !bytes.HasPrefix(cipher, header) {
		return nil, errors.New("配置文件格式无效或超过大小限制")
	}
	plain, err := unprotect(cipher[len(header):])
	if err != nil || len(plain) == 0 || len(plain) > MaxBytes || !json.Valid(plain) {
		return nil, errors.New("配置无法解密，请使用原 Windows 账户或恢复备份")
	}
	return plain, nil
}

func (f *File) Save(plain []byte) error {
	if len(plain) == 0 || len(plain) > MaxBytes || !json.Valid(plain) {
		return errors.New("保存内容无效或超过大小限制")
	}
	f.mu.Lock()
	defer f.mu.Unlock()
	cipher, err := protect(plain)
	if err != nil {
		return errors.New("无法加密配置，未写入文件")
	}
	if err := atomicWrite(f.path, append(bytes.Clone(header), cipher...)); err != nil {
		return errors.New("无法保存配置，原文件保持不变")
	}
	return nil
}

func atomicWrite(path string, data []byte) error {
	if err := os.MkdirAll(filepath.Dir(path), 0700); err != nil {
		return err
	}
	temp, err := os.CreateTemp(filepath.Dir(path), ".ferry-save-*")
	if err != nil {
		return err
	}
	defer os.Remove(temp.Name())
	defer temp.Close()
	if _, err := temp.Write(data); err != nil {
		return err
	}
	if err := temp.Sync(); err != nil {
		return err
	}
	if err := temp.Close(); err != nil {
		return err
	}
	return renameRetry(temp.Name(), path)
}

func renameRetry(src, dst string) error {
	var err error
	for attempt := 0; attempt < 4; attempt++ {
		if err = os.Rename(src, dst); err == nil {
			return nil
		}
		if attempt < 3 {
			time.Sleep(20 * time.Millisecond)
		}
	}
	return err
}
