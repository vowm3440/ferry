package app

import (
	"context"
	"encoding/base64"
	"errors"
	"ferry/internal/share"
	"strings"
)

func (s *Subs) QRSvg(_ context.Context, payload string, size int) (string, error) {
	return share.QRSVG(payload, size)
}
func (s *Subs) QRPNG(_ context.Context, payload string) (string, error) {
	data, err := share.QRPNG(payload)
	if err != nil {
		return "", err
	}
	return base64.StdEncoding.EncodeToString(data), nil
}
func (s *Session) SetScreenCapturer(fn func() (string, error)) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.screenCapture = fn
}

func (s *Subs) CaptureScreenQR(ctx context.Context) ([]Subscription, error) {
	s.session.mu.Lock()
	capture := s.session.screenCapture
	s.session.mu.Unlock()
	if capture == nil {
		capture = share.CaptureScreenQR
	}
	payload, err := capture()
	if err != nil {
		return nil, err
	}
	if strings.HasPrefix(payload, "https://") || strings.HasPrefix(payload, "http://") {
		return nil, errors.New("二维码包含订阅地址，请在 URL 导入入口确认来源后导入")
	}
	sub, err := s.AddFromText(ctx, "屏幕二维码导入", payload)
	if err != nil {
		return nil, err
	}
	return []Subscription{sub}, nil
}

func (s *Subs) ImportFromQR(ctx context.Context, encoded string) ([]Subscription, error) {
	if len(encoded) > 12<<20 {
		return nil, errors.New("二维码图片超过大小限制")
	}
	if strings.HasPrefix(encoded, "data:") {
		_, data, ok := strings.Cut(encoded, ",")
		if !ok {
			return nil, errors.New("图片编码无效")
		}
		encoded = data
	}
	data, err := base64.StdEncoding.DecodeString(encoded)
	if err != nil {
		return nil, errors.New("图片编码无效")
	}
	payload, err := share.DecodeQR(data)
	if err != nil {
		return nil, err
	}
	if strings.HasPrefix(payload, "https://") || strings.HasPrefix(payload, "http://") {
		return nil, errors.New("二维码包含订阅地址，请在 URL 导入入口确认来源后导入")
	}
	sub, err := s.AddFromText(ctx, "二维码导入", payload)
	if err != nil {
		return nil, err
	}
	return []Subscription{sub}, nil
}
