package auth

import "alloy/models/usermanager"

type RegisterResult struct {
	User usermanager.User `json:"user"`
}

type LoginResult struct {
	User usermanager.User `json:"user"`
}

type LoginEvent struct {
	UserID int64  `json:"user_id"`
	Email  string `json:"email"`
	Name   string `json:"name"`
}

type AuthService interface {
	Register(email, password, name string) (RegisterResult, error)
	Login(email, password string) (LoginResult, error)
}
