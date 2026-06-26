package testharness

import (
	"context"
	"testing"

	"alloy/internal/modules/usermanager/migrations"
	"alloy/internal/modules/usermanager/service"
	"alloy/internal/modules/usermanager/store"
	"github.com/NepMods/ember"
	_ "github.com/mattn/go-sqlite3"
)

type Harness struct {
	Service *service.Service
	DB      *ember.DB
	Close   func()
}

func New(t testing.TB) *Harness {
	t.Helper()

	db, err := ember.Open(ember.Config{
		Driver: "sqlite3",
		Master: ":memory:",
	})
	if err != nil {
		t.Fatal(err)
	}

	migrator := ember.NewMigrator(db, migrations.EmberMigrations()...)
	if err := migrator.Run(context.Background()); err != nil {
		db.Close()
		t.Fatal(err)
	}

	st := store.New(db, context.Background())
	svc := service.New()
	svc.Store = st

	return &Harness{
		Service: svc,
		DB:      db,
		Close:   func() { db.Close() },
	}
}
