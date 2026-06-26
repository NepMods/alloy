package fakes

import (
	"fmt"
	"time"

	"alloy/models/auth"
	"alloy/models/usermanager"
)

var _ auth.AuthService = (*FakeAuthService)(nil)

type FakeAuthService struct {
	Users map[string]auth.RegisterResult
	seq   int64
}

func NewFakeAuthService() *FakeAuthService {
	return &FakeAuthService{
		Users: make(map[string]auth.RegisterResult),
	}
}

func (f *FakeAuthService) Register(email, password, name string) (auth.RegisterResult, error) {
	if _, ok := f.Users[email]; ok {
		return auth.RegisterResult{}, fmt.Errorf("email already exists")
	}
	f.seq++
	now := time.Now()
	result := auth.RegisterResult{
		User: usermanager.User{
			ID:           f.seq,
			Email:        email,
			PasswordHash: password,
			Name:         name,
			CreatedAt:    now,
			UpdatedAt:    now,
		},
	}
	f.Users[email] = result
	return result, nil
}

func (f *FakeAuthService) Login(email, password string) (auth.LoginResult, error) {
	reg, ok := f.Users[email]
	if !ok {
		return auth.LoginResult{}, fmt.Errorf("invalid credentials")
	}
	return auth.LoginResult{User: reg.User}, nil
}
