package calllog

import (
	"fmt"

	"github.com/jmoiron/sqlx"

	_ "github.com/go-sql-driver/mysql"
)

// Connect opens a connection pool to this product's own database — NOT
// qstats (externally owned by the PBX) and not heal-crm's database (this
// product has none). dsn is a go-sql-driver/mysql DSN, e.g.:
//
//	user:pass@tcp(host:3306)/callcenter?parseTime=true
func Connect(dsn string) (*sqlx.DB, error) {
	if dsn == "" {
		return nil, fmt.Errorf("calllog: CALLCENTER_DSN is empty")
	}
	db, err := sqlx.Connect("mysql", dsn)
	if err != nil {
		return nil, fmt.Errorf("calllog: connect: %w", err)
	}
	return db, nil
}
