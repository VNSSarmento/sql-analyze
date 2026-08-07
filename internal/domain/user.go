package domain

import "context"

type UserContact struct {
	DBUser string
	Email  string
}

type UserContactRepository interface {
	GetContactByDBUser(ctx context.Context, dbUser string) (*UserContact, error)
}
