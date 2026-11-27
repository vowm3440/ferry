package host

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

// A probe that takes longer than the 3 s control-request timeout used to be
// reported as a timeout even though the node answered.
func TestLatencyProbeOutlastsControlTimeout(t *testing.T) {
	var timeout string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		timeout = r.URL.Query().Get("timeout")
		time.Sleep(3500 * time.Millisecond)
		fmt.Fprint(w, `{"delay":3400}`)
	}))
	defer server.Close()
	client := server.Client()
	client.Timeout = 3 * time.Second
	h := New()
	h.current = &instance{address: strings.TrimPrefix(server.URL, "http://"), secret: "s", client: client, nodes: map[string]bool{"slow": true}}

	delays, err := h.LatencyTest(context.Background(), []string{"slow"})
	if err != nil || delays["slow"] != 3400 {
		t.Fatalf("slow but working node must report its delay: %v %v", delays, err)
	}
	if timeout != "5000" {
		t.Fatalf("probe timeout sent to the core: %q", timeout)
	}
}
