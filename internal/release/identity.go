// Package release defines optional compile-time publisher pins for service binaries.
package release

// Production release engineering may populate this allowlist with Authenticode
// SPKI SHA-256 pins to layer certificate pinning on top of SHA-256 + token auth.
var signerPins = [...]string{}

func SignerPins() []string { return append([]string{}, signerPins[:]...) }
