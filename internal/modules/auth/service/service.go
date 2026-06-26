package service

import (
	"context"
	"crypto/sha256"
	"crypto/subtle"
	"fmt"

	"alloy/internal/platform/messaging"
	"alloy/models/auth"
)

type Service struct {
	BaseService
	Bus messaging.Bus
}

func (s *Service) Register(email, password, name string) (auth.RegisterResult, error) {
	hash := hashPassword(password)
	user, err := s.BaseService.UserManager.Create(email, hash, name)
	if err != nil {
		return auth.RegisterResult{}, err
	}
	return auth.RegisterResult{User: user}, nil
}

func (s *Service) Login(email, password string) (auth.LoginResult, error) {
	user, err := s.BaseService.UserManager.GetByEmail(email)
	if err != nil {
		return auth.LoginResult{}, fmt.Errorf("invalid credentials")
	}
	// s.Runtime.Bus().Publish(context.Background(), messaging.Message{
	// 	Topic:   "log.info",
	// 	Payload: user.PasswordHash,
	// })
	if !verifyPassword(password, user.PasswordHash) {
		return auth.LoginResult{}, fmt.Errorf("invalid credentials")
	}
	if s.Bus != nil {
		evt := auth.LoginEvent{UserID: user.ID, Email: user.Email, Name: user.Name}
		_ = s.Bus.Publish(context.Background(), messaging.Message{
			Topic:   "auth.user.logged_in",
			Payload: evt,
		})
	}
	return auth.LoginResult{User: user}, nil
}

const passwordSalt = "alloy-auth-v1"

func hashPassword(password string) string {
	h := sha256.Sum256([]byte(password + passwordSalt))
	return fmt.Sprintf("%x", h)
}

func verifyPassword(password, hash string) bool {
	expected := hashPassword(password)
	return subtle.ConstantTimeCompare([]byte(expected), []byte(hash)) == 1
}
