package main

import (
	"context"
	"log"
	"net/http"
	"os"
	"time"

	"gisonline-uat/internal/config"
	"gisonline-uat/internal/uat"
)

func main() {
	if err := config.LoadDotEnv(".env"); err != nil {
		log.Fatalf("load .env: %v", err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	databaseURL := getenv("DATABASE_URL", "postgres://uat_user:uat_password@localhost:5432/gisonline_uat?sslmode=disable")
	store, err := uat.NewPostgresStore(ctx, databaseURL)
	if err != nil {
		log.Fatalf("connect database: %v", err)
	}
	defer store.Close()

	addr := getenv("APP_ADDR", ":8080")
	staticDir := getenv("STATIC_DIR", "web")

	log.Printf("GIS Online UAT listening on http://localhost%s", addr)
	if err := http.ListenAndServe(addr, uat.NewHandler(store, os.DirFS(staticDir))); err != nil {
		log.Fatal(err)
	}
}

func getenv(key, fallback string) string {
	value := os.Getenv(key)
	if value == "" {
		return fallback
	}
	return value
}
