package db

import (
	"github.com/Qcsinc23/qcs-cargo/internal/db/gen"
)

var queries *gen.Queries

// Queries returns the sqlc Queries instance. Call after Connect().
func Queries() *gen.Queries {
	if conn == nil {
		panic("db: Connect must be called first")
	}
	if queries == nil {
		queries = gen.New(conn)
	}
	return queries
}
