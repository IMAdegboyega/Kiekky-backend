// internal/auth/middleware.go
// FIXED: Removed duplicate Service interface

package auth

import (
    "context"
    "net/http"
    "strings"
    
    "github.com/imadgeboyega/kiekky-backend/internal/common/utils"
)

// Middleware provides authentication middleware
type Middleware struct {
    service Service // Uses the Service interface from service.go
}

// NewMiddleware creates a new auth middleware
func NewMiddleware(service Service) *Middleware {
    return &Middleware{
        service: service,
    }
}

// Authenticate is the main middleware function that protects routes
// It verifies the JWT token and adds user information to the request context
func (m *Middleware) Authenticate(next http.Handler) http.Handler {
    return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
        // 1. Extract token from Authorization header
        token := m.extractToken(r)
        if token == "" {
            utils.ErrorResponse(w, "Missing or invalid authorization header", http.StatusUnauthorized)
            return
        }
        
        // 2. Validate token
        claims, err := m.service.ValidateToken(r.Context(), token)
        if err != nil {
            utils.ErrorResponse(w, "Invalid or expired token", http.StatusUnauthorized)
            return
        }
        
        // 3. Check if it's an access token (not refresh)
        if claims.Type != "access" {
            utils.ErrorResponse(w, "Invalid token type", http.StatusUnauthorized)
            return
        }
        
        // 4. Add user information to request context
        // This allows handlers to access user data without another database query
        ctx := context.WithValue(r.Context(), "userID", claims.UserID)
        ctx = context.WithValue(ctx, "email", claims.Email)
        ctx = context.WithValue(ctx, "username", claims.Username)
        
        // 5. Pass to the next handler with the updated context
        next.ServeHTTP(w, r.WithContext(ctx))
    })
}

// OptionalAuthenticate is middleware for routes where auth is optional
// It adds user context if a valid token is present, but doesn't fail if missing
func (m *Middleware) OptionalAuthenticate(next http.Handler) http.Handler {
    return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
        // 1. Try to extract token
        token := m.extractToken(r)
        if token == "" {
            // No token, but that's OK - continue without user context
            next.ServeHTTP(w, r)
            return
        }
        
        // 2. Try to validate token
        claims, err := m.service.ValidateToken(r.Context(), token)
        if err != nil {
            // Invalid token, but that's OK - continue without user context
            next.ServeHTTP(w, r)
            return
        }
        
        // 3. If valid, add user context
        if claims.Type == "access" {
            ctx := context.WithValue(r.Context(), "userID", claims.UserID)
            ctx = context.WithValue(ctx, "email", claims.Email)
            ctx = context.WithValue(ctx, "username", claims.Username)
            r = r.WithContext(ctx)
        }
        
        // 4. Continue with or without user context
        next.ServeHTTP(w, r)
    })
}

// RequireVerified ensures the user has verified their email/phone
// This should be used after Authenticate for routes that need verified users
func (m *Middleware) RequireVerified(next http.Handler) http.Handler {
    return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
        // 1. Get user ID from context (set by Authenticate)
        userID, ok := r.Context().Value("userID").(int64)
        if !ok {
            utils.ErrorResponse(w, "Unauthorized", http.StatusUnauthorized)
            return
        }
        
        // 2. Check if user is verified
        user, err := m.service.GetUserByID(r.Context(), userID)
        if err != nil {
            utils.ErrorResponse(w, "User not found", http.StatusNotFound)
            return
        }
        
        if !user.IsVerified {
            utils.ErrorResponse(w, "Please verify your account first", http.StatusForbidden)
            return
        }
        
        // 3. User is verified, continue
        next.ServeHTTP(w, r)
    })
}

// RequireProfileComplete ensures the user has completed their profile
// Used for features that need a complete profile (dating, etc.)
func (m *Middleware) RequireProfileComplete(next http.Handler) http.Handler {
    return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
        // 1. Get user ID from context
        userID, ok := r.Context().Value("userID").(int64)
        if !ok {
            utils.ErrorResponse(w, "Unauthorized", http.StatusUnauthorized)
            return
        }
        
        // 2. Check if profile is complete
        user, err := m.service.GetUserByID(r.Context(), userID)
        if err != nil {
            utils.ErrorResponse(w, "User not found", http.StatusNotFound)
            return
        }
        
        if !user.IsProfileComplete {
            utils.ErrorResponse(w, "Please complete your profile first", http.StatusForbidden)
            return
        }
        
        // 3. Profile is complete, continue
        next.ServeHTTP(w, r)
    })
}

// extractToken extracts the JWT token from the Authorization header
// Supports "Bearer <token>" format
func (m *Middleware) extractToken(r *http.Request) string {
    // 1. Get Authorization header
    authHeader := r.Header.Get("Authorization")
    if authHeader == "" {
        return ""
    }
    
    // 2. Check format (should be "Bearer <token>")
    parts := strings.Split(authHeader, " ")
    if len(parts) != 2 || parts[0] != "Bearer" {
        return ""
    }
    
    // 3. Return the token part
    return parts[1]
}

// Helper functions for handlers to get user info from context

// GetUserIDFromContext extracts user ID from request context
func GetUserIDFromContext(ctx context.Context) (int64, bool) {
    userID, ok := ctx.Value("userID").(int64)
    return userID, ok
}

// GetEmailFromContext extracts email from request context
func GetEmailFromContext(ctx context.Context) (string, bool) {
    email, ok := ctx.Value("email").(string)
    return email, ok
}

// GetUsernameFromContext extracts username from request context
func GetUsernameFromContext(ctx context.Context) (string, bool) {
    username, ok := ctx.Value("username").(string)
    return username, ok
}