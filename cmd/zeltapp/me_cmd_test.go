package main

import (
	"fmt"
	"testing"
)

// stubMeEndpoints registers /apiv2/users/{id}/{path} -> "{}" for each path.
func stubMeEndpoints(st *serverState, paths ...string) {
	for _, p := range paths {
		st.jsonRoute("GET", fmt.Sprintf("/apiv2/users/%d/%s", fakeUserID, p), 200, []byte(`{}`))
	}
}

func TestCmd_MeContact(t *testing.T) {
	srv, st, _, _ := withTestEnv(t)
	_ = srv
	stubMeEndpoints(st, "address", "emergency-contact", "work-contact", "family", "family/members")
	if err := runCmd(t, "me", "contact"); err != nil {
		t.Fatal(err)
	}
}

func TestCmd_MeEmployment(t *testing.T) {
	srv, st, _, _ := withTestEnv(t)
	_ = srv
	stubMeEndpoints(st, "role", "contracts", "contracts/current", "lifecycle", "events",
		"right-work/documents")
	st.jsonRoute("GET", fmt.Sprintf("/apiv2/users/%d/summary", fakeUserID), 200, []byte(`{}`))
	if err := runCmd(t, "me", "employment"); err != nil {
		t.Fatal(err)
	}
}

func TestCmd_MeCompensation(t *testing.T) {
	srv, st, _, _ := withTestEnv(t)
	_ = srv
	stubMeEndpoints(st, "compensation")
	if err := runCmd(t, "me", "compensation"); err != nil {
		t.Fatal(err)
	}
}

func TestCmd_MeBank(t *testing.T) {
	srv, st, _, _ := withTestEnv(t)
	_ = srv
	stubMeEndpoints(st, "bank-accounts")
	if err := runCmd(t, "me", "bank"); err != nil {
		t.Fatal(err)
	}
}

func TestCmd_MeEquity(t *testing.T) {
	srv, st, _, _ := withTestEnv(t)
	_ = srv
	stubMeEndpoints(st, "equity")
	if err := runCmd(t, "me", "equity"); err != nil {
		t.Fatal(err)
	}
}

func TestCmd_MePayslips(t *testing.T) {
	srv, st, _, _ := withTestEnv(t)
	_ = srv
	stubMeEndpoints(st, "payrolls", "payslips")
	if err := runCmd(t, "me", "payslips"); err != nil {
		t.Fatal(err)
	}
}

func TestCmd_MeDevices(t *testing.T) {
	srv, st, _, _ := withTestEnv(t)
	_ = srv
	st.jsonRoute("GET", fmt.Sprintf("/apiv2/devices/users/%d", fakeUserID), 200, []byte(`{}`))
	st.jsonRoute("GET", fmt.Sprintf("/apiv2/devices/users/%d/in-transit", fakeUserID), 200, []byte(`{}`))
	st.jsonRoute("GET", fmt.Sprintf("/apiv2/devices/orders/users/%d", fakeUserID), 200, []byte(`{}`))
	if err := runCmd(t, "me", "devices"); err != nil {
		t.Fatal(err)
	}
}

func TestCmd_MeBenefits(t *testing.T) {
	srv, st, _, _ := withTestEnv(t)
	_ = srv
	st.jsonRoute("GET", fmt.Sprintf("/apiv2/custom-benefit/by-user/%d/effective", fakeUserID), 200, []byte(`{}`))
	if err := runCmd(t, "me", "benefits"); err != nil {
		t.Fatal(err)
	}
}
