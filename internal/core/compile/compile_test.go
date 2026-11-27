package compile

import (
	"context"
	"encoding/json"
	"net"
	"os"
	"reflect"
	"testing"

	include "ferry/internal/core/boxinclude"
	"ferry/internal/spec"
	box "github.com/sagernet/sing-box"
	corejson "github.com/sagernet/sing/common/json"
	"github.com/sagernet/sing/service"
)

func fixture() spec.RunSpec {
	s := spec.Default()
	s.DNS.Mode = "redirhost"
	s.Nodes = []spec.Node{{ID: "n1", Tag: "测试节点", Outbound: json.RawMessage(`{"type":"socks","server":"127.0.0.1","server_port":1080}`)}}
	return s
}

func TestCompileGoldenAndPure(t *testing.T) {
	s := fixture()
	before, _ := json.Marshal(s)
	options, err := Compile(s)
	if err != nil {
		t.Fatal(err)
	}
	actual, err := corejson.Marshal(options)
	if err != nil {
		t.Fatal(err)
	}
	want, err := os.ReadFile("testdata/minimal.golden.json")
	if err != nil {
		t.Fatal(err)
	}
	var actualObject, wantObject any
	if err := json.Unmarshal(actual, &actualObject); err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal(want, &wantObject); err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(actualObject, wantObject) {
		t.Fatalf("编译结果不一致:\n%s", actual)
	}
	after, _ := json.Marshal(s)
	if string(before) != string(after) {
		t.Fatal("编译器修改了输入")
	}
}

func TestCompileAcceptedByBox(t *testing.T) {
	for _, mode := range []string{"redirhost", "fakeip"} {
		s := fixture()
		s.DNS.Mode = mode
		s.Groups = []spec.Group{{ID: "g", Name: "分组", Type: "select", Members: []string{"u"}}, {ID: "u", Name: "自动", Type: "urltest", Members: []string{"n1"}, Interval: "5m", TestURL: "https://example.com"}}
		options, err := Compile(s)
		if err != nil {
			t.Fatal(err)
		}
		ctx := service.ExtendContext(include.Context(context.Background()))
		instance, err := box.New(box.Options{Context: ctx, Options: options})
		if err != nil {
			t.Fatalf("%s: box.New拒绝: %v", mode, err)
		}
		if err := instance.Close(); err != nil {
			t.Fatal(err)
		}
	}
}

func TestCompileRejectsNoNodesAndUnsafeSpec(t *testing.T) {
	if _, err := Compile(spec.Default()); err == nil {
		t.Fatal("没有节点却可以启动")
	}
	s := fixture()
	s.Inbound.Tun.Enabled = true
	s.Inbound.Tun.Stack = "invalid"
	if _, err := Compile(s); err == nil {
		t.Fatal("允许了无效的 TUN 协议栈")
	}
}

func TestCompiledMixedStarts(t *testing.T) {
	s := fixture()
	listener, err := net.Listen("tcp4", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	s.Inbound.MixedPort = uint16(listener.Addr().(*net.TCPAddr).Port)
	listener.Close()
	s.DNS.Mode = "fakeip"
	options, err := Compile(s)
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithCancel(service.ExtendContext(include.Context(context.Background())))
	defer cancel()
	instance, err := box.New(box.Options{Context: ctx, Options: options})
	if err != nil {
		t.Fatal(err)
	}
	if err := instance.Start(); err != nil {
		t.Fatal(err)
	}
	if err := instance.Close(); err != nil {
		t.Fatal(err)
	}
}
