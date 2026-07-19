package main

import (
	"embed"
	"io/fs"
	"log"
	"net/http"
	"os"

	"redchef/db"
	"redchef/handlers"
)

//go:embed static/*
var staticFiles embed.FS

var Version = "0.1.0"

func main() {
	// Configuration from environment
	port := getEnv("PORT", "8080")
	dbPath := getEnv("DB_PATH", "redchef.db")
	adminUser := getEnv("ADMIN_USERNAME", "admin")
	adminPass := getEnv("ADMIN_PASSWORD", "admin")

	// Initialize database
	if err := db.Init(dbPath); err != nil {
		log.Fatalf("Failed to initialize database: %v", err)
	}

	// Seed default admin user
	if err := db.SeedAdmin(adminUser, adminPass); err != nil {
		log.Fatalf("Failed to seed admin user: %v", err)
	}

	// Set up routes
	mux := http.NewServeMux()

	// Static file server for uploads (served at /uploads/)
	uploadFS := http.FileServer(http.Dir("uploads"))
	mux.Handle("GET /uploads/", http.StripPrefix("/uploads/", uploadFS))

	// Public API
	mux.HandleFunc("GET /api/posts", handlers.ListPosts)
	mux.HandleFunc("GET /api/posts/{id}", handlers.GetPost)
	mux.HandleFunc("POST /api/unlock", handlers.Unlock)

	// Admin API
	mux.HandleFunc("POST /api/admin/login", handlers.Login)
	mux.HandleFunc("POST /api/admin/logout", handlers.Logout)

	// Admin API (authenticated)
	mux.Handle("GET /api/admin/posts", handlers.AdminAuth(http.HandlerFunc(handlers.AdminListPosts)))
	mux.Handle("POST /api/admin/upload", handlers.AdminAuth(http.HandlerFunc(handlers.AdminUpload)))
	mux.Handle("DELETE /api/admin/posts/{id}", handlers.AdminAuth(http.HandlerFunc(handlers.AdminDeletePost)))

	// SPA fallback: serve embedded static files for all non-API routes
	staticSub, err := fs.Sub(staticFiles, "static")
	if err != nil {
		log.Fatalf("Failed to create static sub filesystem: %v", err)
	}
	staticHandler := http.FileServer(http.FS(staticSub))

	mux.Handle("GET /", staticHandler)

	// Wrap with CORS and logging middleware
	handler := withMiddleware(mux)

	log.Printf("RedChef starting on :%s", port)
	if err := http.ListenAndServe(":"+port, handler); err != nil {
		log.Fatalf("Server failed: %v", err)
	}
}

func withMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// CORS headers for development
		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.Header().Set("Access-Control-Allow-Methods", "GET, POST, DELETE, OPTIONS")
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type")

		if r.Method == "OPTIONS" {
			w.WriteHeader(http.StatusOK)
			return
		}

		log.Printf("%s %s", r.Method, r.URL.Path)
		next.ServeHTTP(w, r)
	})
}

func getEnv(key, fallback string) string {
	if val := os.Getenv(key); val != "" {
		return val
	}
	return fallback
}
