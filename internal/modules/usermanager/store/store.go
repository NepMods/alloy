package store

import (
	"context"
	"fmt"

	"alloy/internal/modules/usermanager/models"
	"alloy/models/usermanager"
	"github.com/NepMods/ember"
)

type Store struct {
	db  *ember.DB
	ctx context.Context
}

func New(db *ember.DB, ctx context.Context) *Store {
	return &Store{db: db, ctx: ctx}
}

func (s *Store) Create(email, passwordHash, name string) (usermanager.User, error) {
	row := models.User{
		Email:        email,
		PasswordHash: passwordHash,
		Name:         name,
	}
	if err := s.db.Model().Create(s.ctx, &row); err != nil {
		return usermanager.User{}, err
	}
	return toUser(row), nil
}

func (s *Store) GetByEmail(email string) (usermanager.User, error) {
	var row models.User
	err := s.db.Model().First(s.ctx, &row, func(b *ember.Builder) {
		b.Where("email", "=", email)
	})
	if err != nil {
		return usermanager.User{}, fmt.Errorf("user not found")
	}
	return toUser(row), nil
}

func (s *Store) GetByID(id int64) (usermanager.User, error) {
	var row models.User
	if err := s.db.Model().Find(s.ctx, &row, id); err != nil {
		return usermanager.User{}, fmt.Errorf("user not found")
	}
	return toUser(row), nil
}

func toUser(row models.User) usermanager.User {
	return usermanager.User{
		ID:           row.ID,
		Email:        row.Email,
		PasswordHash: row.PasswordHash,
		Name:         row.Name,
		CreatedAt:    row.CreatedAt,
		UpdatedAt:    row.UpdatedAt,
	}
}
