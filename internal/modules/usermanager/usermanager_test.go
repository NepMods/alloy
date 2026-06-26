package usermanager_test

import (
	"testing"

	"alloy/internal/modules/usermanager/testharness"
)

func TestCreateAndGetByEmail(t *testing.T) {
	h := testharness.New(t)
	user, err := h.Service.Create("alice@test.com", "hash", "Alice")
	if err != nil {
		t.Fatal(err)
	}
	if user.Email != "alice@test.com" {
		t.Fatalf("expected alice@test.com, got %s", user.Email)
	}
	got, err := h.Service.GetByEmail("alice@test.com")
	if err != nil {
		t.Fatal(err)
	}
	if got.ID != user.ID {
		t.Fatalf("expected id %d, got %d", user.ID, got.ID)
	}
}

func TestCreateDuplicateEmail(t *testing.T) {
	h := testharness.New(t)
	_, err := h.Service.Create("dup@test.com", "hash1", "First")
	if err != nil {
		t.Fatal(err)
	}
	_, err = h.Service.Create("dup@test.com", "hash2", "Second")
	if err == nil {
		t.Fatal("expected error for duplicate email")
	}
}

func TestGetByID(t *testing.T) {
	h := testharness.New(t)
	user, err := h.Service.Create("bob@test.com", "hash", "Bob")
	if err != nil {
		t.Fatal(err)
	}
	got, err := h.Service.GetByID(user.ID)
	if err != nil {
		t.Fatal(err)
	}
	if got.Name != "Bob" {
		t.Fatalf("expected Bob, got %s", got.Name)
	}
}
