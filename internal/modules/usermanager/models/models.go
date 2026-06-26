package models

import "time"

type User struct {
	ID           int64     `ember:"primaryKey;autoIncr"`
	Email        string    `ember:"unique"`
	PasswordHash string    `ember:"column:password_hash"`
	Name         string
	CreatedAt    time.Time
	UpdatedAt    time.Time
}

func (User) TableName() string { return "usermanager_users" }
