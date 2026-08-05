package secondarydb

import (
	"database/sql"
	"fmt"
	"os"

	_ "github.com/lib/pq"
)

// DB is the connection pool for the secondary Postgres database, configured
// via the DB2_* env vars. It holds the individuals/professions data shared
// with account-service-app, and is separate from GoFr's own c.SQL pool
// (which owns this app's own files table).
var DB *sql.DB

func Init() error {
	dsn := fmt.Sprintf("host=%s port=%s user=%s password=%s dbname=%s sslmode=%s",
		os.Getenv("DB2_HOST"), os.Getenv("DB2_PORT"), os.Getenv("DB2_USER"),
		os.Getenv("DB2_PASSWORD"), os.Getenv("DB2_NAME"), os.Getenv("DB2_SSL_MODE"))

	db, err := sql.Open("postgres", dsn)
	if err != nil {
		return err
	}

	if err := db.Ping(); err != nil {
		db.Close()
		return err
	}

	DB = db

	return nil
}
