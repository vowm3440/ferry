//go:build windows

package share

import (
	"errors"
	"image"
	"unsafe"

	"golang.org/x/sys/windows"
)

const (
	smXVirtualScreen  = 76
	smYVirtualScreen  = 77
	smCXVirtualScreen = 78
	smCYVirtualScreen = 79
	srccopy           = 0x00CC0020
	dibRGBColors      = 0
	biRGB             = 0
)

var (
	user32DLL             = windows.NewLazySystemDLL("user32.dll")
	gdi32DLL              = windows.NewLazySystemDLL("gdi32.dll")
	procGetSystemMetrics  = user32DLL.NewProc("GetSystemMetrics")
	procGetDC             = user32DLL.NewProc("GetDC")
	procReleaseDC         = user32DLL.NewProc("ReleaseDC")
	procCreateCompatDC    = gdi32DLL.NewProc("CreateCompatibleDC")
	procCreateCompatBmp   = gdi32DLL.NewProc("CreateCompatibleBitmap")
	procSelectObject      = gdi32DLL.NewProc("SelectObject")
	procBitBlt            = gdi32DLL.NewProc("BitBlt")
	procGetDIBits         = gdi32DLL.NewProc("GetDIBits")
	procDeleteObject      = gdi32DLL.NewProc("DeleteObject")
	procDeleteDC          = gdi32DLL.NewProc("DeleteDC")
)

type bitmapInfoHeader struct {
	Size          uint32
	Width         int32
	Height        int32
	Planes        uint16
	BitCount      uint16
	Compression   uint32
	SizeImage     uint32
	XPelsPerMeter int32
	YPelsPerMeter int32
	ClrUsed       uint32
	ClrImportant  uint32
}

// CaptureScreenQR captures the Windows virtual desktop via GDI and decodes a QR code.
func CaptureScreenQR() (string, error) {
	img, err := captureVirtualDesktop()
	if err != nil {
		return "", err
	}
	return DecodeQRImage(img)
}

func captureVirtualDesktop() (*image.NRGBA, error) {
	x := int32(systemMetric(smXVirtualScreen))
	y := int32(systemMetric(smYVirtualScreen))
	w := int(systemMetric(smCXVirtualScreen))
	h := int(systemMetric(smCYVirtualScreen))
	if w < 1 || h < 1 || w > 8192 || h > 8192 || int64(w)*int64(h) > 16<<20 {
		return nil, errors.New("当前屏幕分辨率超出截图识别范围")
	}
	screenDC, _, _ := procGetDC.Call(0)
	if screenDC == 0 {
		return nil, errors.New("无法获取屏幕画面")
	}
	defer procReleaseDC.Call(0, screenDC)
	memDC, _, _ := procCreateCompatDC.Call(screenDC)
	if memDC == 0 {
		return nil, errors.New("无法创建截图缓冲区")
	}
	defer procDeleteDC.Call(memDC)
	bmp, _, _ := procCreateCompatBmp.Call(screenDC, uintptr(w), uintptr(h))
	if bmp == 0 {
		return nil, errors.New("无法分配截图位图")
	}
	defer procDeleteObject.Call(bmp)
	prev, _, _ := procSelectObject.Call(memDC, bmp)
	defer procSelectObject.Call(memDC, prev)
	r, _, _ := procBitBlt.Call(memDC, 0, 0, uintptr(w), uintptr(h), screenDC, uintptr(x), uintptr(y), srccopy)
	if r == 0 {
		return nil, errors.New("屏幕截图失败")
	}
	return readDIBits(memDC, bmp, w, h)
}

func readDIBits(memDC, bmp uintptr, w, h int) (*image.NRGBA, error) {
	hdr := bitmapInfoHeader{
		Size:        uint32(unsafe.Sizeof(bitmapInfoHeader{})),
		Width:       int32(w),
		Height:      -int32(h),
		Planes:      1,
		BitCount:    32,
		Compression: biRGB,
	}
	buf := make([]byte, w*h*4)
	lines, _, _ := procGetDIBits.Call(memDC, bmp, 0, uintptr(h), uintptr(unsafe.Pointer(&buf[0])), uintptr(unsafe.Pointer(&hdr)), dibRGBColors)
	if lines == 0 {
		return nil, errors.New("无法读取屏幕像素")
	}
	img := image.NewNRGBA(image.Rect(0, 0, w, h))
	for i := 0; i < len(buf); i += 4 {
		img.Pix[i] = buf[i+2]
		img.Pix[i+1] = buf[i+1]
		img.Pix[i+2] = buf[i]
		img.Pix[i+3] = 255
	}
	return img, nil
}

func systemMetric(index int) int {
	v, _, _ := procGetSystemMetrics.Call(uintptr(index))
	return int(int32(v))
}
