package fakes

import (
	"fmt"
	"sync"
	"time"

	"alloy/models/usermanager"
)

var _ usermanager.UserManager = (*FakeUserManager)(nil)

type FakeUserManager struct {
	mu    sync.Mutex
	users []usermanager.User
	seq   int64
}

func NewFakeUserManager() *FakeUserManager {
	return &FakeUserManager{}
}

func (f *FakeUserManager) Create(email, passwordHash, name string) (usermanager.User, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	for _, u := range f.users {
		if u.Email == email {
			return usermanager.User{}, fmt.Errorf("email already exists")
		}
	}
	f.seq++
	now := time.Now()
	user := usermanager.User{
		ID:           f.seq,
		Email:        email,
		PasswordHash: passwordHash,
		Name:         name,
		CreatedAt:    now,
		UpdatedAt:    now,
	}
	f.users = append(f.users, user)
	return user, nil
}

func (f *FakeUserManager) GetByEmail(email string) (usermanager.User, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	for _, u := range f.users {
		if u.Email == email {
			return u, nil
		}
	}
	return usermanager.User{}, fmt.Errorf("user not found")
}

func (f *FakeUserManager) GetByID(id int64) (usermanager.User, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	for _, u := range f.users {
		if u.ID == id {
			return u, nil
		}
	}
	return usermanager.User{}, fmt.Errorf("user not found")
}
