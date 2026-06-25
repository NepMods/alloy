// Package core's migrations. Core owns only its module-specific tables. The base
// identity tables (users, memberships, tenants) live in the kernel because
// auth/tenancy is a platform invariant. Core may add extension tables that hang
// off the kernel-owned users table — but never FK into other modules' tables.
package migrations

import (
	"alloy/models/contract"
	"context"
)

// Specs returns core's migrations in run order.
func Specs() []contract.MigrationSpec {
	return []contract.MigrationSpec{
		createUserProfiles(),
	}
}

// createUserProfiles adds the module-specific profile table. The kernel's users
// table holds email/password/name (auth essentials); this holds the richer
// profile data that only core manages (avatar, locale, phone, preferences).
func createUserProfiles() contract.MigrationSpec {
	return contract.MigrationSpec{
		Version:     "0001_create_user_profiles",
		Description: "core: user profile extension table",
		Up: func(_ context.Context, s contract.MigrationSchema) error {
			return s.Create("core_user_profiles", func(b contract.Blueprint) {
				b.UnsignedBigInteger("user_id")
				b.String("phone").Nullable()
				b.String("locale", 16).Default("en")
				b.String("avatar_url").Nullable()
				b.JSONB("preferences").Nullable()
				b.Timestamps()
				// PK + uniqueness. user_id is both PK and the join key to kernel.users.
				b.Primary("user_id")
				b.Index("locale")
			})
		},
		Down: func(_ context.Context, s contract.MigrationSchema) error {
			return s.DropIfExists("core_user_profiles")
		},
	}
}
