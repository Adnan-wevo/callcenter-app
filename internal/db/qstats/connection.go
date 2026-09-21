package qstats

import (
	"fmt"

	"github.com/jmoiron/sqlx"

	_ "github.com/go-sql-driver/mysql"
)

// Connect opens a connection pool to the qstats MySQL database.
// dsn is expected in go-sql-driver/mysql DSN form, e.g.:
//
//	user:pass@tcp(host:3306)/qstats?parseTime=true
func Connect(dsn string) (*sqlx.DB, error) {
	if dsn == "" {
		return nil, fmt.Errorf("qstats: QSTATS_DSN is empty")
	}
	db, err := sqlx.Connect("mysql", dsn)
	if err != nil {
		return nil, fmt.Errorf("qstats: connect: %w", err)
	}
	return db, nil
}
