package spec

import (
	"strconv"
	"testing"
)

func TestReservedProxyNodeIDRejected(t *testing.T) {
	s := sample()
	s.Nodes[0].ID = "proxy"
	if err := s.Validate(); err == nil {
		t.Fatal("节点占用了默认选择分组名称")
	}
}

func TestDeepGroupGraphAlwaysRejected(t *testing.T) {
	s := sample()
	for i := 0; i < 18; i++ {
		member := "n1"
		if i < 17 {
			member = "g" + strconv.Itoa(i+1)
		}
		s.Groups = append(s.Groups, Group{ID: "g" + strconv.Itoa(i), Name: "G", Type: "select", Members: []string{member}})
	}
	for range 30 {
		if err := s.Validate(); err == nil {
			t.Fatal("分组深度超过16仍被接受")
		}
	}
}

func TestJSONFieldNamesAreExact(t *testing.T) {
	for _, data := range []string{`{"Mode":"direct"}`, `{"inbound":{"MixedPort":1080}}`} {
		if _, err := Decode([]byte(data)); err == nil {
			t.Fatal("接受了契约之外的大小写字段")
		}
	}
}
