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

var staticHandler http.Handler

//go:embed static/*
var staticFiles embed.FS

var Version = "0.1.0"

func main() {
	// Configuration from environment
	port := getEnv("PORT", "8080")
	dbPath := getEnv("DB_PATH", "/db/redchef.db")

	// Initialize database
	if err := db.Init(dbPath); err != nil {
		log.Fatalf("Failed to initialize database: %v", err)
	}

	// Set up routes
	mux := http.NewServeMux()

	// Static file server for uploads (served at /uploads/)
	uploadDir := getEnv("UPLOAD_DIR", "/app/media")
	uploadFS := http.FileServer(http.Dir(uploadDir))
	mux.Handle("GET /uploads/", http.StripPrefix("/uploads/", uploadFS))

	// Setup (first-run, no auth)
	mux.HandleFunc("POST /api/setup", handlers.Setup)
	mux.HandleFunc("GET /api/setup/status", handlers.SetupStatus)

	// Public API
	mux.HandleFunc("GET /api/posts", handlers.ListPosts)
	mux.HandleFunc("GET /api/posts/{id}", handlers.GetPost)
	mux.HandleFunc("POST /api/subscribe", handlers.Subscribe)

	// Admin API
	mux.HandleFunc("POST /api/admin/login", handlers.Login)
	mux.HandleFunc("POST /api/admin/logout", handlers.Logout)

	// Public settings (no auth)
	mux.HandleFunc("GET /api/settings/analytics", handlers.PublicGetAnalyticsSettings)

	// Admin API (authenticated)
	mux.Handle("GET /api/admin/posts", handlers.AdminAuth(http.HandlerFunc(handlers.AdminListPosts)))
	mux.Handle("POST /api/admin/upload", handlers.AdminAuth(http.HandlerFunc(handlers.AdminUpload)))
	mux.Handle("DELETE /api/admin/posts/{id}", handlers.AdminAuth(http.HandlerFunc(handlers.AdminDeletePost)))
	mux.Handle("PATCH /api/admin/posts/{id}", handlers.AdminAuth(http.HandlerFunc(handlers.AdminUpdatePost)))
	mux.Handle("GET /api/admin/settings/analytics", handlers.AdminAuth(http.HandlerFunc(handlers.AdminGetAnalyticsSettings)))
	mux.Handle("PUT /api/admin/settings/analytics", handlers.AdminAuth(http.HandlerFunc(handlers.AdminUpdateAnalyticsSettings)))

	// SPA fallback: serve embedded static files for all non-API routes
	staticSub, err := fs.Sub(staticFiles, "static")
	if err != nil {
		log.Fatalf("Failed to create static sub filesystem: %v", err)
	}
	staticHandler = http.FileServer(http.FS(staticSub))

	// Setup page: redirect to admin if admin exists, serve setup.html if not
	mux.HandleFunc("GET /setup", func(w http.ResponseWriter, r *http.Request) {
		hasUsers, err := db.HasUsers()
		if err == nil && hasUsers {
			http.Redirect(w, r, "/admin.html", http.StatusFound)
			return
		}
		// serve setup.html from embedded files
		r.URL.Path = "/setup.html"
		staticHandler.ServeHTTP(w, r)
	})

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
		w.Header().Set("Access-Control-Allow-Methods", "GET, POST, PATCH, DELETE, OPTIONS")
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
