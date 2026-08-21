package main

import (
	"strings"
	"testing"
)

func TestRunMFACommand_ExtractsCode(t *testing.T) {
	// stdout with surrounding noise - only the 6-digit code should be taken.
	code, err := runMFACommand("echo 'your code is 123456 thanks'", "email")
	if err != nil {
		t.Fatal(err)
	}
	if code != "123456" {
		t.Errorf("want 123456, got %q", code)
	}
}

func TestRunMFACommand_ExportsEnv(t *testing.T) {
	// The command can see ZELT_MFA_METHOD and ZELT_MFA_SINCE (POSIX sh - the
	// command runs via `sh -c`, which is dash on Linux CI). It only emits the
	// code when both are set.
	code, err := runMFACommand(`[ -n "$ZELT_MFA_METHOD" ] && [ -n "$ZELT_MFA_SINCE" ] && echo 654321`, "email")
	if err != nil {
		t.Fatal(err)
	}
	if code != "654321" {
		t.Errorf("want 654321, got %q", code)
	}
}

func TestRunMFACommand_NoCode(t *testing.T) {
	_, err := runMFACommand("echo nothing here", "email")
	if err == nil || !strings.Contains(err.Error(), "no 6-digit code") {
		t.Fatalf("want no-code error, got %v", err)
	}
}

func TestRunMFACommand_CommandFails(t *testing.T) {
	_, err := runMFACommand("exit 3", "email")
	if err == nil || !strings.Contains(err.Error(), "mfa-command failed") {
		t.Fatalf("want failure error, got %v", err)
	}
}
