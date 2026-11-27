//go:build windows

package wintrust

import (
	"fmt"
	"unsafe"

	"golang.org/x/sys/windows"
)

var (
	trustLibrary       = windows.NewLazySystemDLL("wintrust.dll")
	providerDataProc   = trustLibrary.NewProc("WTHelperProvDataFromStateData")
	providerSignerProc = trustLibrary.NewProc("WTHelperGetProvSignerFromChain")
	providerCertProc   = trustLibrary.NewProc("WTHelperGetProvCertFromChain")
)

// Only the leading fields of CRYPT_PROVIDER_CERT are needed. Pointer alignment
// is native, matching Windows on both 32- and 64-bit architectures.
type providerCertificate struct {
	size    uint32
	context uintptr
}

// Leading CERT_CONTEXT fields; addresses stay integers until ReadProcessMemory
// copies the provider-owned bytes into Go-owned memory.
type certificateContext struct {
	encoding uint32
	encoded  uintptr
	length   uint32
}

func verifiedSignerCertificate(path string, file windows.Handle) (_ []byte, resultErr error) {
	name, err := windows.UTF16PtrFromString(path)
	if err != nil {
		return nil, ErrFileAccess
	}
	info := windows.WinTrustFileInfo{Size: uint32(unsafe.Sizeof(windows.WinTrustFileInfo{})),
		FilePath: name, File: file}
	data := trustData(&info)
	defer func() {
		data.StateAction = windows.WTD_STATEACTION_CLOSE
		if err := windows.WinVerifyTrustEx(windows.InvalidHWND, &windows.WINTRUST_ACTION_GENERIC_VERIFY_V2, &data); err != nil {
			resultErr = joinCleanup(resultErr, ErrCleanup)
		}
	}()
	if err := windows.WinVerifyTrustEx(windows.InvalidHWND, &windows.WINTRUST_ACTION_GENERIC_VERIFY_V2, &data); err != nil {
		return nil, trustError(err)
	}
	return signerDERFromState(data.StateData)
}

func trustData(info *windows.WinTrustFileInfo) windows.WinTrustData {
	// No online retrieval: revocation is checked for the chain excluding the
	// root using cached data. Missing/stale revocation evidence fails closed.
	// Do not use REVOCATION_CHECK_NONE, trust-error exceptions or lifetime flags;
	// WinVerifyTrust applies the system's trusted Authenticode timestamp policy.
	return windows.WinTrustData{
		Size: uint32(unsafe.Sizeof(windows.WinTrustData{})), UIChoice: windows.WTD_UI_NONE,
		RevocationChecks: windows.WTD_REVOKE_WHOLECHAIN, UnionChoice: windows.WTD_CHOICE_FILE,
		StateAction: windows.WTD_STATEACTION_VERIFY, FileOrCatalogOrBlobOrSgnrOrCert: unsafe.Pointer(info),
		ProvFlags: windows.WTD_CACHE_ONLY_URL_RETRIEVAL | windows.WTD_REVOCATION_CHECK_CHAIN_EXCLUDE_ROOT |
			windows.WTD_DISABLE_MD2_MD4, UIContext: windows.WTD_UICONTEXT_EXECUTE,
	}
}

func signerDERFromState(state windows.Handle) ([]byte, error) {
	if state == 0 {
		return nil, ErrSignerCertificate
	}
	provider, _, _ := providerDataProc.Call(uintptr(state))
	if provider == 0 {
		return nil, ErrSignerCertificate
	}
	signer, _, _ := providerSignerProc.Call(provider, 0, 0, 0)
	if signer == 0 {
		return nil, ErrSignerCertificate
	}
	certificate, _, _ := providerCertProc.Call(signer, 0)
	if certificate == 0 {
		return nil, ErrSignerCertificate
	}
	var entry providerCertificate
	if err := copyNativeMemory(certificate, unsafe.Pointer(&entry), unsafe.Sizeof(entry)); err != nil {
		return nil, err
	}
	if entry.size < uint32(unsafe.Sizeof(entry)) || entry.context == 0 {
		return nil, ErrSignerCertificate
	}
	return signerDERFromContext(entry.context)
}

func signerDERFromContext(address uintptr) ([]byte, error) {
	var context certificateContext
	if err := copyNativeMemory(address, unsafe.Pointer(&context), unsafe.Sizeof(context)); err != nil {
		return nil, err
	}
	if context.encoded == 0 || context.length == 0 || context.length > 1<<20 {
		return nil, ErrSignerCertificate
	}
	der := make([]byte, int(context.length))
	// Copy before WTD_STATEACTION_CLOSE releases the provider's memory.
	if err := copyNativeMemory(context.encoded, unsafe.Pointer(&der[0]), uintptr(len(der))); err != nil {
		return nil, err
	}
	return der, nil
}

func copyNativeMemory(address uintptr, destination unsafe.Pointer, size uintptr) error {
	var copied uintptr
	err := windows.ReadProcessMemory(windows.CurrentProcess(), address, (*byte)(destination), size, &copied)
	if err != nil || copied != size {
		return ErrSignerCertificate
	}
	return nil
}

func trustError(err error) error {
	if code, ok := err.(windows.Errno); ok {
		return &nativeTrustError{code: code}
	}
	return ErrTrust
}

type nativeTrustError struct{ code windows.Errno }

func (err *nativeTrustError) Error() string {
	return fmt.Sprintf("%s（状态码 0x%08x）", ErrTrust, uint32(err.code))
}

func (err *nativeTrustError) Unwrap() error { return ErrTrust }
