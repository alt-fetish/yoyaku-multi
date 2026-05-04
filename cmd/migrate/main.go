package main

import (
	"context"
	"fmt"
	"log"
	"os"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/joho/godotenv"
)

func main() {
	_ = godotenv.Load()

	pool, err := pgxpool.New(context.Background(), os.Getenv("DATABASE_URL"))
	if err != nil {
		log.Fatal(err)
	}
	defer pool.Close()

	files := []string{
		"migrations/001_init.sql",
		"migrations/002_chat.sql",
		"migrations/003_sessions.sql",
		"migrations/004_event_capacity.sql",
		"migrations/005_option_sets.sql",
	}

	for _, f := range files {
		sql, err := os.ReadFile(f)
		if err != nil {
			log.Fatal(err)
		}
		if _, err := pool.Exec(context.Background(), string(sql)); err != nil {
			log.Fatalf("%s: %v", f, err)
		}
		fmt.Println("✓", f)
	}
	fmt.Println("マイグレーション完了")
}
