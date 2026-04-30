package utils

import (
	"errors"

	"github.com/jackc/pgx/v5/pgconn"
)

const pgUniqueViolationCode = "23505"

// IsUniqueViolation returns true when the underlying DB error is a Postgres unique violation.
func IsUniqueViolation(err error) bool {
	if err == nil {
		return false
	}

	var pgErr *pgconn.PgError
	if errors.As(err, &pgErr) {
		return pgErr.Code == pgUniqueViolationCode
	}

	return false
}
