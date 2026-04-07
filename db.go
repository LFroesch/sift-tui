package main

import (
	"database/sql"

	_ "github.com/lib/pq"
)

type DB struct{ *sql.DB }

func NewDB(connStr string) (*DB, error) {
	sqlDB, err := sql.Open("postgres", connStr)
	if err != nil {
		return nil, err
	}
	if err := sqlDB.Ping(); err != nil {
		return nil, err
	}
	return &DB{sqlDB}, nil
}
