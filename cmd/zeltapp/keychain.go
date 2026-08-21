package main

import (
	"bytes"
	"errors"
	"os/exec"
	"strings"
)

// macOS Keychain credential storage via the `security` CLI. No-op on platforms
// without `/usr/bin/security`. The implementation lives on fileStore so the
// `store` interface stays single-implementation in production.

const keychainService = "zeltapp-cli"

func (s *fileStore) GetPassword(email string) (string, error) {
	if s.keychainCmd == "" {
		return "", errors.New("keychain unavailable")
	}
	cmd := exec.Command(s.keychainCmd, "find-generic-password",
		"-a", email, "-s", keychainService, "-w")
	var out, errb bytes.Buffer
	cmd.Stdout = &out
	cmd.Stderr = &errb
	if err := cmd.Run(); err != nil {
		if strings.Contains(errb.String(), "could not be found") {
			return "", errors.New("not in keychain")
		}
		return "", errors.New("keychain: " + errb.String())
	}
	return strings.TrimRight(out.String(), "\n"), nil
}

func (s *fileStore) SetPassword(email, password string) error {
	if s.keychainCmd == "" {
		return errors.New("keychain unavailable")
	}
	// Pipe the password via stdin rather than argv so it never appears in
	// `ps` output (review #4). `security add-generic-password -w` with no
	// inline value prompts for the password AND a retype confirmation, so we
	// must feed the value twice - sending it once stored a truncated/empty
	// value or failed with "passwords don't match".
	//
	// We also delete first: `-U` (update) is silently ignored when the value
	// comes from stdin rather than an inline `-w <value>`, so `add ... -U`
	// errors "already exists" on every re-login once an entry is present.
	// Delete-then-add is the only idempotent path that keeps the password out
	// of argv.
	_ = s.DeletePassword(email) // ignore "not found"
	cmd := exec.Command(s.keychainCmd, "add-generic-password",
		"-a", email, "-s", keychainService, "-w")
	cmd.Stdin = strings.NewReader(password + "\n" + password + "\n")
	if out, err := cmd.CombinedOutput(); err != nil {
		return errors.New("keychain set: " + strings.TrimSpace(string(out)))
	}
	return nil
}

func (s *fileStore) DeletePassword(email string) error {
	if s.keychainCmd == "" {
		return errors.New("keychain unavailable")
	}
	cmd := exec.Command(s.keychainCmd, "delete-generic-password",
		"-a", email, "-s", keychainService)
	if out, err := cmd.CombinedOutput(); err != nil {
		if strings.Contains(string(out), "could not be found") {
			return nil
		}
		return errors.New("keychain delete: " + strings.TrimSpace(string(out)))
	}
	return nil
}
