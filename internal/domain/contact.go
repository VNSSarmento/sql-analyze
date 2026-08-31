package domain

import (
	"context"
)

type UserContact struct {
	DBUser      string
	Email       string
	DisplayName string
}

type ContactRepository interface {
	GetByDBUser(ctx context.Context, dbUser string) (*UserContact, error)
}
