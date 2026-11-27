//go:build windows

package main

import "testing"

func TestUserModeRejectsServiceAndBuiltInServiceAccounts(t *testing.T) {
	for _, identity := range []struct {
		sid     string
		service bool
	}{{"S-1-5-18", false}, {"S-1-5-19", false}, {"S-1-5-20", false}, {"S-1-5-21-123", true}} {
		if err := validateUserIdentity(identity.sid, identity.service); err == nil {
			t.Fatal("privileged service entered user-mode protocol")
		}
	}
	if err := validateUserIdentity("S-1-5-21-123", false); err != nil {
		t.Fatal(err)
	}
}
