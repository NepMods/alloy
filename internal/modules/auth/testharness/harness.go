package testharness

import (
	"testing"

	"alloy/internal/modules/auth/service"
	usermanagerfakes "alloy/internal/modules/usermanager/fakes"
)

type Harness struct {
	Service   *service.Service
	FakeUsers *usermanagerfakes.FakeUserManager
}

func New(t testing.TB) *Harness {
	t.Helper()
	fakeUsers := usermanagerfakes.NewFakeUserManager()
	svc := service.New()
	svc.BaseService = service.NewBaseService(nil, fakeUsers)
	return &Harness{
		Service:   svc,
		FakeUsers: fakeUsers,
	}
}
