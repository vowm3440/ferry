//go:build !windows

package wintrust

func verifyNative(string, []spkiPin) (*VerifiedFile, error) {
	return nil, ErrUnsupported
}
