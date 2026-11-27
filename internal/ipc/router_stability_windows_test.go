//go:build windows

package ipc

import (
	"context"
	"encoding/json"
	"fmt"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"ferry/internal/telemetry"
)

type fakeSvcBackend struct {
	mu       sync.Mutex
	status   string
	stops    atomic.Int32
	starts   atomic.Int32
	logs     []telemetry.LogEntry
	nextSeq  atomic.Int64
}

func newFakeSvcBackend() *fakeSvcBackend {
	return &fakeSvcBackend{status: "stopped"}
}

func (f *fakeSvcBackend) Status() State {
	f.mu.Lock()
	defer f.mu.Unlock()
	return State{Status: f.status, Mode: "rule", ServiceMode: true, Version: "0.1.0"}
}
func (f *fakeSvcBackend) Start(context.Context, json.RawMessage) error {
	f.starts.Add(1)
	f.mu.Lock()
	f.status = "running"
	f.mu.Unlock()
	seq := f.nextSeq.Add(1)
	f.mu.Lock()
	f.logs = append(f.logs, telemetry.LogEntry{Seq: seq, Time: time.Now().UnixMilli(), Level: "info", Message: fmt.Sprintf("started-%d", seq)})
	f.mu.Unlock()
	return nil
}
func (f *fakeSvcBackend) Stop(context.Context) error {
	f.stops.Add(1)
	f.mu.Lock()
	f.status = "stopped"
	f.mu.Unlock()
	return nil
}
func (f *fakeSvcBackend) SetMode(context.Context, string) error         { return nil }
func (f *fakeSvcBackend) SelectNode(context.Context, string, string) error { return nil }
func (f *fakeSvcBackend) Snapshot(context.Context) (Traffic, error) {
	return Traffic{UpHistory: make([]int64, 60), DownHistory: make([]int64, 60)}, nil
}
func (f *fakeSvcBackend) LatencyTest(context.Context, []string) (map[string]int, error) {
	return map[string]int{}, nil
}
func (f *fakeSvcBackend) Connections(context.Context) ([]telemetry.Connection, error) {
	return []telemetry.Connection{}, nil
}
func (f *fakeSvcBackend) CloseConnection(context.Context, string) error { return nil }
func (f *fakeSvcBackend) Logs(_ context.Context, after int64, limit int) ([]telemetry.LogEntry, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	var out []telemetry.LogEntry
	for _, entry := range f.logs {
		if entry.Seq > after {
			out = append(out, entry)
			if len(out) >= limit {
				break
			}
		}
	}
	return out, nil
}
func (f *fakeSvcBackend) SetLogLevel(context.Context, string) error { return nil }
func (f *fakeSvcBackend) TestRule(context.Context, telemetry.RuleTestRequest) (telemetry.RuleTestResult, error) {
	return telemetry.RuleTestResult{}, nil
}
func (f *fakeSvcBackend) RuleSetList(context.Context) ([]telemetry.RuleSetInfo, error) {
	return nil, nil
}
func (f *fakeSvcBackend) RuleSetUpdate(context.Context, string) error { return nil }

func serveFakeService(t *testing.T, tok string, b Backend) (string, func()) {
	t.Helper()
	pipe := randomTestPipe(t)
	sid := currentUserSID(t)
	v := &fakeVerifier{allowClient: true, allowServer: true}
	ln, err := listenServicePipeSDDL(pipe, "D:P(A;;GA;;;"+sid+")", v)
	if err != nil {
		t.Fatal(err)
	}
	srv := &http.Server{
		Handler:     RequireVerifiedPeerWithToken(tok, NewServiceHandler(b)),
		ConnContext: ServiceConnContext,
	}
	go func() { _ = srv.Serve(ln) }()
	return pipe, func() {
		ctx, cancel := context.WithTimeout(context.Background(), time.Second)
		defer cancel()
		_ = srv.Shutdown(ctx)
	}
}

func TestRouterCloseStopsActiveServiceAndExitedDetectsServicePIDChange(t *testing.T) {
	tok := string(make([]byte, 64))
	for i := range 64 {
		tok = tok[:i] + "a" + tok[i+1:]
	}
	b1 := newFakeSvcBackend()
	pipe1, close1 := serveFakeService(t, tok, b1)
	defer close1()
	b2 := newFakeSvcBackend()
	pipe2, close2 := serveFakeService(t, tok, b2)
	defer close2()

	var pid atomic.Uint32
	pid.Store(uint32(os.Getpid()))
	var usePipe2 atomic.Bool

	r := NewRouter(filepath.Join(t.TempDir(), "ferry-core.exe"))
	r.findService = func() (ServiceInfo, InstallPaths, ClientAuth, bool) {
		return ServiceInfo{Installed: true, Running: true, StartType: "auto", PID: pid.Load()},
			InstallPaths{ServiceCorePath: `C:\ProgramData\Ferry\service\ferry-core.exe`},
			ClientAuth{CoreExeSHA256: tok, Token: tok},
			true
	}
	r.dialService = func(expectedPID uint32, _ InstallPaths, auth ClientAuth) (*Client, error) {
		target := pipe1
		if usePipe2.Load() {
			target = pipe2
		}
		v := &fakeVerifier{allowClient: true, allowServer: true}
		client := NewClientWithDialer(auth.Token, func(ctx context.Context) (net.Conn, error) {
			return DialServicePipe(ctx, target, expectedPID, v)
		})
		if _, err := client.Status(context.Background()); err != nil {
			client.Close()
			return nil, err
		}
		return client, nil
	}

	ctx := context.Background()
	if err := r.Start(ctx, json.RawMessage(`{}`)); err != nil {
		t.Fatalf("Router.Start failed: %v", err)
	}
	logs1, err := r.Logs(ctx, 0, 50)
	if err != nil || len(logs1) != 1 || logs1[0].Seq != 1 {
		t.Fatalf("unexpected initial service logs: %+v %v", logs1, err)
	}

	// Simulate FerryCore crashing and SCM restarting it with a new PID (seq resets to 1).
	usePipe2.Store(true)
	r.dialService = func(expectedPID uint32, _ InstallPaths, auth ClientAuth) (*Client, error) {
		v := &fakeVerifier{allowClient: true, allowServer: true}
		client := NewClientWithDialer(auth.Token, func(ctx context.Context) (net.Conn, error) {
			return DialServicePipe(ctx, pipe2, uint32(os.Getpid()), v)
		})
		if _, err := client.Status(context.Background()); err != nil {
			client.Close()
			return nil, err
		}
		_ = expectedPID
		return client, nil
	}
	pid.Store(uint32(os.Getpid()) + 1)

	if !r.Exited() {
		t.Fatal("expected Router.Exited() to report true after service PID changed while active")
	}
	if r.Exited() {
		t.Fatal("Router.Exited() must clear replacement latch after reporting once")
	}
	if err := r.Start(ctx, json.RawMessage(`{}`)); err != nil {
		t.Fatalf("recovery Router.Start failed: %v", err)
	}
	logs2, err := r.Logs(ctx, logs1[0].Seq, 50)
	if err != nil || len(logs2) != 1 || logs2[0].Seq <= logs1[0].Seq {
		t.Fatalf("expected strictly monotonic log Seq across service restart, got prev=%+v next=%+v err=%v", logs1, logs2, err)
	}

	if err := r.Close(); err != nil {
		t.Fatalf("Router.Close failed: %v", err)
	}
	if b2.stops.Load() != 1 {
		t.Fatalf("expected Router.Close to send Stop to active service, got stops=%d", b2.stops.Load())
	}
}

func TestRealCoreProcessStressRestartAndCleanShutdown(t *testing.T) {
	executable := os.Getenv("FERRY_TEST_CORE")
	if executable == "" {
		t.Skip("requires built core subprocess")
	}
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	port := ln.Addr().(*net.TCPAddr).Port
	_ = ln.Close()

	cfg := json.RawMessage(fmt.Sprintf(`{"nodes":[{"id":"local","tag":"test","outbound":{"type":"socks","server":"127.0.0.1","server_port":1080}}],"inbound":{"mixedPort":%d},"log":{"level":"debug"}}`, port))
	p := NewProcess(executable)
	defer p.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 25*time.Second)
	defer cancel()
	if err := p.Start(ctx, cfg); err != nil {
		t.Fatalf("initial Start failed: %v", err)
	}
	conn, err := net.DialTimeout("tcp", fmt.Sprintf("127.0.0.1:%d", port), 2*time.Second)
	if err != nil {
		t.Fatalf("mixed port unreachable after Start: %v", err)
	}
	_ = conn.Close()

	var wg sync.WaitGroup
	for range 6 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			_, _ = p.Status(ctx)
			_, _ = p.Snapshot(ctx)
			_, _ = p.Connections(ctx)
			_, _ = p.Logs(ctx, 0, 100)
			_ = p.SetMode(ctx, "global")
		}()
	}
	wg.Wait()

	logs1, err := p.Logs(ctx, 0, 200)
	if err != nil || len(logs1) == 0 {
		t.Fatalf("expected initial logs, got %d err=%v", len(logs1), err)
	}
	lastSeq := logs1[len(logs1)-1].Seq

	// Kill subprocess to simulate crash, verify Exited(), then recover via Start().
	if err := p.cmd.Process.Kill(); err != nil {
		t.Fatal(err)
	}
	<-p.done
	if !p.Exited() {
		t.Fatal("expected Process.Exited() == true after kill")
	}
	if err := p.Start(ctx, cfg); err != nil {
		t.Fatalf("recovery Start failed: %v", err)
	}
	logs2, err := p.Logs(ctx, lastSeq, 200)
	if err != nil || len(logs2) == 0 || logs2[0].Seq <= lastSeq {
		t.Fatalf("expected monotonic logs after crash recovery, lastSeq=%d got=%+v err=%v", lastSeq, logs2, err)
	}
	if err := p.Close(); err != nil {
		t.Fatalf("Process.Close failed: %v", err)
	}
	// Verify port is released immediately after Close.
	checkLn, err := net.Listen("tcp", fmt.Sprintf("127.0.0.1:%d", port))
	if err != nil {
		t.Fatalf("mixed port still occupied after Close: %v", err)
	}
	_ = checkLn.Close()
}
