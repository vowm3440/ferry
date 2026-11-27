package rulesets

import (
	"context"
	"encoding/binary"
	"net/http"
	"os"
	"path/filepath"
	"syscall"
	"testing"
	"time"

	"golang.org/x/sys/windows"
)

func TestLockedCachePreservesPreviousVersion(t *testing.T) {
	root := t.TempDir()
	path := filepath.Join(root, "geosite-cn.srs")
	previous, replacement := fixtureSRS(t, "old.example.cn"), fixtureSRS(t, "new.example.cn")
	writeFixture(t, path, previous)
	name, err := windows.UTF16PtrFromString(path)
	if err != nil {
		t.Fatal(err)
	}
	handle, err := windows.CreateFile(name, windows.GENERIC_READ, windows.FILE_SHARE_READ, nil, windows.OPEN_EXISTING, 0, 0)
	if err != nil {
		t.Fatal(err)
	}
	defer windows.CloseHandle(handle)
	client := tlsClient(t, http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) { _, _ = w.Write(replacement) }))
	if err = NewWithClient(root, client).Update(context.Background(), "geosite-cn"); err == nil {
		t.Fatal("locked cache replaced")
	}
	assertFileBytes(t, path, previous)
	assertNoStagingFiles(t, root)
}

func TestWindowsReparseAttributesAreRejected(t *testing.T) {
	info := windowsFileInfo{attributes: syscall.FILE_ATTRIBUTE_REPARSE_POINT}
	if !isReparse(info) {
		t.Fatal("non-symlink reparse point accepted")
	}
	if isReparse(windowsFileInfo{}) {
		t.Fatal("ordinary file rejected")
	}
}

func TestJunctionRootAndAncestorAreRejected(t *testing.T) {
	outside := t.TempDir()
	data := fixtureSRS(t, "outside.example.cn")
	writeFixture(t, filepath.Join(outside, "geosite-cn.srs"), data)
	link := filepath.Join(t.TempDir(), "junction")
	createJunction(t, link, outside)
	for _, root := range []string{link, filepath.Join(link, "nested-cache")} {
		m := NewWithClient(root, offlineClient(t))
		if _, err := m.Load("geosite-cn"); err == nil {
			t.Fatal("junction root accepted for reading")
		}
		if err := m.Update(context.Background(), "geosite-cn"); err == nil {
			t.Fatal("junction root accepted for writing")
		}
	}
	if _, err := os.Stat(filepath.Join(outside, "nested-cache")); !os.IsNotExist(err) {
		t.Fatalf("created cache outside trusted root: %v", err)
	}
	assertFileBytes(t, filepath.Join(outside, "geosite-cn.srs"), data)
}

func createJunction(t *testing.T, link, target string) {
	t.Helper()
	if err := os.Mkdir(link, 0700); err != nil {
		t.Fatal(err)
	}
	name, err := windows.UTF16PtrFromString(link)
	if err != nil {
		t.Fatal(err)
	}
	handle, err := windows.CreateFile(name, windows.GENERIC_WRITE, 0, nil, windows.OPEN_EXISTING, windows.FILE_FLAG_OPEN_REPARSE_POINT|windows.FILE_FLAG_BACKUP_SEMANTICS, 0)
	if err != nil {
		t.Fatal(err)
	}
	defer windows.CloseHandle(handle)
	substitute, err := windows.UTF16FromString(`\??\` + target)
	if err != nil {
		t.Fatal(err)
	}
	printName, err := windows.UTF16FromString(target)
	if err != nil {
		t.Fatal(err)
	}
	data := make([]byte, 16+2*(len(substitute)+len(printName)))
	binary.LittleEndian.PutUint32(data, windows.IO_REPARSE_TAG_MOUNT_POINT)
	binary.LittleEndian.PutUint16(data[4:], uint16(len(data)-8))
	binary.LittleEndian.PutUint16(data[10:], uint16(2*(len(substitute)-1)))
	binary.LittleEndian.PutUint16(data[12:], uint16(2*len(substitute)))
	binary.LittleEndian.PutUint16(data[14:], uint16(2*(len(printName)-1)))
	for index, word := range append(substitute, printName...) {
		binary.LittleEndian.PutUint16(data[16+index*2:], word)
	}
	var returned uint32
	if err = windows.DeviceIoControl(handle, windows.FSCTL_SET_REPARSE_POINT, &data[0], uint32(len(data)), nil, 0, &returned, nil); err != nil {
		t.Fatal(err)
	}
}

type windowsFileInfo struct{ attributes uint32 }

func (windowsFileInfo) Name() string       { return "fixture" }
func (windowsFileInfo) Size() int64        { return 0 }
func (windowsFileInfo) Mode() os.FileMode  { return 0 }
func (windowsFileInfo) ModTime() time.Time { return time.Time{} }
func (windowsFileInfo) IsDir() bool        { return false }
func (f windowsFileInfo) Sys() any {
	return &syscall.Win32FileAttributeData{FileAttributes: f.attributes}
}
