package db

import (
	"context"
	"fmt"
	"time"

	"github.com/jmoiron/sqlx"
	_ "github.com/lib/pq"
)

func Connect(dsn string) (*sqlx.DB, error) {
	db, err := sqlx.Open("postgres", dsn)
	if err != nil {
		return nil, fmt.Errorf("failed to open database: %w", err)
	}

	db.SetMaxOpenConns(10)
	db.SetMaxIdleConns(5)
	db.SetConnMaxLifetime(30 * time.Minute)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := db.PingContext(ctx); err != nil {
		return nil, fmt.Errorf("failed to ping database: %w", err)
	}

	if err := runMigrations(db); err != nil {
		return nil, err
	}

	return db, nil
}

func runMigrations(db *sqlx.DB) error {
	queries := []string{
		`CREATE TABLE IF NOT EXISTS friend_requests (
			id SERIAL PRIMARY KEY,
			from_user_id INT NOT NULL,
			to_user_id INT NOT NULL,
			status TEXT NOT NULL CHECK (status IN ('pending','accepted','rejected')),
			created_at TIMESTAMPTZ DEFAULT NOW()
			)`,
		`CREATE TABLE IF NOT EXISTS friendships (
			id SERIAL PRIMARY KEY,
			user_id INT NOT NULL,
			friend_id INT NOT NULL,
			UNIQUE (user_id, friend_id)
			)`,
		`ALTER TABLE IF EXISTS users ADD COLUMN IF NOT EXISTS avatar_url TEXT`,
	}

	for _, q := range queries {
		if _, err := db.Exec(q); err != nil {
			return fmt.Errorf("failed to run migration: %w", err)
		}
	}

	return nil
}
