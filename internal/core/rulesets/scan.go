package rulesets

import (
	"bytes"
	"context"
	"encoding/binary"
	"io"
	"math/bits"
	"net/netip"
)

// This preflight only accepts the geographic fields used by the fixed catalog.
// All counts must fit the remaining input before srs.Read allocates anything.
// Rule and string/range budgets also limit decoded object amplification.
type scanner struct {
	*bytes.Reader
	ctx   context.Context
	rules int
	items uint64
}

func scanPayload(ctx context.Context, payload []byte) error {
	s := &scanner{Reader: bytes.NewReader(payload), ctx: ctx, rules: 16384, items: 1 << 20}
	count, err := s.count(1)
	if err != nil || count == 0 {
		return errInvalidSRS
	}
	for range count {
		if err = s.rule(0); err != nil {
			return err
		}
	}
	if s.Len() != 0 {
		return errInvalidSRS
	}
	return nil
}

func (s *scanner) count(width uint64) (uint64, error) {
	value, err := binary.ReadUvarint(s)
	if err != nil || value > uint64(s.Len())/width {
		return 0, errInvalidSRS
	}
	return value, nil
}

func (s *scanner) rule(depth int) error {
	if err := s.ctx.Err(); err != nil {
		return err
	}
	if depth > 100 || s.rules == 0 {
		return errInvalidSRS
	}
	s.rules--
	kind, err := s.ReadByte()
	if err != nil {
		return errInvalidSRS
	}
	switch kind {
	case 0:
		return s.defaultRule()
	case 1:
		return s.logicalRule(depth)
	default:
		return errInvalidSRS
	}
}

func (s *scanner) logicalRule(depth int) error {
	mode, err := s.ReadByte()
	if err != nil || mode > 1 {
		return errInvalidSRS
	}
	count, err := s.count(1)
	if err != nil || count == 0 {
		return errInvalidSRS
	}
	for range count {
		if err = s.rule(depth + 1); err != nil {
			return err
		}
	}
	return s.boolean()
}

func (s *scanner) defaultRule() error {
	seen := uint16(0)
	for {
		item, err := s.ReadByte()
		if err != nil {
			return errInvalidSRS
		}
		if item == 255 {
			if seen == 0 {
				return errInvalidSRS
			}
			return s.boolean()
		}
		if item > 6 || seen&(1<<item) != 0 {
			return errInvalidSRS
		}
		seen |= 1 << item
		switch item {
		case 2:
			err = s.matcher()
		case 3, 4:
			err = s.strings()
		case 6:
			err = s.ipSet()
		default:
			return errInvalidSRS
		}
		if err != nil {
			return err
		}
	}
}

func (s *scanner) boolean() error {
	value, err := s.ReadByte()
	if err != nil || value > 1 {
		return errInvalidSRS
	}
	return nil
}

func (s *scanner) strings() error {
	count, err := s.count(1)
	if err != nil || count > s.items {
		return errInvalidSRS
	}
	s.items -= count
	for range count {
		length, err := s.count(1)
		if err != nil || length > 4096 {
			return errInvalidSRS
		}
		if _, err = s.Seek(int64(length), io.SeekCurrent); err != nil {
			return errInvalidSRS
		}
	}
	return nil
}

func (s *scanner) matcher() error {
	version, err := s.ReadByte()
	if err != nil || version != 0 {
		return errInvalidSRS
	}
	leaves, err := s.count(8)
	if err != nil {
		return err
	}
	if _, err = s.Seek(int64(leaves*8), io.SeekCurrent); err != nil {
		return errInvalidSRS
	}
	words, err := s.count(8)
	if err != nil {
		return err
	}
	labels, err := s.bitmap(words)
	if err != nil {
		return err
	}
	length, err := s.count(1)
	if err != nil || length != labels {
		return errInvalidSRS
	}
	_, err = s.Seek(int64(length), io.SeekCurrent)
	return err
}

func (s *scanner) bitmap(words uint64) (uint64, error) {
	var labels uint64
	pending := int64(1)
	for index := uint64(0); index < words; index++ {
		var word uint64
		if err := binary.Read(s, binary.BigEndian, &word); err != nil {
			return 0, errInvalidSRS
		}
		width := 64
		if index == words-1 {
			width -= bits.LeadingZeros64(word)
		}
		for bit := 0; bit < width; bit++ {
			if pending == 0 {
				return 0, errInvalidSRS
			}
			if word&(1<<bit) == 0 {
				pending++
				labels++
			} else {
				pending--
			}
		}
	}
	if pending != 0 {
		return 0, errInvalidSRS
	}
	return labels, nil
}

func (s *scanner) ipSet() error {
	version, err := s.ReadByte()
	if err != nil || version != 1 {
		return errInvalidSRS
	}
	var count uint64
	if err = binary.Read(s, binary.BigEndian, &count); err != nil || count > uint64(s.Len())/10 || count > s.items {
		return errInvalidSRS
	}
	s.items -= count
	var previous netip.Addr
	for range count {
		from, err := s.address()
		if err != nil {
			return err
		}
		to, err := s.address()
		if err != nil || from.BitLen() != to.BitLen() || from.Compare(to) > 0 || (previous.IsValid() && previous.Compare(from) >= 0) {
			return errInvalidSRS
		}
		previous = to
	}
	return nil
}

func (s *scanner) address() (netip.Addr, error) {
	length, err := s.count(1)
	if err != nil || (length != 4 && length != 16) {
		return netip.Addr{}, errInvalidSRS
	}
	var data [16]byte
	if _, err = io.ReadFull(s, data[:length]); err != nil {
		return netip.Addr{}, errInvalidSRS
	}
	address, _ := netip.AddrFromSlice(data[:length])
	return address, nil
}
