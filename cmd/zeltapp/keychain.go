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
	// -U updates if exists
	cmd := exec.Command(s.keychainCmd, "add-generic-password",
		"-a", email, "-s", keychainService, "-w", password, "-U")
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
