package db

import (
	"database/sql"
	"fmt"
	"log"
	"os"

	_ "github.com/lib/pq"
)

var DB *sql.DB

func Init() {
	var err error

	connStr := os.Getenv("DATABASE_URL")
	if connStr == "" {
		dbHost := os.Getenv("DB_HOST")
		dbPort := os.Getenv("DB_PORT")
		dbUser := os.Getenv("DB_USER")
		dbPass := os.Getenv("DB_PASSWORD")
		dbName := os.Getenv("DB_NAME")

		if dbHost == "" { dbHost = "localhost" }
		if dbPort == "" { dbPort = "5432" }
		if dbUser == "" { dbUser = "postgres" }
		if dbPass == "" { dbPass = "" }
		if dbName == "" { dbName = "postgres" }

		connStr = fmt.Sprintf("host=%s port=%s user=%s password=%s dbname=%s sslmode=disable", 
			dbHost, dbPort, dbUser, dbPass, dbName)
	}

	DB, err = sql.Open("postgres", connStr)
	if err != nil {
		log.Fatal(err)
	}

	if err = DB.Ping(); err != nil {
		log.Fatal("Cannot connect to database:", err)
	}

	fmt.Println("Successfully connected to database")

	migrate()
}

func migrate() {
	query := `
	CREATE TABLE IF NOT EXISTS reviews (
		id SERIAL PRIMARY KEY,
		description TEXT NOT NULL,
		stars INTEGER CHECK (stars >= 1 AND stars <= 5)
	);`
	if _, err := DB.Exec(query); err != nil {
		log.Fatal("Failed to run migrations:", err)
	}
}
