package ipc

import (
	"bytes"
	"context"
	"crypto/subtle"
	"encoding/json"
	"errors"
	"io"
	"net"
	"net/http"
	"strings"
	"time"

	"ferry/internal/spec/policy"
)

// NewServiceHandler builds the IPC route mux without bearer-token middleware;
// callers wrap it with RequireVerifiedPeer and ServiceConnContext.
func NewServiceHandler(b Backend) http.Handler {
	return newRoutes(b)
}

func NewHandler(token string, b Backend) http.Handler {
	return authenticated(token, newRoutes(b))
}

func newRoutes(b Backend) http.Handler {
	mux := http.NewServeMux()
	registerExtraRoutes(mux, b)
	mux.HandleFunc("GET /v1/status", func(w http.ResponseWriter, r *http.Request) { writeJSON(w, 200, b.Status()) })
	mux.HandleFunc("GET /v1/traffic", func(w http.ResponseWriter, r *http.Request) {
		value, err := b.Snapshot(r.Context())
		if err != nil {
			failure(w)
			return
		}
		writeJSON(w, 200, value)
	})
	mux.HandleFunc("POST /v1/start", func(w http.ResponseWriter, r *http.Request) {
		var value json.RawMessage
		if !readJSON(w, r, &value) {
			return
		}
		complete(w, b.Start(r.Context(), value))
	})
	mux.HandleFunc("POST /v1/stop", func(w http.ResponseWriter, r *http.Request) {
		var value struct{}
		if !readJSON(w, r, &value) {
			return
		}
		complete(w, b.Stop(r.Context()))
	})
	mux.HandleFunc("POST /v1/mode", func(w http.ResponseWriter, r *http.Request) {
		var value ModeRequest
		if !readJSON(w, r, &value) {
			return
		}
		if value.Mode != "rule" && value.Mode != "global" && value.Mode != "direct" {
			writeError(w, 400, "代理模式无效")
			return
		}
		complete(w, b.SetMode(r.Context(), value.Mode))
	})
	mux.HandleFunc("POST /v1/select", func(w http.ResponseWriter, r *http.Request) {
		var value SelectRequest
		if !readJSON(w, r, &value) {
			return
		}
		if value.GroupID == "" || value.NodeID == "" {
			writeError(w, 400, "请选择节点与分组")
			return
		}
		complete(w, b.SelectNode(r.Context(), value.GroupID, value.NodeID))
	})
	return mux
}

func registerExtraRoutes(mux *http.ServeMux, b Backend) {
	registerRuleSets(mux, b)
	registerRules(mux, b)
	registerInspection(mux, b)
}

func authenticated(token string, next http.Handler) http.Handler {
	slots := make(chan struct{}, 8)
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		provided := strings.TrimPrefix(r.Header.Get("Authorization"), "Bearer ")
		if len(token) != 64 || subtle.ConstantTimeCompare([]byte(provided), []byte(token)) != 1 {
			writeError(w, 401, "内核连接未授权")
			return
		}
		select {
		case slots <- struct{}{}:
			defer func() { <-slots }()
		default:
			writeError(w, 429, "内核忙，请稍后重试")
			return
		}
		if r.ContentLength > MaxBodySize {
			writeError(w, 413, "配置过大")
			return
		}
		next.ServeHTTP(w, r)
	})
}

func readJSON(w http.ResponseWriter, r *http.Request, target any) bool {
	if !strings.HasPrefix(r.Header.Get("Content-Type"), "application/json") {
		writeError(w, 415, "需要 JSON 格式")
		return false
	}
	r.Body = http.MaxBytesReader(w, r.Body, MaxBodySize)
	data, err := io.ReadAll(r.Body)
	if err == nil {
		err = policy.CheckJSON(data, 16)
	}
	if err == nil {
		decoder := json.NewDecoder(bytes.NewReader(data))
		decoder.DisallowUnknownFields()
		err = decoder.Decode(target)
	}
	if err == nil {
		return true
	}
	var tooLarge *http.MaxBytesError
	if errors.As(err, &tooLarge) {
		writeError(w, 413, "配置过大")
	} else {
		writeError(w, 400, "配置格式无效")
	}
	return false
}

func complete(w http.ResponseWriter, err error) {
	if err != nil {
		failure(w)
		return
	}
	writeJSON(w, 200, struct{}{})
}

func failure(w http.ResponseWriter) {
	writeError(w, 500, "内核操作失败，请检查配置或端口占用")
}
func writeError(w http.ResponseWriter, code int, message string) {
	writeJSON(w, code, responseError{Error: message})
}

func writeJSON(w http.ResponseWriter, code int, value any) {
	data, err := json.Marshal(value)
	if err != nil {
		http.Error(w, "响应编码失败", 500)
		return
	}
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.Header().Set("Cache-Control", "no-store")
	w.WriteHeader(code)
	// A write failure means the peer is gone; no response channel remains.
	if _, err = w.Write(data); err != nil {
		return
	}
}

func Serve(ctx context.Context, listener net.Listener, handler http.Handler) error {
	server := &http.Server{Handler: handler, ReadHeaderTimeout: 3 * time.Second, ReadTimeout: 15 * time.Second, WriteTimeout: 20 * time.Second, IdleTimeout: 30 * time.Second, MaxHeaderBytes: 8 << 10}
	stopped := make(chan struct{})
	go func() {
		select {
		case <-ctx.Done():
			shutdown, cancel := context.WithTimeout(context.Background(), 3*time.Second)
			defer cancel()
			if err := server.Shutdown(shutdown); err != nil {
				_ = server.Close()
			}
		case <-stopped:
		}
	}()
	err := server.Serve(listener)
	close(stopped)
	if errors.Is(err, http.ErrServerClosed) {
		return nil
	}
	return err
}
