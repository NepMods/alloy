package boot

import (
	"alloy/internal/app/contract"
	"alloy/models/app"
	"context"
)

func Modules() []contract.Module {
	return []contract.Module{}
}

func Build(ctx context.Context) (*app.App, error) {
	return &app.App{}, nil
}
