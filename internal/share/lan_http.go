package share

import (
	"context"
	"crypto/rand"
	"crypto/subtle"
	"encoding/base64"
	"errors"
	"io"
	"log"
	"net"
	"net/http"
	"net/url"
	"strconv"
	"sync"
	"time"
)

type lanSession struct {
	mu        sync.Mutex
	info      LANInfo
	active    bool
	path      string
	payload   []byte
	mediaType string
	tokens    float64
	lastToken time.Time
	server    *http.Server
	listener  *lanListener
	timer     *time.Timer
	done      chan struct{}
	closeOnce sync.Once
	closeErr  error
}

func newLANSession(listener net.Listener, data []byte, mediaType string, ttl time.Duration) *lanSession {
	var secret [32]byte
	rand.Read(secret[:]) // crypto/rand terminates the process if secure randomness is unavailable.
	path := "/s/" + base64.RawURLEncoding.EncodeToString(secret[:])
	now := time.Now()
	s := &lanSession{
		info:   LANInfo{Active: true, URL: (&url.URL{Scheme: "http", Host: listener.Addr().String(), Path: path}).String(), ExpiresAt: now.Add(ttl)},
		active: true, path: path, payload: append([]byte(nil), data...), mediaType: mediaType,
		tokens: 8, lastToken: now, done: make(chan struct{}), timer: time.NewTimer(ttl),
		listener: &lanListener{Listener: listener, slots: make(chan struct{}, 8)},
	}
	s.server = &http.Server{
		Handler: s, ReadHeaderTimeout: 2 * time.Second, ReadTimeout: 3 * time.Second,
		WriteTimeout: 5 * time.Second, IdleTimeout: 2 * time.Second, MaxHeaderBytes: 4096,
		ErrorLog: log.New(io.Discard, "", 0),
	}
	s.server.SetKeepAlivesEnabled(false)
	go s.serve()
	go s.expire()
	return s
}

func (s *lanSession) serve() {
	if err := s.server.Serve(s.listener); err != nil {
		s.close()
	}
}

func (s *lanSession) expire() {
	select {
	case <-s.timer.C:
		s.close()
	case <-s.done:
	}
}

func (s *lanSession) status() LANInfo {
	s.mu.Lock()
	defer s.mu.Unlock()
	if !s.active || !time.Now().Before(s.info.ExpiresAt) {
		return LANInfo{}
	}
	return s.info
}

func (s *lanSession) close() error {
	s.closeOnce.Do(func() {
		s.mu.Lock()
		s.active, s.payload, s.path = false, nil, ""
		s.info.URL = ""
		s.mu.Unlock()
		s.timer.Stop()
		close(s.done)
		listenerErr := s.listener.Close()
		if errors.Is(listenerErr, net.ErrClosed) {
			listenerErr = nil
		}
		s.closeErr = errors.Join(listenerErr, s.server.Close())
		if errors.Is(s.closeErr, net.ErrClosed) {
			s.closeErr = listenerErr
		}
	})
	return s.closeErr
}

func (s *lanSession) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Cache-Control", "no-store")
	w.Header().Set("Connection", "close")
	w.Header().Set("X-Content-Type-Options", "nosniff")
	w.Header().Set("Referrer-Policy", "no-referrer")
	if r.Context().Err() != nil {
		return
	}
	if !s.allowRequest() {
		w.Header().Set("Retry-After", "1")
		http.Error(w, "too many requests", http.StatusTooManyRequests)
		return
	}
	if r.Method != http.MethodGet {
		w.Header().Set("Allow", "GET")
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	if r.ContentLength != 0 || len(r.TransferEncoding) != 0 {
		http.Error(w, "request body not allowed", http.StatusRequestEntityTooLarge)
		return
	}
	data, status := s.claim(r.Context(), r.URL.EscapedPath(), r.URL.RawQuery)
	if status != http.StatusOK {
		http.Error(w, http.StatusText(status), status)
		return
	}
	defer s.close()
	w.Header().Set("Content-Type", s.mediaType)
	w.Header().Set("Content-Disposition", `attachment; filename="ferry-share.txt"`)
	w.Header().Set("Content-Length", strconv.Itoa(len(data)))
	if _, err := w.Write(data); err != nil {
		return
	}
	if err := http.NewResponseController(w).Flush(); err != nil {
		return
	}
}

func (s *lanSession) claim(ctx context.Context, path, query string) ([]byte, int) {
	s.mu.Lock()
	defer s.mu.Unlock()
	now := time.Now()
	if ctx.Err() != nil || !s.active || !now.Before(s.info.ExpiresAt) {
		return nil, http.StatusNotFound
	}
	if query != "" || subtle.ConstantTimeCompare([]byte(path), []byte(s.path)) != 1 {
		return nil, http.StatusNotFound
	}
	data := s.payload
	s.payload, s.active = nil, false
	return data, http.StatusOK
}

func (s *lanSession) allowRequest() bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	now := time.Now()
	s.tokens = min(8, s.tokens+now.Sub(s.lastToken).Seconds()*4)
	s.lastToken = now
	if s.tokens < 1 {
		return false
	}
	s.tokens--
	return true
}
