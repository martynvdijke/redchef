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

var Version = "1.17.0"

func main() {
	port := getEnv("PORT", "8080")
	dbPath := getEnv("DB_PATH", "/db/redchef.db")

	if err := db.Init(dbPath); err != nil {
		log.Fatalf("Failed to initialize database: %v", err)
	}

	mux := http.NewServeMux()

	// Static file server for uploads
	uploadDir := getEnv("UPLOAD_DIR", "/app/media")
	uploadFS := http.FileServer(http.Dir(uploadDir))
	mux.Handle("GET /uploads/", http.StripPrefix("/uploads/", uploadFS))

	// Setup (first-run, no auth)
	mux.HandleFunc("POST /api/setup", handlers.Setup)
	mux.HandleFunc("GET /api/setup/status", handlers.SetupStatus)

	// Auth API (public)
	mux.HandleFunc("POST /api/auth/register", handlers.Register)
	mux.HandleFunc("POST /api/auth/login", handlers.Login)
	mux.HandleFunc("POST /api/auth/logout", handlers.Logout)
	mux.HandleFunc("POST /api/auth/forgot", handlers.ForgotPassword)
	mux.HandleFunc("POST /api/auth/reset", handlers.ResetPassword)
	mux.Handle("GET /api/auth/me", handlers.AuthMiddleware(http.HandlerFunc(handlers.Me)))

	// Paywall API (mutations require session + bearer API token)
	payMux := http.NewServeMux()
	payMux.HandleFunc("POST /api/pay/unlock", handlers.PayUnlock)
	payMux.HandleFunc("POST /api/pay/item", handlers.PayItem)
	mux.Handle("POST /api/pay/", handlers.AuthMiddleware(handlers.RequireAuth(handlers.RequireMutationToken(payMux))))

	// DAS / RSS feed (public, no auth required) — title, message and image per post
	mux.HandleFunc("GET /feed.xml", handlers.Feed)
	mux.HandleFunc("GET /rss.xml", handlers.Feed)
	mux.HandleFunc("GET /api/feed", handlers.Feed)
	mux.HandleFunc("GET /feed", handlers.Feed)

	// Public API (auth-aware via middleware)
	publicMux := http.NewServeMux()
	publicMux.HandleFunc("GET /api/posts", handlers.ListPosts)
	publicMux.HandleFunc("GET /api/posts/{id}", handlers.GetPost)
	publicMux.HandleFunc("GET /api/posts/{id}/comments", handlers.ListComments)
	publicMux.HandleFunc("GET /api/trmnl/latest", handlers.TRMNLLatestPost)
	publicMux.HandleFunc("GET /api/favourites", handlers.ListFavourites)
	publicMux.HandleFunc("GET /api/settings/analytics", handlers.PublicGetAnalyticsSettings)
	mux.Handle("GET /api/", handlers.AuthMiddleware(publicMux))

	// Authenticated actions (require session + bearer API token)
	authMux := http.NewServeMux()
	authMux.HandleFunc("POST /api/posts/{id}/comments", handlers.CreateComment)
	authMux.HandleFunc("POST /api/posts/{id}/favourite", handlers.ToggleFavourite)
	authMux.HandleFunc("POST /api/posts/{id}/tip", handlers.CreateTip)
	mux.Handle("POST /api/posts/", handlers.AuthMiddleware(handlers.RequireAuth(handlers.RequireMutationToken(authMux))))

	// API token management (session only — creating a token must not require one)
	tokenGuard := func(h http.HandlerFunc) http.Handler {
		return handlers.AuthMiddleware(handlers.RequireAuth(h))
	}
	mux.Handle("GET /api/tokens", tokenGuard(handlers.ListTokens))
	mux.Handle("POST /api/tokens", tokenGuard(handlers.CreateToken))
	mux.Handle("DELETE /api/tokens/{id}", tokenGuard(handlers.RevokeToken))
	mux.Handle("POST /api/tokens/{id}/rotate", tokenGuard(handlers.RotateToken))

	// Admin API (authenticated as admin)
	adminMux := http.NewServeMux()
	adminMux.HandleFunc("GET /api/admin/posts", handlers.AdminListPosts)
	adminMux.HandleFunc("POST /api/admin/upload", handlers.AdminUpload)
	adminMux.HandleFunc("DELETE /api/admin/posts/{id}", handlers.AdminDeletePost)
	adminMux.HandleFunc("PATCH /api/admin/posts/{id}", handlers.AdminUpdatePost)
	adminMux.HandleFunc("PUT /api/admin/posts/{id}/media", handlers.AdminReplaceMedia)
	adminMux.HandleFunc("GET /api/admin/users", handlers.AdminListUsers)
	adminMux.HandleFunc("PATCH /api/admin/users/{id}", handlers.AdminUpdateUser)
	adminMux.HandleFunc("DELETE /api/admin/users/{id}", handlers.AdminDeleteUser)
	adminMux.HandleFunc("GET /api/admin/posts/simple", handlers.AdminListAllPostsSimple)
	adminMux.HandleFunc("GET /api/admin/settings/profile", handlers.AdminGetProfile)
	adminMux.HandleFunc("PUT /api/admin/settings/profile", handlers.AdminUpdateProfile)
	adminMux.HandleFunc("GET /api/admin/settings/analytics", handlers.AdminGetAnalyticsSettings)
	adminMux.HandleFunc("PUT /api/admin/settings/analytics", handlers.AdminUpdateAnalyticsSettings)
	adminMux.HandleFunc("GET /api/admin/settings/email", handlers.AdminGetEmailSettings)
	adminMux.HandleFunc("PUT /api/admin/settings/email", handlers.AdminUpdateEmailSettings)
	adminMux.HandleFunc("GET /api/admin/comments", handlers.AdminListComments)
	adminMux.HandleFunc("DELETE /api/admin/comments/{id}", handlers.AdminDeleteComment)
	adminMux.HandleFunc("PUT /api/admin/posts/{id}/links", handlers.AdminSetPostLinks)
	adminMux.HandleFunc("GET /api/admin/posts/{id}/links", handlers.AdminGetPostLinks)
	mux.Handle("GET /api/admin/", handlers.AdminAuth(adminMux))
	mux.Handle("POST /api/admin/", handlers.AdminAuth(adminMux))
	mux.Handle("DELETE /api/admin/", handlers.AdminAuth(adminMux))
	mux.Handle("PATCH /api/admin/", handlers.AdminAuth(adminMux))
	mux.Handle("PUT /api/admin/", handlers.AdminAuth(adminMux))

	// Admin test email (POST with auth)
	mux.Handle("POST /api/admin/email/test", handlers.AdminAuth(http.HandlerFunc(handlers.AdminTestEmail)))

	// Old admin login/logout (redirect to new auth)
	mux.HandleFunc("POST /api/admin/login", handlers.Login)
	mux.HandleFunc("POST /api/admin/logout", handlers.Logout)

	// SPA fallback
	staticSub, err := fs.Sub(staticFiles, "static")
	if err != nil {
		log.Fatalf("Failed to create static sub filesystem: %v", err)
	}
	staticHandler = http.FileServer(http.FS(staticSub))

	mux.HandleFunc("GET /setup", func(w http.ResponseWriter, r *http.Request) {
		hasUsers, err := db.HasUsers()
		if err == nil && hasUsers {
			http.Redirect(w, r, "/admin.html", http.StatusFound)
			return
		}
		r.URL.Path = "/setup.html"
		staticHandler.ServeHTTP(w, r)
	})

	// Shareable post page — serves the SPA, which renders the single post.
	// Note: path is rewritten to "/" (not "/index.html") because FileServer
	// 301-redirects /index.html to / and the post id would be lost.
	mux.HandleFunc("GET /posts/{id}", func(w http.ResponseWriter, r *http.Request) {
		r.URL.Path = "/"
		staticHandler.ServeHTTP(w, r)
	})

	// Password reset page — serves the SPA, which renders the reset modal
	// from the ?token= query parameter.
	mux.HandleFunc("GET /reset", func(w http.ResponseWriter, r *http.Request) {
		r.URL.Path = "/"
		staticHandler.ServeHTTP(w, r)
	})

	mux.Handle("GET /", staticHandler)

	handler := withMiddleware(mux)

	log.Printf("RedChef starting on :%s", port)
	if err := http.ListenAndServe(":"+port, handler); err != nil {
		log.Fatalf("Server failed: %v", err)
	}
}

func withMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.Header().Set("Access-Control-Allow-Methods", "GET, POST, PATCH, DELETE, OPTIONS")
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization")

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
