package usermanager

import "time"

type User struct {
	ID           int64     `json:"id"`
	Email        string    `json:"email"`
	PasswordHash string    `json:"password_hash"`
	Name         string    `json:"name"`
	CreatedAt    time.Time `json:"created_at"`
	UpdatedAt    time.Time `json:"updated_at"`
}

type UserManager interface {
	Create(email, passwordHash, name string) (User, error)
	GetByEmail(email string) (User, error)
	GetByID(id int64) (User, error)
}
