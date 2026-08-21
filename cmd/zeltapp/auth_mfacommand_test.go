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
	// The command can see ZELT_MFA_METHOD and ZELT_MFA_SINCE, and emit a code
	// derived from them (here we just echo the method-tagged code back).
	code, err := runMFACommand(`printf '%s\n' "0000${ZELT_MFA_SINCE: -2}"; test -n "$ZELT_MFA_METHOD"`, "email")
	if err != nil {
		t.Fatal(err)
	}
	if len(code) != 6 {
		t.Errorf("want 6-digit code, got %q", code)
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
