//go:build !windows

package store

import "errors"

func protect([]byte) ([]byte, error)   { return nil, errors.New("Windows DPAPI is required") }
func unprotect([]byte) ([]byte, error) { return nil, errors.New("Windows DPAPI is required") }
