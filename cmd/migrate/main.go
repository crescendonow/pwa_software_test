package main

import (
	"context"
	"log"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"gisonline-uat/internal/config"

	"github.com/jackc/pgx/v5/pgxpool"
)

func main() {
	if err := config.LoadDotEnv(".env"); err != nil {
		log.Fatalf("load .env: %v", err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	databaseURL := getenv("DATABASE_URL", "postgres://uat_user:uat_password@localhost:5432/gisonline_uat?sslmode=disable")
	migrationsDir := getenv("MIGRATIONS_DIR", "db")

	pool, err := pgxpool.New(ctx, databaseURL)
	if err != nil {
		log.Fatalf("connect database: %v", err)
	}
	defer pool.Close()

	conn, err := pool.Acquire(ctx)
	if err != nil {
		log.Fatalf("acquire connection: %v", err)
	}
	defer conn.Release()

	files, err := filepath.Glob(filepath.Join(migrationsDir, "*.sql"))
	if err != nil {
		log.Fatalf("read migrations: %v", err)
	}
	sort.Strings(files)
	if len(files) == 0 {
		log.Fatalf("no SQL files found in %s", migrationsDir)
	}

	for _, file := range files {
		contents, err := os.ReadFile(file)
		if err != nil {
			log.Fatalf("read %s: %v", file, err)
		}
		if strings.TrimSpace(string(contents)) == "" {
			continue
		}

		if _, err := conn.Conn().PgConn().Exec(ctx, string(contents)).ReadAll(); err != nil {
			log.Fatalf("apply %s: %v", file, err)
		}
		log.Printf("applied %s", file)
	}
}

func getenv(key, fallback string) string {
	value := os.Getenv(key)
	if value == "" {
		return fallback
	}
	return value
}
