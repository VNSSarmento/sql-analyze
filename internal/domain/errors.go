package domain

import "errors"

var ErrQueryNotFound = errors.New("query not found")

var ErrContactNotFound = errors.New("contact not found")

var ErrCacheMiss = errors.New("cache miss")
