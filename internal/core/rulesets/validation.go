package rulesets

import (
	"bytes"
	"compress/zlib"
	"context"
	"errors"
	"io"
	"regexp/syntax"

	"github.com/sagernet/sing-box/common/srs"
	C "github.com/sagernet/sing-box/constant"
	"github.com/sagernet/sing-box/option"
)

const maxDecompressed = 64 << 20

var errInvalidSRS = errors.New("invalid or unsupported geographic rule-set")

func validateSRS(data []byte) (rules option.PlainRuleSet, err error) {
	return validateSRSContext(context.Background(), data)
}

func validateSRSContext(ctx context.Context, data []byte) (rules option.PlainRuleSet, err error) {
	// Keep malformed upstream binaries from turning a decoder panic into a core
	// crash. Size/count checks below run before the official allocating decoder.
	defer func() {
		if recover() != nil {
			rules, err = option.PlainRuleSet{}, errInvalidSRS
		}
	}()
	if err = ctx.Err(); err != nil {
		return rules, err
	}
	payload, err := decompressSRS(ctx, data)
	if err != nil {
		return rules, err
	}
	if err = scanPayload(ctx, payload); err != nil {
		if ctx.Err() != nil {
			return rules, ctx.Err()
		}
		return rules, errInvalidSRS
	}
	if err = ctx.Err(); err != nil {
		return rules, err
	}
	decoded, err := srs.Read(bytes.NewReader(data), false)
	if err != nil {
		return rules, errInvalidSRS
	}
	if len(decoded.Options.Rules) == 0 {
		return rules, errInvalidSRS
	}
	budget := 65536
	if err = checkRegexes(decoded.Options.Rules, &budget); err != nil {
		return rules, err
	}
	return decoded.Options, ctx.Err()
}

func decompressSRS(ctx context.Context, data []byte) ([]byte, error) {
	if len(data) < 4 || len(data) > maxCompressed || string(data[:3]) != "SRS" || data[3] == 0 || data[3] > C.RuleSetVersionCurrent {
		return nil, errInvalidSRS
	}
	reader := bytes.NewReader(data[4:])
	compressed, err := zlib.NewReader(reader)
	if err != nil {
		return nil, errInvalidSRS
	}
	defer compressed.Close()
	payload, err := io.ReadAll(io.LimitReader(contextReader{ctx, compressed}, maxDecompressed+1))
	// Fully consume zlib to verify its checksum. srs.Read alone stops after the
	// declared rules and can accept missing checksums or trailing payload.
	if err != nil || len(payload) > maxDecompressed || reader.Len() != 0 {
		if ctx.Err() != nil {
			return nil, ctx.Err()
		}
		return nil, errInvalidSRS
	}
	return payload, nil
}

type contextReader struct {
	ctx    context.Context
	reader io.Reader
}

func (r contextReader) Read(p []byte) (int, error) {
	if err := r.ctx.Err(); err != nil {
		return 0, err
	}
	return r.reader.Read(p)
}

func checkRegexes(rules []option.HeadlessRule, budget *int) error {
	for _, rule := range rules {
		if rule.Type == C.RuleTypeLogical {
			if err := checkRegexes(rule.LogicalOptions.Rules, budget); err != nil {
				return err
			}
		} else if !rule.DefaultOptions.IsValid() {
			return errInvalidSRS
		}
		for _, expression := range rule.DefaultOptions.DomainRegex {
			parsed, err := syntax.Parse(expression, syntax.Perl)
			if err != nil {
				return errInvalidSRS
			}
			cost := regexCost(parsed, *budget)
			if cost > *budget {
				return errInvalidSRS
			}
			*budget -= cost
			if _, err := syntax.Compile(parsed.Simplify()); err != nil {
				return errInvalidSRS
			}
		}
	}
	return nil
}
