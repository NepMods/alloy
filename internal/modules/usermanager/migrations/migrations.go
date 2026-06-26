package migrations

import (
	"context"

	"alloy/models/contract"
	"github.com/NepMods/ember"
)

func Specs() []contract.MigrationSpec {
	return []contract.MigrationSpec{
		{
			Version:     "2025_01_01_000001",
			Description: "create usermanager_users table",
			Up:          upCreateUsers,
			Down:        downCreateUsers,
		},
	}
}

func upCreateUsers(ctx context.Context, schema contract.MigrationSchema) error {
	return schema.Create("usermanager_users", func(b contract.Blueprint) {
		b.ID()
		b.String("email").Unique()
		b.String("password_hash")
		b.String("name")
		b.Timestamps()
	})
}

func downCreateUsers(ctx context.Context, schema contract.MigrationSchema) error {
	return schema.Drop("usermanager_users")
}

func EmberMigrations() []ember.Migration {
	return []ember.Migration{
		&createUsersTable{},
	}
}

type createUsersTable struct{}

func (m *createUsersTable) Version() string { return "2025_01_01_000001" }

func (m *createUsersTable) Up(schema *ember.Schema) error {
	return schema.Create("usermanager_users", func(b *ember.Blueprint) {
		b.ID()
		b.String("email").Unique()
		b.String("password_hash")
		b.String("name")
		b.Timestamps()
	})
}

func (m *createUsersTable) Down(schema *ember.Schema) error {
	return schema.Drop("usermanager_users")
}
