// Package policy enforces a small allowlist without importing the core.
package policy

import (
	"bytes"
	"encoding/json"
	"errors"
	"io"
)

var errJSON = errors.New("配置 JSON 无效、重复字段、空值或嵌套过深")

// CheckJSON rejects ambiguous input before either Go or sing-box decodes it.
func CheckJSON(data []byte, maxDepth int) error {
	d := json.NewDecoder(bytes.NewReader(data))
	d.UseNumber()
	first, err := d.Token()
	if err != nil || first != json.Delim('{') {
		return errJSON
	}
	if err := readObject(d, 1, maxDepth); err != nil {
		return err
	}
	if _, err := d.Token(); err != io.EOF {
		return errJSON
	}
	return nil
}

func readObject(d *json.Decoder, depth, limit int) error {
	if depth > limit {
		return errJSON
	}
	seen := make(map[string]bool)
	for d.More() {
		token, err := d.Token()
		key, ok := token.(string)
		if err != nil || !ok || seen[key] {
			return errJSON
		}
		seen[key] = true
		if err := readValue(d, depth+1, limit); err != nil {
			return err
		}
	}
	if token, err := d.Token(); err != nil || token != json.Delim('}') {
		return errJSON
	}
	return nil
}

func readValue(d *json.Decoder, depth, limit int) error {
	if depth > limit {
		return errJSON
	}
	token, err := d.Token()
	if err != nil || token == nil {
		return errJSON
	}
	switch token {
	case json.Delim('{'):
		return readObject(d, depth, limit)
	case json.Delim('['):
		for d.More() {
			if err := readValue(d, depth+1, limit); err != nil {
				return err
			}
		}
		if token, err := d.Token(); err != nil || token != json.Delim(']') {
			return errJSON
		}
	case json.Delim('}'), json.Delim(']'):
		return errJSON
	}
	return nil
}
