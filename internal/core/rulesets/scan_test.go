package rulesets

import (
	"bytes"
	"encoding/binary"
	"testing"

	C "github.com/sagernet/sing-box/constant"
	"github.com/sagernet/sing-box/option"
)

func TestSRSPreflightRejectsMalformedGeographicStructures(t *testing.T) {
	cases := map[string][]byte{
		"unknown-rule":        {1, 2},
		"empty-rule":          {1, 0, 255, 0},
		"invalid-bool":        {1, 0, 3, 1, 1, 'a', 255, 2},
		"duplicate-item":      {1, 0, 3, 0, 3, 0, 255, 0},
		"invalid-mode":        {1, 1, 2},
		"empty-logical":       {1, 1, 0, 0, 0},
		"truncated-logical":   {1, 1},
		"huge-matcher":        append([]byte{1, 0, 2, 0}, binary.AppendUvarint(nil, 1<<40)...),
		"bad-matcher-version": {1, 0, 2, 1},
		"empty-bitmap":        {1, 0, 2, 0, 0, 0, 0, 255, 0},
		"cyclic-bitmap":       append([]byte{1, 0, 2, 0, 0, 1}, append(binary.BigEndian.AppendUint64(nil, 0b101), []byte{1, 'a', 255, 0}...)...),
		"matcher-label-count": append([]byte{1, 0, 2, 0, 0, 1}, append(binary.BigEndian.AppendUint64(nil, 0b110), []byte{0, 255, 0}...)...),
		"bad-ip-version":      {1, 0, 6, 2},
		"huge-ip-count":       append([]byte{1, 0, 6, 1}, binary.BigEndian.AppendUint64(nil, 1<<40)...),
		"bad-ip-size":         append([]byte{1, 0, 6, 1}, append(binary.BigEndian.AppendUint64(nil, 1), bytes.Repeat([]byte{99}, 16)...)...),
		"reverse-range":       append([]byte{1, 0, 6, 1}, append(binary.BigEndian.AppendUint64(nil, 1), []byte{4, 10, 0, 0, 2, 4, 10, 0, 0, 1, 255, 0}...)...),
		"deep-logical":        append([]byte{1}, bytes.Repeat([]byte{1, 0, 1}, 102)...),
	}
	for name, payload := range cases {
		t.Run(name, func(t *testing.T) {
			if _, err := validateSRS(compressedSRS(t, payload)); err == nil {
				t.Fatal("malformed structure accepted")
			}
		})
	}
}

func TestInvalidDomainRegexCannotBePublished(t *testing.T) {
	data := writeSRS(t, option.PlainRuleSet{Rules: []option.HeadlessRule{{Type: C.RuleTypeDefault, DefaultOptions: option.DefaultHeadlessRule{DomainRegex: []string{"["}}}}})
	if _, err := validateSRS(data); err == nil {
		t.Fatal("invalid regex accepted for runtime")
	}
}

func TestDecodedRecordBudgetRejectsAmplification(t *testing.T) {
	payload := binary.AppendUvarint(nil, 16385)
	for range 16385 {
		payload = append(payload, 0, 3, 1, 1, 'a', 255, 0)
	}
	if _, err := validateSRS(compressedSRS(t, payload)); err == nil {
		t.Fatal("excessive decoded rules accepted")
	}
	payload = []byte{1, 0, 3}
	payload = binary.AppendUvarint(payload, 1<<20+1)
	payload = append(payload, make([]byte, 1<<20+1)...)
	payload = append(payload, 255, 0)
	if _, err := validateSRS(compressedSRS(t, payload)); err == nil {
		t.Fatal("excessive decoded strings accepted")
	}
}

func FuzzValidateSRS(f *testing.F) {
	f.Add([]byte("SRS\x05"))
	f.Add([]byte("invalid"))
	f.Fuzz(func(t *testing.T, data []byte) {
		if len(data) > 1<<20 {
			t.Skip()
		}
		rules, err := validateSRS(data)
		if err == nil && len(rules.Rules) > 16384 {
			t.Fatal("unbounded rule count accepted")
		}
	})
}
