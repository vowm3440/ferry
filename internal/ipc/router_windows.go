//go:build windows

package ipc

import (
	"context"
	"encoding/json"
	"errors"
	"ferry/internal/telemetry"
	"net"
	"sync"
)

// Router directs GUI IPC calls to the running FerryCore Windows service when
// available, and transparently falls back to the unprivileged child process.
type Router struct {
	mu      sync.Mutex
	process *Process
	service *Client
	svcPID  uint32
	svcTok  string
}

func NewRouter(coreExecutable string) *Router {
	return &Router{process: NewProcess(coreExecutable)}
}

func (r *Router) serviceClient() *Client {
	info, err := QueryServiceStatus(ServiceName)
	if err != nil || !info.Installed || !info.Running || info.PID == 0 {
		r.dropServiceClient()
		return nil
	}
	paths, err := ServicePaths()
	if err != nil {
		return nil
	}
	auth, err := ReadClientAuth(paths.ClientAuthPath)
	if err != nil {
		return nil
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.service != nil && r.svcPID == info.PID && r.svcTok == auth.Token {
		return r.service
	}
	if r.service != nil {
		r.service.Close()
	}
	verifier, err := NewClientHashPeerVerifier(paths.ServiceCorePath, auth.CoreExeSHA256)
	if err != nil {
		return nil
	}
	pid := info.PID
	r.service = NewClientWithDialer(auth.Token, func(ctx context.Context) (net.Conn, error) {
		return DialServicePipe(ctx, ServicePipeName, pid, verifier)
	})
	r.svcPID, r.svcTok = pid, auth.Token
	_ = r.process.Close()
	return r.service
}

func (r *Router) dropServiceClient() {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.service != nil {
		r.service.Close()
		r.service = nil
		r.svcPID = 0
		r.svcTok = ""
	}
}

func (r *Router) StopUserProcess() error { return r.process.Close() }

func (r *Router) Close() error {
	r.dropServiceClient()
	return r.process.Close()
}

func (r *Router) Exited() bool {
	if r.serviceClient() != nil {
		return false
	}
	return r.process.Exited()
}

func (r *Router) Status(ctx context.Context) (State, error) {
	if svc := r.serviceClient(); svc != nil {
		return svc.Status(ctx)
	}
	return r.process.Status(ctx)
}

func (r *Router) Snapshot(ctx context.Context) (Traffic, error) {
	if svc := r.serviceClient(); svc != nil {
		return svc.Snapshot(ctx)
	}
	return r.process.Snapshot(ctx)
}

func (r *Router) Start(ctx context.Context, config json.RawMessage) error {
	if svc := r.serviceClient(); svc != nil {
		return svc.Start(ctx, config)
	}
	return r.process.Start(ctx, config)
}

func (r *Router) Stop(ctx context.Context) error {
	if svc := r.serviceClient(); svc != nil {
		return svc.Stop(ctx)
	}
	return r.process.Stop(ctx)
}

func (r *Router) SetMode(ctx context.Context, mode string) error {
	if svc := r.serviceClient(); svc != nil {
		return svc.SetMode(ctx, mode)
	}
	return r.process.SetMode(ctx, mode)
}

func (r *Router) SelectNode(ctx context.Context, group, node string) error {
	if svc := r.serviceClient(); svc != nil {
		return svc.SelectNode(ctx, group, node)
	}
	return r.process.SelectNode(ctx, group, node)
}

func (r *Router) LatencyTest(ctx context.Context, ids []string) (map[string]int, error) {
	if svc := r.serviceClient(); svc != nil {
		return svc.LatencyTest(ctx, ids)
	}
	return r.process.LatencyTest(ctx, ids)
}

func (r *Router) Connections(ctx context.Context) ([]telemetry.Connection, error) {
	if svc := r.serviceClient(); svc != nil {
		return svc.Connections(ctx)
	}
	return r.process.Connections(ctx)
}

func (r *Router) CloseConnection(ctx context.Context, id string) error {
	if svc := r.serviceClient(); svc != nil {
		return svc.CloseConnection(ctx, id)
	}
	return r.process.CloseConnection(ctx, id)
}

func (r *Router) Logs(ctx context.Context, after int64, limit int) ([]telemetry.LogEntry, error) {
	if svc := r.serviceClient(); svc != nil {
		return svc.Logs(ctx, after, limit)
	}
	return r.process.Logs(ctx, after, limit)
}

func (r *Router) SetLogLevel(ctx context.Context, level string) error {
	if svc := r.serviceClient(); svc != nil {
		return svc.SetLogLevel(ctx, level)
	}
	return r.process.SetLogLevel(ctx, level)
}

func (r *Router) TestRule(ctx context.Context, input telemetry.RuleTestRequest) (telemetry.RuleTestResult, error) {
	if svc := r.serviceClient(); svc != nil {
		return svc.TestRule(ctx, input)
	}
	return r.process.TestRule(ctx, input)
}

func (r *Router) RuleSetList(ctx context.Context) ([]telemetry.RuleSetInfo, error) {
	if svc := r.serviceClient(); svc != nil {
		return svc.RuleSetList(ctx)
	}
	return r.process.RuleSetList(ctx)
}

func (r *Router) RuleSetUpdate(ctx context.Context, id string) error {
	if svc := r.serviceClient(); svc != nil {
		return svc.RuleSetUpdate(ctx, id)
	}
	return r.process.RuleSetUpdate(ctx, id)
}

var _ = errors.New
