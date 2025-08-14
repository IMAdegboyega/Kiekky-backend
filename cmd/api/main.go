// cmd/api/main.go
// Main entry point for the application with debug logging
// This file bootstraps all components and starts the server

package main

import (
    "context"
    "database/sql"
    "encoding/json"
    "fmt"
    "log"
    "net/http"
    "os"
    "os/signal"
    "syscall"
    "time"
    
    "github.com/gorilla/mux"
    "github.com/jmoiron/sqlx"
    "github.com/joho/godotenv"
    "github.com/go-redis/redis/v8"
    
    // Internal packages
    "github.com/imadgeboyega/kiekky-backend/internal/auth"
    "github.com/imadgeboyega/kiekky-backend/internal/otp"
    "github.com/imadgeboyega/kiekky-backend/internal/profile"
    "github.com/imadgeboyega/kiekky-backend/internal/stories"
    "github.com/imadgeboyega/kiekky-backend/internal/posts" 
    "github.com/imadgeboyega/kiekky-backend/internal/common/database"
    "github.com/imadgeboyega/kiekky-backend/internal/config"
)

func main() {
    // Enable detailed logging
    log.SetFlags(log.Ldate | log.Ltime | log.Lshortfile)
    
    log.Println("========================================")
    log.Println("🚀 Starting Kiekky Social Dating App API")
    log.Println("========================================")
    
    // 1. Load environment variables
    log.Println("📁 Step 1: Loading .env file...")
    if err := godotenv.Load(); err != nil {
        log.Printf("⚠️  Warning: No .env file found (%v), using environment variables", err)
    } else {
        log.Println("✅ .env file loaded successfully")
    }
    
    // 2. Load configuration
    log.Println("\n📋 Step 2: Loading configuration...")
    cfg := config.Load()
    log.Printf("✅ Configuration loaded")
    
    // 3. Validate configuration
    log.Println("\n✔️  Step 3: Validating configuration...")
    if err := cfg.Validate(); err != nil {
        log.Fatal("❌ Configuration validation failed:", err)
    }
    log.Println("✅ Configuration is valid")
    
    // 4. Connect to PostgreSQL
    log.Println("\n🗄️  Step 4: Connecting to PostgreSQL...")
    db, err := database.NewPostgresDBFromURL(cfg.DatabaseURL)
    if err != nil {
        log.Fatal("❌ Failed to connect to PostgreSQL:", err)
    }
    defer db.Close()
    
    if err := db.Ping(); err != nil {
        log.Fatal("❌ Failed to ping PostgreSQL:", err)
    }
    log.Println("✅ Connected to PostgreSQL successfully")
    
    // 5. Connect to Redis (optional)
    log.Println("\n📮 Step 5: Connecting to Redis...")
    var redisClient *redis.Client
    
    if cfg.RedisURL != "" {
        opt, err := redis.ParseURL(cfg.RedisURL)
        if err != nil {
            log.Printf("⚠️  Warning: Invalid Redis URL (%v), continuing without Redis", err)
        } else {
            redisClient = redis.NewClient(opt)
            ctx := context.Background()
            if err := redisClient.Ping(ctx).Err(); err != nil {
                log.Printf("⚠️  Redis ping failed: %v, continuing without Redis", err)
                redisClient = nil
            } else {
                defer redisClient.Close()
                log.Println("✅ Connected to Redis successfully")
            }
        }
    } else {
        log.Println("⚠️  Redis URL not configured, skipping Redis connection")
    }
    
    // 6. Run database migrations
    log.Println("\n🔨 Step 6: Running database migrations...")
    if err := runMigrations(db); err != nil {
        log.Printf("❌ Migration error: %v", err)
        log.Fatal("Failed to run migrations")
    }
    log.Println("✅ Database migrations completed")
    
    // 7. Initialize OTP system (NEW)
    log.Println("\n📱 Step 7: Initializing OTP system...")
    
    // Create OTP repository
    otpRepo := otp.NewPostgresRepository(sqlx.NewDb(db, "postgres"))
    
    // Initialize email provider
    var emailProvider otp.EmailProvider
    switch cfg.EmailProvider {
    case "sendgrid":
        emailProvider = otp.NewSendGridEmailProvider(cfg.SendGridAPIKey, cfg.EmailFrom)
        log.Println("   ✅ Using SendGrid for emails")
    case "smtp":
        emailProvider = otp.NewSMTPEmailProvider(
            cfg.SMTPHost,
            fmt.Sprintf("%d", cfg.SMTPPort),
            cfg.SMTPUsername,
            cfg.SMTPPassword,
            cfg.EmailFrom,
        )
        log.Println("   ✅ Using SMTP for emails")
    default:
        emailProvider = otp.NewMockEmailProvider()
        log.Println("   ⚠️  Using mock email provider (development mode)")
    }
    
    // Initialize SMS provider
    var smsProvider otp.SMSProvider
    switch cfg.SMSProvider {
    case "twilio":
        smsProvider = otp.NewTwilioSMSProvider(
            cfg.TwilioAccountSID,
            cfg.TwilioAuthToken,
            cfg.TwilioPhoneNumber,
        )
        log.Println("   ✅ Using Twilio for SMS")
    default:
        smsProvider = otp.NewMockSMSProvider()
        log.Println("   ⚠️  Using mock SMS provider (development mode)")
    }
    
    // Create OTP configuration
    otpConfig := &otp.OTPConfig{
        Length:      cfg.OTPLength,
        Expiry:      cfg.OTPExpiry,
        MaxAttempts: cfg.MaxOTPAttempts,
        RateLimit: otp.RateLimitConfig{
            MaxRequests: 3,
            Window:      time.Hour,
        },
    }
    
    // Create OTP service
    otpService := otp.NewService(otpRepo, emailProvider, smsProvider, otpConfig)
    log.Println("✅ OTP system initialized")
    
    // Start OTP cleanup job
    go startOTPCleanup(otpService)
    
    // 8. Initialize Profile system (NEW)
    log.Println("\n👤 Step 8: Initializing Profile system...")
    
    // Create profile repository
    profileRepo := profile.NewPostgresRepository(sqlx.NewDb(db, "postgres"))
    
    // Initialize upload service for profiles
    var profileUploadService profile.UploadService
    if cfg.UseS3 {
        profileUploadService, err = profile.NewS3UploadService(cfg.S3Bucket, cfg.S3Region)
        if err != nil {
            log.Printf("⚠️  Failed to init S3 for profiles, using local: %v", err)
            profileUploadService = profile.NewLocalUploadService(cfg.LocalUploadDir, cfg.BaseURL+"/uploads")
        } else {
            log.Println("   ✅ Using S3 for profile uploads")
        }
    } else {
        profileUploadService = profile.NewLocalUploadService(cfg.LocalUploadDir, cfg.BaseURL+"/uploads")
        log.Println("   ✅ Using local storage for profile uploads")
    }
    
    // Create profile service
    profileService := profile.NewService(profileRepo, profileUploadService)
    
    // Create profile handler
    profileHandler := profile.NewHandler(profileService)
    log.Println("✅ Profile system initialized")
    
    // 9. Initialize Auth system (UPDATED to use OTP service)
    log.Println("\n🔐 Step 9: Initializing authentication system...")
    
    authRepo := auth.NewPostgresRepository(db)
    
    authConfig := &auth.Config{
        JWTSecret:          cfg.JWTSecret,
        AccessTokenExpiry:  cfg.AccessTokenExpiry,
        RefreshTokenExpiry: cfg.RefreshTokenExpiry,
        BCryptCost:         cfg.BCryptCost,
        Enable2FA:          cfg.Enable2FA, // From config
    }
    
    // Pass OTP service to auth service
    authService := auth.NewService(authRepo, redisClient, otpService, authConfig)
    authHandler := auth.NewHandler(authService)
    authMiddleware := auth.NewMiddleware(authService)
    
    log.Println("✅ Authentication system initialized")
    
    // 10. Initialize Posts module
    log.Println("\n📝 Step 10: Initializing Posts module...")
    
    postsRepo := posts.NewRepository(db)
    
    uploadConfig := posts.UploadConfig{
        UseS3:          cfg.UseS3,
        S3Bucket:       cfg.S3Bucket,
        AWSRegion:      cfg.S3Region,
        LocalUploadDir: cfg.LocalUploadDir,
        BaseURL:        cfg.BaseURL,
    }
    
    uploadService := posts.NewUploadService(uploadConfig)
    postsService := posts.NewService(postsRepo, uploadService)
    postsHandler := posts.NewHandler(postsService)
    
    log.Println("✅ Posts module initialized")
    
    // 11. Setup routes
    log.Println("\n🛣️  Step 11: Setting up routes...")
    router := mux.NewRouter()
    
    // Static files for uploads
    if !cfg.UseS3 {
        router.PathPrefix("/uploads/").Handler(
            http.StripPrefix("/uploads/", 
                http.FileServer(http.Dir(cfg.LocalUploadDir))))
        log.Println("   ✅ Static file server configured")
    }
    
    // Health check
    router.HandleFunc("/health", healthCheck).Methods("GET")
    router.HandleFunc("/api", apiInfo).Methods("GET")
    
    // Register auth routes (includes OTP endpoints)
    authHandler.RegisterRoutes(router)
    log.Println("   ✅ Auth routes registered")
    
    // Register profile routes (NEW)
    log.Println("   - Registering profile routes...")
    registerProfileRoutes(router, profileHandler, authMiddleware)
    log.Println("   ✅ Profile routes registered")
    
    // Register posts routes
    posts.RegisterRoutes(router, postsHandler, authMiddleware)
    log.Println("   ✅ Posts routes registered")

    // Initialize Stories module
    log.Println("\n📸 Initializing Stories module...")

    storiesRepo := stories.NewPostgresRepository(sqlx.NewDb(db, "postgres"))
    storiesUploadService := stories.NewUploadService(uploadConfig) // Reuse or create new
    storiesService := stories.NewService(storiesRepo, storiesUploadService)
    storiesHandler := stories.NewHandler(storiesService)

    // Register routes
    stories.RegisterRoutes(router, storiesHandler, authMiddleware)

    // Start cleanup job
    cleanupService := stories.NewCleanupService(storiesService)
    go cleanupService.Start(context.Background())

    log.Println("✅ Stories module initialized")    
    
    // Add middleware
    router.Use(loggingMiddleware)
    router.Use(corsMiddleware)
    
    // 12. Create and start HTTP server
    srv := &http.Server{
        Addr:         fmt.Sprintf(":%s", cfg.Port),
        Handler:      router,
        ReadTimeout:  15 * time.Second,
        WriteTimeout: 15 * time.Second,
        IdleTimeout:  60 * time.Second,
    }
    
    go func() {
        log.Println("\n========================================")
        log.Printf("🚀 Server starting on http://localhost%s", srv.Addr)
        log.Printf("🌍 Environment: %s", cfg.Environment)
        log.Println("========================================")
        
        if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
            log.Fatal("❌ Failed to start server:", err)
        }
    }()
    
    // Wait for interrupt signal
    quit := make(chan os.Signal, 1)
    signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
    <-quit
    
    log.Println("\n⚠️  Shutdown signal received...")
    
    // Graceful shutdown
    ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
    defer cancel()
    
    if err := srv.Shutdown(ctx); err != nil {
        log.Fatal("❌ Server forced to shutdown:", err)
    }
    
    log.Println("✅ Server exited gracefully")
}

// Register profile routes
func registerProfileRoutes(router *mux.Router, handler *profile.Handler, authMiddleware *auth.Middleware) {
    // Protected profile routes
    api := router.PathPrefix("/api/v1").Subrouter()
    api.Use(authMiddleware.Authenticate)
    
    // Profile management
    api.HandleFunc("/profile", handler.GetMyProfile).Methods("GET")
    api.HandleFunc("/profile", handler.UpdateProfile).Methods("PUT")
    api.HandleFunc("/profile/setup", handler.SetupProfile).Methods("POST")
    api.HandleFunc("/profile/picture", handler.UploadProfilePicture).Methods("POST")
    api.HandleFunc("/profile/cover", handler.UploadCoverPhoto).Methods("POST")
    api.HandleFunc("/profile/picture", handler.DeleteProfilePicture).Methods("DELETE")
    api.HandleFunc("/profile/completion", handler.GetProfileCompletion).Methods("GET")
    
    // Privacy & Settings
    api.HandleFunc("/profile/privacy", handler.UpdatePrivacySettings).Methods("PUT")
    api.HandleFunc("/profile/notifications", handler.UpdateNotificationSettings).Methods("PUT")
    api.HandleFunc("/profile/blocked", handler.GetBlockedUsers).Methods("GET")
    
    // User interactions
    api.HandleFunc("/users/{id}/profile", handler.GetUserProfile).Methods("GET")
    api.HandleFunc("/users/{id}/block", handler.BlockUser).Methods("POST")
    api.HandleFunc("/users/{id}/block", handler.UnblockUser).Methods("DELETE")
    
    // Discovery & Search
    api.HandleFunc("/discover", handler.DiscoverProfiles).Methods("GET")
    api.HandleFunc("/search/users", handler.SearchUsers).Methods("GET")
    api.HandleFunc("/profile/views/{id}", handler.RecordProfileView).Methods("POST")
}

// OTP cleanup job
func startOTPCleanup(otpService otp.Service) {
    ticker := time.NewTicker(1 * time.Hour)
    defer ticker.Stop()
    
    for{
        select {
        case <-ticker.C:
            ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
            if err := otpService.CleanupExpiredOTPs(ctx); err != nil {
                log.Printf("Failed to cleanup expired OTPs: %v", err)
            }
            cancel()
        }
    }
}

// healthCheck returns server health status
func healthCheck(w http.ResponseWriter, r *http.Request) {
    log.Printf("📥 Health check request from %s", r.RemoteAddr)
    
    response := map[string]interface{}{
        "status":    "healthy",
        "timestamp": time.Now().Format(time.RFC3339),
        "uptime":    time.Since(startTime).String(),
    }
    
    w.Header().Set("Content-Type", "application/json")
    w.WriteHeader(http.StatusOK)
    json.NewEncoder(w).Encode(response)
}

// apiInfo returns API information - UPDATED
func apiInfo(w http.ResponseWriter, r *http.Request) {
    log.Printf("📥 API info request from %s", r.RemoteAddr)
    
    w.Header().Set("Content-Type", "application/json")
    w.WriteHeader(http.StatusOK)
    w.Write([]byte(`{
        "name": "Social Dating App API",
        "version": "1.0.0",
        "status": "running",
        "endpoints": {
            "health": "GET /health",
            "auth": {
                "signup": "POST /api/auth/signup",
                "signin": "POST /api/auth/signin",
                "verify": "POST /api/auth/verify-otp",
                "refresh": "POST /api/auth/refresh",
                "logout": "POST /api/auth/logout"
            },
            "posts": {
                "create": "POST /api/v1/posts",
                "get": "GET /api/v1/posts/{id}",
                "update": "PUT /api/v1/posts/{id}",
                "delete": "DELETE /api/v1/posts/{id}",
                "like": "POST /api/v1/posts/{id}/like",
                "unlike": "DELETE /api/v1/posts/{id}/like",
                "comment": "POST /api/v1/posts/{id}/comment",
                "feed": "GET /api/v1/posts/feed",
                "explore": "GET /api/v1/posts/explore"
            },
            "protected": {
                "me": "GET /api/v1/me (requires auth)"
            }
        }
    }`))
}

// getCurrentUser is an example protected endpoint
func getCurrentUser(authService auth.Service) http.HandlerFunc {
    return func(w http.ResponseWriter, r *http.Request) {
        log.Printf("📥 Get current user request from %s", r.RemoteAddr)
        
        // Get user ID from context (set by auth middleware)
        userIDValue := r.Context().Value("userID")
        if userIDValue == nil {
            log.Println("⚠️  No userID in context")
            w.WriteHeader(http.StatusUnauthorized)
            w.Write([]byte(`{"error":"Unauthorized"}`))
            return
        }
        
        userID, ok := userIDValue.(int64)
        if !ok {
            log.Printf("⚠️  Invalid userID type: %T", userIDValue)
            w.WriteHeader(http.StatusUnauthorized)
            w.Write([]byte(`{"error":"Invalid user ID"}`))
            return
        }
        
        log.Printf("🔍 Fetching user with ID: %d", userID)
        
        // Get user from service
        user, err := authService.GetUserByID(r.Context(), userID)
        if err != nil {
            log.Printf("❌ Error fetching user: %v", err)
            w.WriteHeader(http.StatusNotFound)
            w.Write([]byte(`{"error":"User not found"}`))
            return
        }
        
        log.Printf("✅ Found user: %s", user.Username)
        
        // Return user data
        w.Header().Set("Content-Type", "application/json")
        json.NewEncoder(w).Encode(user)
    }
}

// Middleware functions

var startTime = time.Now()

// loggingMiddleware logs all requests
func loggingMiddleware(next http.Handler) http.Handler {
    return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
        start := time.Now()
        
        // Log request details
        log.Printf("→ %s %s from %s", r.Method, r.RequestURI, r.RemoteAddr)
        
        // Wrap response writer to capture status code
        wrapped := &responseWriter{ResponseWriter: w, statusCode: http.StatusOK}
        
        next.ServeHTTP(wrapped, r)
        
        // Log response details
        duration := time.Since(start)
        log.Printf("← %s %s [%d] %v", r.Method, r.RequestURI, wrapped.statusCode, duration)
    })
}

// responseWriter wraps http.ResponseWriter to capture status code
type responseWriter struct {
    http.ResponseWriter
    statusCode int
}

func (rw *responseWriter) WriteHeader(code int) {
    rw.statusCode = code
    rw.ResponseWriter.WriteHeader(code)
}

// corsMiddleware handles CORS
func corsMiddleware(next http.Handler) http.Handler {
    return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
        w.Header().Set("Access-Control-Allow-Origin", "*")
        w.Header().Set("Access-Control-Allow-Methods", "GET, POST, PUT, DELETE, OPTIONS")
        w.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization")
        
        if r.Method == "OPTIONS" {
            log.Printf("📥 CORS preflight request from %s", r.RemoteAddr)
            w.WriteHeader(http.StatusOK)
            return
        }
        
        next.ServeHTTP(w, r)
    })
}

// runMigrations executes database migrations - UPDATED
func runMigrations(db *sql.DB) error {
    log.Println("   - Checking existing tables...")
    
    // Check if tables already exist
    var userTableExists bool
    err := db.QueryRow(`
        SELECT EXISTS (
            SELECT FROM information_schema.tables 
            WHERE table_schema = 'public' 
            AND table_name = 'users'
        )
    `).Scan(&userTableExists)
    
    if err != nil {
        return fmt.Errorf("failed to check tables: %w", err)
    }
    
    if userTableExists {
        log.Println("   ✅ Tables already exist, running additional migrations if needed...")
    }
    
    log.Println("   - Creating/updating tables...")
    
    // Create tables if they don't exist
    migrations := []string{
        // Users table (simplified to match what's already in Supabase)
        `CREATE TABLE IF NOT EXISTS users (
            id SERIAL PRIMARY KEY,
            email VARCHAR(255) UNIQUE,
            username VARCHAR(100) UNIQUE NOT NULL,
            password_hash VARCHAR(255),
            phone VARCHAR(20) UNIQUE,
            provider VARCHAR(50) DEFAULT 'local',
            provider_id VARCHAR(255),
            is_verified BOOLEAN DEFAULT FALSE,
            is_profile_complete BOOLEAN DEFAULT FALSE,
            created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
            updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
        )`,
        
        // Sessions table
        `CREATE TABLE IF NOT EXISTS sessions (
            id SERIAL PRIMARY KEY,
            user_id INTEGER NOT NULL REFERENCES users(id) ON DELETE CASCADE,
            token TEXT NOT NULL UNIQUE,
            refresh_token TEXT NOT NULL UNIQUE,
            device_info TEXT,
            ip_address VARCHAR(45),
            expires_at TIMESTAMP NOT NULL,
            created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
        )`,
        
        // Posts tables - ADD THESE
        `CREATE TABLE IF NOT EXISTS posts (
            id SERIAL PRIMARY KEY,
            user_id INTEGER NOT NULL REFERENCES users(id) ON DELETE CASCADE,
            caption TEXT,
            location VARCHAR(255),
            visibility VARCHAR(20) DEFAULT 'public',
            created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
            updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
        )`,
        
        `CREATE TABLE IF NOT EXISTS post_media (
            id SERIAL PRIMARY KEY,
            post_id INTEGER NOT NULL REFERENCES posts(id) ON DELETE CASCADE,
            media_url TEXT NOT NULL,
            media_type VARCHAR(20) NOT NULL,
            position INTEGER DEFAULT 0,
            created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
        )`,
        
        `CREATE TABLE IF NOT EXISTS post_likes (
            post_id INTEGER NOT NULL REFERENCES posts(id) ON DELETE CASCADE,
            user_id INTEGER NOT NULL REFERENCES users(id) ON DELETE CASCADE,
            created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
            PRIMARY KEY (post_id, user_id)
        )`,
        
        `CREATE TABLE IF NOT EXISTS comments (
            id SERIAL PRIMARY KEY,
            post_id INTEGER NOT NULL REFERENCES posts(id) ON DELETE CASCADE,
            user_id INTEGER NOT NULL REFERENCES users(id) ON DELETE CASCADE,
            parent_id INTEGER REFERENCES comments(id) ON DELETE CASCADE,
            content TEXT NOT NULL,
            created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
        )`,
        
        `CREATE TABLE IF NOT EXISTS follows (
            follower_id INTEGER NOT NULL REFERENCES users(id) ON DELETE CASCADE,
            following_id INTEGER NOT NULL REFERENCES users(id) ON DELETE CASCADE,
            created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
            PRIMARY KEY (follower_id, following_id)
        )`,
        
        // Create indexes
        `CREATE INDEX IF NOT EXISTS idx_users_email ON users(email)`,
        `CREATE INDEX IF NOT EXISTS idx_users_username ON users(username)`,
        `CREATE INDEX IF NOT EXISTS idx_sessions_token ON sessions(token)`,
        `CREATE INDEX IF NOT EXISTS idx_sessions_user_id ON sessions(user_id)`,
        
        // Posts indexes - ADD THESE
        `CREATE INDEX IF NOT EXISTS idx_posts_user_id ON posts(user_id)`,
        `CREATE INDEX IF NOT EXISTS idx_posts_created_at ON posts(created_at DESC)`,
        `CREATE INDEX IF NOT EXISTS idx_posts_visibility ON posts(visibility)`,
        `CREATE INDEX IF NOT EXISTS idx_post_media_post_id ON post_media(post_id)`,
        `CREATE INDEX IF NOT EXISTS idx_post_likes_post_id ON post_likes(post_id)`,
        `CREATE INDEX IF NOT EXISTS idx_post_likes_user_id ON post_likes(user_id)`,
        `CREATE INDEX IF NOT EXISTS idx_comments_post_id ON comments(post_id)`,
        `CREATE INDEX IF NOT EXISTS idx_comments_parent_id ON comments(parent_id)`,
        `CREATE INDEX IF NOT EXISTS idx_follows_follower_id ON follows(follower_id)`,
        `CREATE INDEX IF NOT EXISTS idx_follows_following_id ON follows(following_id)`,
    }
    
    for i, migration := range migrations {
        log.Printf("   - Running migration %d/%d...", i+1, len(migrations))
        if _, err := db.Exec(migration); err != nil {
            // Don't fail on duplicate key errors (indexes already exist)
            if !contains(err.Error(), "already exists") {
                return fmt.Errorf("migration %d failed: %w", i+1, err)
            }
            log.Printf("   - Migration %d skipped (already exists)", i+1)
        }
    }
    
    log.Println("   ✅ All migrations executed successfully")
    return nil
}

// Helper function
func contains(s, substr string) bool {
    return len(s) > 0 && len(substr) > 0 && (s == substr || len(s) > len(substr) && (s[:len(substr)] == substr || s[len(s)-len(substr):] == substr || len(substr) < len(s) && containsMiddle(s, substr)))
}

func containsMiddle(s, substr string) bool {
    for i := 1; i < len(s)-len(substr); i++ {
        if s[i:i+len(substr)] == substr {
            return true
        }
    }
    return false
}