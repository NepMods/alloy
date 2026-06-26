package auth_test

import (
	"testing"

	"alloy/internal/modules/auth/testharness"
)

func TestRegisterAndLogin(t *testing.T) {
	h := testharness.New(t)

	_, err := h.Service.Register("alice@test.com", "secret123", "Alice")
	if err != nil {
		t.Fatal(err)
	}

	result, err := h.Service.Login("alice@test.com", "secret123")
	if err != nil {
		t.Fatal(err)
	}
	if result.User.Email != "alice@test.com" {
		t.Fatalf("expected alice@test.com, got %s", result.User.Email)
	}
}

func TestRegisterDuplicate(t *testing.T) {
	h := testharness.New(t)

	_, err := h.Service.Register("dup@test.com", "pass1", "First")
	if err != nil {
		t.Fatal(err)
	}
	_, err = h.Service.Register("dup@test.com", "pass2", "Second")
	if err == nil {
		t.Fatal("expected error for duplicate email")
	}
}

func TestLoginWrongPassword(t *testing.T) {
	h := testharness.New(t)

	_, err := h.Service.Register("bob@test.com", "correct", "Bob")
	if err != nil {
		t.Fatal(err)
	}

	_, err = h.Service.Login("bob@test.com", "wrong")
	if err == nil {
		t.Fatal("expected error for wrong password")
	}
}

func TestLoginUnknownUser(t *testing.T) {
	h := testharness.New(t)

	_, err := h.Service.Login("nobody@test.com", "whatever")
	if err == nil {
		t.Fatal("expected error for unknown user")
	}
}
