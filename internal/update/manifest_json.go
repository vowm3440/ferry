package update

import (
	"bytes"
	"encoding/json"
	"errors"
	"io"
	"strconv"
)

func decodeManifest(raw []byte) (Manifest, error) {
	if !validUnicodeEscapes(raw) {
		return Manifest{}, errors.New("update manifest has invalid Unicode escapes")
	}
	d := json.NewDecoder(bytes.NewReader(raw))
	token, err := d.Token()
	if err != nil || token != json.Delim('{') {
		return Manifest{}, errors.New("update manifest must be a JSON object")
	}
	m := Manifest{}
	fields := map[string]any{"version": &m.version, "channel": &m.channel,
		"url": &m.url, "sha256": &m.sha256, "size": &m.size}
	seen := make(map[string]bool, len(fields))
	for d.More() {
		if err = decodeManifestField(d, fields, seen); err != nil {
			return Manifest{}, err
		}
	}
	if token, err = d.Token(); err != nil || token != json.Delim('}') || len(seen) != len(fields) {
		return Manifest{}, errors.New("update manifest has missing or malformed fields")
	}
	if _, err = d.Token(); err != io.EOF {
		return Manifest{}, errors.New("update manifest has trailing data")
	}
	return m, nil
}

// encoding/json replaces unpaired UTF-16 surrogates with U+FFFD. Reject them
// so the authenticated document cannot silently name a different artifact URL.
func validUnicodeEscapes(raw []byte) bool {
	for i := 0; i < len(raw); i++ {
		if raw[i] != '\\' {
			continue
		}
		i++
		if i >= len(raw) || raw[i] != 'u' {
			continue
		}
		if i+5 > len(raw) {
			return false
		}
		code, err := strconv.ParseUint(string(raw[i+1:i+5]), 16, 16)
		if err != nil || (code >= 0xdc00 && code <= 0xdfff) {
			return false
		}
		i += 4
		if code < 0xd800 || code > 0xdbff {
			continue
		}
		if i+7 > len(raw) || raw[i+1] != '\\' || raw[i+2] != 'u' {
			return false
		}
		low, err := strconv.ParseUint(string(raw[i+3:i+7]), 16, 16)
		if err != nil || low < 0xdc00 || low > 0xdfff {
			return false
		}
		i += 6
	}
	return true
}

func decodeManifestField(d *json.Decoder, fields map[string]any, seen map[string]bool) error {
	token, err := d.Token()
	key, ok := token.(string)
	if err != nil || !ok || fields[key] == nil || seen[key] {
		return errors.New("update manifest has unknown or duplicate fields")
	}
	if d.Decode(fields[key]) != nil {
		return errors.New("update manifest field has an invalid type")
	}
	seen[key] = true
	return nil
}
