//go:build windows

package wintrust

import (
	"strings"

	"golang.org/x/sys/windows"
)

type lockedFile struct {
	handle    windows.Handle
	finalPath string
	identity  FileIdentity
}

func openLockedFile(path string) (*lockedFile, error) {
	name, err := windows.UTF16PtrFromString(path)
	if err != nil {
		return nil, ErrFileAccess
	}
	handle, err := windows.CreateFile(name, windows.GENERIC_READ,
		windows.FILE_SHARE_READ, nil, windows.OPEN_EXISTING,
		windows.FILE_FLAG_OPEN_REPARSE_POINT|windows.FILE_FLAG_SEQUENTIAL_SCAN, 0)
	if err != nil {
		return nil, ErrFileAccess
	}
	file := &lockedFile{handle: handle}
	if err := file.readIdentity(); err != nil {
		return nil, joinCleanup(err, file.close())
	}
	return file, nil
}

func (file *lockedFile) readIdentity() error {
	kind, err := windows.GetFileType(file.handle)
	if err != nil || kind != windows.FILE_TYPE_DISK {
		return ErrUnsafeFile
	}
	var info windows.ByHandleFileInformation
	if err := windows.GetFileInformationByHandle(file.handle, &info); err != nil {
		return ErrFileAccess
	}
	if info.FileAttributes&(windows.FILE_ATTRIBUTE_REPARSE_POINT|windows.FILE_ATTRIBUTE_DIRECTORY) != 0 {
		return ErrUnsafeFile
	}
	file.identity = FileIdentity{VolumeSerialNumber: info.VolumeSerialNumber,
		Index: uint64(info.FileIndexHigh)<<32 | uint64(info.FileIndexLow)}
	file.finalPath, err = finalPath(file.handle)
	return err
}

func finalPath(handle windows.Handle) (string, error) {
	buffer := make([]uint16, windows.MAX_LONG_PATH)
	length, err := windows.GetFinalPathNameByHandle(handle, &buffer[0], uint32(len(buffer)), 0)
	if err != nil || length == 0 || length >= uint32(len(buffer)) {
		return "", ErrFileAccess
	}
	path := windows.UTF16ToString(buffer[:length])
	if strings.HasPrefix(path, `\\?\UNC\`) {
		return `\\` + path[len(`\\?\UNC\`):], nil
	}
	return strings.TrimPrefix(path, `\\?\`), nil
}

func (file *lockedFile) close() error {
	if err := windows.CloseHandle(file.handle); err != nil {
		return ErrCleanup
	}
	return nil
}
