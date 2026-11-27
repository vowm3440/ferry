package convert

import "testing"

func TestClashExportDoesNotChangeSOCKSVersion(t *testing.T) {
	if _, err := exportClash(map[string]any{"type": "socks", "version": "4a", "server": "example.com", "server_port": 1080}); err == nil {
		t.Fatal("SOCKS4a silently exported as SOCKS5")
	}
}

func TestSingboxDirectImportRejectsUnrepresentableDialer(t *testing.T) {
	_, err := ParseText(`{"outbounds":[{"type":"socks","tag":"node","server":"127.0.0.1","server_port":1080},{"type":"direct","tag":"direct","bind_interface":"Ethernet"}]}`)
	if err == nil {
		t.Fatal("direct interface binding silently removed")
	}
}
