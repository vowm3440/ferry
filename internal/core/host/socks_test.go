//go:build with_clash_api

package host

import (
	"bytes"
	"encoding/binary"
	"encoding/json"
	"io"
	"net"
	"strconv"
	"sync"
	"sync/atomic"
	"testing"

	"ferry/internal/spec"
)

type socksServer struct {
	net.Listener
	requests    atomic.Int64
	mu          sync.Mutex
	connections map[net.Conn]bool
}

func newSOCKS(t *testing.T) *socksServer {
	t.Helper()
	listener, err := net.Listen("tcp4", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	s := &socksServer{Listener: listener, connections: make(map[net.Conn]bool)}
	t.Cleanup(func() {
		listener.Close()
		s.mu.Lock()
		defer s.mu.Unlock()
		for conn := range s.connections {
			conn.Close()
		}
	})
	go func() {
		for {
			conn, err := listener.Accept()
			if err != nil {
				return
			}
			s.mu.Lock()
			s.connections[conn] = true
			s.mu.Unlock()
			go s.serve(conn)
		}
	}()
	return s
}

func (s *socksServer) serve(conn net.Conn) {
	defer conn.Close()
	defer func() { s.mu.Lock(); delete(s.connections, conn); s.mu.Unlock() }()
	var greeting [2]byte
	if _, err := io.ReadFull(conn, greeting[:]); err != nil {
		return
	}
	if _, err := io.CopyN(io.Discard, conn, int64(greeting[1])); err != nil {
		return
	}
	if _, err := conn.Write([]byte{5, 0}); err != nil {
		return
	}
	destination, err := socksDestination(conn)
	if err != nil {
		return
	}
	target, err := net.Dial("tcp", destination)
	if err != nil {
		return
	}
	defer target.Close()
	s.requests.Add(1)
	if _, err := conn.Write([]byte{5, 0, 0, 1, 127, 0, 0, 1, 0, 0}); err != nil {
		return
	}
	go func() { _, _ = io.Copy(target, conn); target.Close() }()
	_, _ = io.Copy(conn, target)
}

func socksDestination(conn net.Conn) (string, error) {
	var request [4]byte
	if _, err := io.ReadFull(conn, request[:]); err != nil {
		return "", err
	}
	var host string
	if request[3] == 3 {
		var size [1]byte
		if _, err := io.ReadFull(conn, size[:]); err != nil {
			return "", err
		}
		data := make([]byte, int(size[0]))
		if _, err := io.ReadFull(conn, data); err != nil {
			return "", err
		}
		host = string(data)
	} else {
		size := 4
		if request[3] == 4 {
			size = 16
		}
		data := make([]byte, size)
		if _, err := io.ReadFull(conn, data); err != nil {
			return "", err
		}
		host = net.IP(data).String()
	}
	var port [2]byte
	if _, err := io.ReadFull(conn, port[:]); err != nil {
		return "", err
	}
	return net.JoinHostPort(host, strconv.Itoa(int(binary.BigEndian.Uint16(port[:])))), nil
}

func hostFixture(t *testing.T, port int) spec.RunSpec {
	t.Helper()
	listener, err := net.Listen("tcp4", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	s := spec.Default()
	s.Inbound.MixedPort = uint16(listener.Addr().(*net.TCPAddr).Port)
	listener.Close()
	s.Nodes = []spec.Node{{ID: "n1", Tag: "本机节点", Outbound: json.RawMessage(`{"type":"socks","server":"127.0.0.1","server_port":` + strconv.Itoa(port) + `}`)}}
	return s
}

func containsSecret(data []byte, secret string) bool { return bytes.Contains(data, []byte(secret)) }
