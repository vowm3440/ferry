//go:build windows

package wintrust

import "errors"

func verifyNative(path string, pins []spkiPin) (_ *VerifiedFile, resultErr error) {
	file, err := openLockedFile(path)
	if err != nil {
		return nil, err
	}
	keepOpen := false
	defer func() {
		if !keepOpen {
			resultErr = joinCleanup(resultErr, file.close())
		}
	}()
	der, err := verifiedSignerCertificate(file.finalPath, file.handle)
	if err != nil {
		return nil, err
	}
	pin, err := validateSignerDER(der, pins)
	if err != nil {
		return nil, err
	}
	verified := &VerifiedFile{finalPath: file.finalPath, identity: file.identity,
		signerSPKI: pin, release: &fileRelease{close: file.close}}
	keepOpen = true
	return verified, nil
}

func joinCleanup(original, cleanup error) error {
	if cleanup == nil {
		return original
	}
	return errors.Join(original, cleanup)
}
