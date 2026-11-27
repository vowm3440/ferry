package update

import (
	"context"
	"errors"
	"io"
	"net/http"
	"time"
)

// Downloader stages authenticated updates. Its client never carries cookies,
// credentials, or caller-supplied redirect hooks into artifact requests.
type Downloader struct{ client *http.Client }

// NewDownloader uses the supplied client's transport and bounded timeout only.
// A nil client uses the system's default HTTPS transport. A custom transport is
// trusted configuration; tests can supply an httptest TLS client's transport.
func NewDownloader(client *http.Client) *Downloader {
	configured := &http.Client{Timeout: 5 * time.Minute, CheckRedirect: artifactRedirect}
	if client != nil {
		configured.Transport = client.Transport
		if client.Timeout > 0 && client.Timeout < configured.Timeout {
			configured.Timeout = client.Timeout
		}
	}
	return &Downloader{client: configured}
}

// Download stages an update with the default HTTPS transport. The destination
// must not exist; it is published atomically only after full integrity checks.
// The caller must use a directory it owns on a filesystem supporting hard links.
func Download(ctx context.Context, manifest Manifest, path string) error {
	return NewDownloader(nil).Download(ctx, manifest, path)
}

// Download stages an update using this downloader's configured transport.
func (d *Downloader) Download(ctx context.Context, manifest Manifest, path string) (err error) {
	if !manifest.verified {
		return errors.New("update download requires a verified manifest")
	}
	if err = ctx.Err(); err != nil {
		return err
	}
	stage, err := openStage(path)
	if err != nil {
		return err
	}
	defer func() { err = errors.Join(err, stage.cleanup()) }()
	body, err := d.fetch(ctx, manifest)
	if err != nil {
		return err
	}
	defer body.Close()
	if err = stage.write(ctx, body, manifest); err != nil {
		return err
	}
	return stage.publish(ctx)
}

func (d *Downloader) fetch(ctx context.Context, m Manifest) (io.ReadCloser, error) {
	request, err := http.NewRequestWithContext(ctx, http.MethodGet, m.url, nil)
	if err != nil {
		return nil, errors.New("unable to create update download request")
	}
	request.Header.Set("Accept-Encoding", "identity")
	client := d.client
	if client == nil {
		client = NewDownloader(nil).client
	}
	response, err := client.Do(request)
	if err != nil {
		return nil, downloadError(ctx, err)
	}
	encoding := response.Header.Get("Content-Encoding")
	if response.StatusCode != http.StatusOK || (encoding != "" && encoding != "identity") || (response.ContentLength >= 0 && response.ContentLength != m.size) {
		response.Body.Close()
		return nil, errors.New("update response has an invalid status, encoding, or size")
	}
	return response.Body, nil
}

func artifactRedirect(request *http.Request, via []*http.Request) error {
	if len(via) >= 5 {
		return errors.New("update download exceeded redirect limit")
	}
	if _, err := artifactURL(request.URL.String()); err != nil {
		return err
	}
	for _, header := range []string{"Authorization", "Proxy-Authorization", "Cookie", "Referer"} {
		request.Header.Del(header)
	}
	return nil
}

func downloadError(ctx context.Context, err error) error {
	if ctx.Err() != nil {
		return ctx.Err()
	}
	for _, target := range []error{context.Canceled, context.DeadlineExceeded} {
		if errors.Is(err, target) {
			return target
		}
	}
	return errors.New("update download failed")
}
