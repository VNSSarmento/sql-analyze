package domain

type QueryRepository interface {
	GetByID(queryID, dbUser string) (*Query, error)
	Save(*Query) error
	GetQueryLimit(limit int) []*Query
}
