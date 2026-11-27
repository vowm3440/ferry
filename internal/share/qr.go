// Package share handles bounded, local sharing encodings without network side effects.
package share

import (
	"bytes"
	"errors"
	"fmt"
	"github.com/makiuchi-d/gozxing"
	"github.com/makiuchi-d/gozxing/qrcode"
	"image"
	_ "image/jpeg"
	_ "image/png"
	"rsc.io/qr"
	"strings"
)

func encode(payload string) (*qr.Code, error) {
	if payload == "" || len(payload) > 4096 {
		return nil, errors.New("二维码内容为空或过长")
	}
	code, err := qr.Encode(payload, qr.M)
	if err != nil {
		return nil, errors.New("内容超过二维码容量，请使用文件分享")
	}
	return code, nil
}

func QRPNG(payload string) ([]byte, error) {
	code, err := encode(payload)
	if err != nil {
		return nil, err
	}
	return code.PNG(), nil
}

func QRSVG(payload string, size int) (string, error) {
	if size < 128 || size > 1024 {
		return "", errors.New("二维码尺寸须在 128 到 1024 之间")
	}
	code, err := encode(payload)
	if err != nil {
		return "", err
	}
	var svg strings.Builder
	fmt.Fprintf(&svg, `<svg xmlns="http://www.w3.org/2000/svg" width="%d" height="%d" viewBox="0 0 %d %d" shape-rendering="crispEdges"><rect width="100%%" height="100%%" fill="white"/><path fill="black" d="`, size, size, code.Size+8, code.Size+8)
	for y := 0; y < code.Size; y++ {
		for x := 0; x < code.Size; x++ {
			if code.Black(x, y) {
				fmt.Fprintf(&svg, "M%d %dh1v1h-1z", x+4, y+4)
			}
		}
	}
	svg.WriteString(`"/></svg>`)
	return svg.String(), nil
}

func DecodeQR(data []byte) (string, error) {
	if len(data) == 0 || len(data) > 8<<20 {
		return "", errors.New("二维码图片为空或超过 8 MiB")
	}
	config, _, err := image.DecodeConfig(bytes.NewReader(data))
	if err != nil || config.Width < 1 || config.Height < 1 || config.Width > 8192 || config.Height > 8192 || int64(config.Width)*int64(config.Height) > 16<<20 {
		return "", errors.New("二维码图片格式或尺寸无效")
	}
	img, _, err := image.Decode(bytes.NewReader(data))
	if err != nil {
		return "", errors.New("二维码图片无法解码")
	}
	return DecodeQRImage(img)
}

func DecodeQRImage(img image.Image) (string, error) {
	if img == nil {
		return "", errors.New("二维码图片为空")
	}
	bounds := img.Bounds()
	w, h := bounds.Dx(), bounds.Dy()
	if w < 1 || h < 1 || w > 8192 || h > 8192 || int64(w)*int64(h) > 16<<20 {
		return "", errors.New("二维码图片格式或尺寸无效")
	}
	binary, err := gozxing.NewBinaryBitmapFromImage(img)
	if err != nil {
		return "", errors.New("二维码图片无法读取")
	}
	result, err := qrcode.NewQRCodeReader().Decode(binary, map[gozxing.DecodeHintType]interface{}{gozxing.DecodeHintType_TRY_HARDER: true})
	if err != nil || len(result.GetText()) > 1<<20 {
		return "", errors.New("未找到可读取的二维码")
	}
	return result.GetText(), nil
}
