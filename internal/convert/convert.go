// Package convert turns share links and subscription bodies into restricted
// RunSpec nodes. It never imports the core, and every node it returns has
// already passed the outbound allowlist. Anything it cannot represent exactly
// is skipped with a reason; parameters are never dropped silently.
package convert

import (
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"net"
	"strconv"
	"strings"
	"unicode"
	"unicode/utf8"

	"ferry/internal/spec"
	"ferry/internal/spec/policy"
)

// MaxBytes bounds pasted text and downloaded subscription bodies.
const MaxBytes = 1 << 20

const maxLabelBytes = 256

// Skipped explains why one line was not imported. Reason never quotes the
// line itself, because links carry credentials.
type Skipped struct {
	Line   int    `json:"line"`
	Reason string `json:"reason"`
}

type Result struct {
	Nodes   []spec.Node
	Groups  []spec.Group
	Skipped []Skipped
}

// ParseText accepts one link per line, or the same list wrapped in base64.
func ParseText(text string) (Result, error) {
	if len(text) > MaxBytes {
		return Result{}, errors.New("内容过大：不能超过 1 MiB")
	}
	if structuredDocument(text) {
		return parseDocument(text)
	}
	lines := splitLines(text)
	if !hasLink(lines) {
		decoded, _ := decodeBase64(strings.Join(strings.Fields(text), ""))
		lines = splitLines(decoded)
	}
	if !hasLink(lines) {
		return Result{}, errors.New("没有找到可导入的节点链接")
	}
	result := Result{Nodes: []spec.Node{}, Skipped: []Skipped{}}
	seen := make(map[string]bool)
	for index, line := range lines {
		if line == "" {
			continue
		}
		node, reason := parseLine(line)
		if reason == "" && seen[node.ID] {
			reason = "与前面的节点重复"
		}
		if reason != "" {
			result.Skipped = append(result.Skipped, Skipped{Line: index + 1, Reason: reason})
			continue
		}
		seen[node.ID] = true
		result.Nodes = append(result.Nodes, node)
	}
	if len(result.Nodes) > spec.MaxNodes {
		return Result{}, errors.New("节点数量超过限制（最多 1024 个）")
	}
	return result, nil
}

func splitLines(text string) []string {
	lines := strings.Split(strings.ReplaceAll(text, "\r\n", "\n"), "\n")
	for index, line := range lines {
		lines[index] = strings.TrimSpace(line)
	}
	return lines
}

func hasLink(lines []string) bool {
	for _, line := range lines {
		if strings.Contains(line, "://") {
			return true
		}
	}
	return false
}

func parseLine(line string) (spec.Node, string) {
	scheme, _, found := strings.Cut(line, "://")
	if !found || !plainWord(scheme) {
		return spec.Node{}, "无法识别的内容"
	}
	var draft draft
	var reason string
	switch strings.ToLower(scheme) {
	case "vless":
		draft, reason = parseVLESS(line)
	case "trojan":
		draft, reason = parseTrojan(line)
	case "ss":
		draft, reason = parseShadowsocks(line)
	case "socks", "socks5":
		draft, reason = parseSocks(line)
	case "vmess":
		draft, reason = parseVMess(line)
	case "hysteria2", "hy2", "tuic", "anytls":
		draft, reason = parseModernURL(line, strings.ToLower(scheme))
	case "wireguard", "wg":
		draft, reason = parseWireGuard(line)
	default:
		reason = "暂不支持 " + strings.ToLower(scheme) + " 协议"
	}
	if reason != "" {
		return spec.Node{}, reason
	}
	return finish(draft)
}

// draft is a parsed link before the allowlist has seen it.
type draft struct {
	name     string
	outbound map[string]any
}

func finish(d draft) (spec.Node, string) {
	data, err := json.Marshal(d.outbound)
	if err != nil {
		return spec.Node{}, "节点参数无法编码"
	}
	clean, err := policy.SanitizeOutbound(data)
	if err != nil {
		return spec.Node{}, "节点参数不在当前支持的范围内"
	}
	sum := sha256.Sum256(clean)
	return spec.Node{ID: "n-" + hex.EncodeToString(sum[:8]), Tag: label(d), Outbound: clean}, ""
}

func label(d draft) string {
	cleaned := strings.TrimSpace(strings.Map(func(r rune) rune {
		if unicode.IsControl(r) {
			return -1
		}
		return r
	}, d.name))
	if cleaned == "" || !utf8.ValidString(cleaned) {
		server, _ := d.outbound["server"].(string)
		port, _ := d.outbound["server_port"].(int)
		cleaned = server + ":" + strconv.Itoa(port)
	}
	for len(cleaned) > maxLabelBytes {
		_, size := utf8.DecodeLastRuneInString(cleaned)
		cleaned = cleaned[:len(cleaned)-size]
	}
	return cleaned
}

// plainWord keeps echoed scheme and parameter names free of user-controlled noise.
func plainWord(value string) bool {
	if value == "" || len(value) > 24 {
		return false
	}
	for _, c := range value {
		if !(c >= 'a' && c <= 'z') && !(c >= 'A' && c <= 'Z') && !(c >= '0' && c <= '9') && c != '-' && c != '_' {
			return false
		}
	}
	return true
}

func decodeBase64(value string) (string, bool) {
	trimmed := strings.TrimRight(value, "=")
	if trimmed == "" {
		return "", false
	}
	for _, encoding := range []*base64.Encoding{base64.RawURLEncoding, base64.RawStdEncoding} {
		if data, err := encoding.DecodeString(trimmed); err == nil && utf8.Valid(data) {
			return string(data), true
		}
	}
	return "", false
}

func splitHostPort(value string) (string, int, bool) {
	host, portText, err := net.SplitHostPort(value)
	if err != nil {
		return "", 0, false
	}
	port, convErr := strconv.Atoi(portText)
	return host, port, convErr == nil && port >= 1 && port <= 65535
}
