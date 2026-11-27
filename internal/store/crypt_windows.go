//go:build windows

package store

import (
	"errors"
	"runtime"
	"unsafe"

	"golang.org/x/sys/windows"
)

func protect(data []byte) ([]byte, error)   { return crypt(data, true) }
func unprotect(data []byte) ([]byte, error) { return crypt(data, false) }

func crypt(data []byte, seal bool) ([]byte, error) {
	if len(data) == 0 || len(data) > MaxBytes+(64<<10) {
		return nil, errors.New("invalid encrypted data size")
	}
	input := windows.DataBlob{Size: uint32(len(data)), Data: &data[0]}
	var output windows.DataBlob
	var err error
	if seal {
		err = windows.CryptProtectData(&input, nil, nil, 0, nil, windows.CRYPTPROTECT_UI_FORBIDDEN, &output)
	} else {
		err = windows.CryptUnprotectData(&input, nil, nil, 0, nil, windows.CRYPTPROTECT_UI_FORBIDDEN, &output)
	}
	runtime.KeepAlive(data)
	if err != nil {
		return nil, err
	}
	defer windows.LocalFree(windows.Handle(unsafe.Pointer(output.Data)))
	if output.Size > MaxBytes+(64<<10) {
		return nil, errors.New("invalid decrypted data size")
	}
	return append([]byte(nil), unsafe.Slice(output.Data, int(output.Size))...), nil
}
