package main

import (
	"bufio"
	"strings"
	"testing"
)

func TestBootstrapRejectsMissingAndOversizedCredentials(t *testing.T) {
	for _, input := range []string{"", `{"token":"short"}` + "\n", strings.Repeat("x", 8192) + "\n"} {
		if _, err := bootstrap(bufio.NewReaderSize(strings.NewReader(input), 4096)); err == nil {
			t.Fatal("invalid bootstrap accepted")
		}
	}
}

func TestBootstrapLeavesPipeOpenForParentLifetime(t *testing.T) {
	input := `{"pipe":"\\\\.\\pipe\\ferry-core-0123456789abcdef0123456789abcdef","token":"` + strings.Repeat("a", 64) + `"}` + "\nstill open"
	reader := bufio.NewReaderSize(strings.NewReader(input), 4096)
	credentials, err := bootstrap(reader)
	if err != nil || len(credentials.Token) != 64 {
		t.Fatalf("bootstrap: %v", err)
	}
	rest, err := reader.ReadString('!')
	if rest != "still open" {
		t.Fatalf("bootstrap consumed parent lifetime stream: %q %v", rest, err)
	}
}
