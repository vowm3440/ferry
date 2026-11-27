package rulesets

import (
	"context"
	"errors"
	"io"
	"net/http"
	"time"
)

const maxCompressed = 16 << 20

func downloadClient(supplied *http.Client) *http.Client {
	client := &http.Client{Timeout: 12 * time.Second, CheckRedirect: checkRedirect}
	if supplied != nil {
		client.Transport = supplied.Transport
		if supplied.Timeout > 0 && supplied.Timeout < client.Timeout {
			client.Timeout = supplied.Timeout
		}
	}
	return client
}

func checkRedirect(request *http.Request, via []*http.Request) error {
	if len(via) >= 5 || request.URL.Scheme != "https" || request.URL.Host != "raw.githubusercontent.com" || request.URL.User != nil {
		return errors.New("rule-set redirect is not allowed")
	}
	for _, key := range []string{"Authorization", "Proxy-Authorization", "Cookie", "Referer"} {
		request.Header.Del(key)
	}
	return nil
}

func (m *Manager) fetch(ctx context.Context, address string) ([]byte, error) {
	request, err := http.NewRequestWithContext(ctx, http.MethodGet, address, nil)
	if err != nil {
		return nil, errors.New("cannot create rule-set download request")
	}
	request.Header.Set("Accept-Encoding", "identity")
	response, err := m.client.Do(request)
	if err != nil {
		return nil, fetchError(ctx, err)
	}
	defer response.Body.Close()
	encoding := response.Header.Get("Content-Encoding")
	if response.StatusCode != http.StatusOK || response.ContentLength > maxCompressed || (encoding != "" && encoding != "identity") {
		return nil, errors.New("rule-set response status, encoding, or size is invalid")
	}
	data, err := io.ReadAll(io.LimitReader(response.Body, maxCompressed+1))
	if err != nil {
		return nil, fetchError(ctx, err)
	}
	if len(data) > maxCompressed {
		return nil, errors.New("rule-set download exceeds size limit")
	}
	if err = ctx.Err(); err != nil {
		return nil, err
	}
	return data, nil
}

func fetchError(ctx context.Context, err error) error {
	if ctx.Err() != nil {
		return ctx.Err()
	}
	for _, target := range []error{context.Canceled, context.DeadlineExceeded} {
		if errors.Is(err, target) {
			return target
		}
	}
	return errors.New("rule-set download failed")
}
