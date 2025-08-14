// internal/auth/repository.go
// Repository pattern isolates database queries from business logic.
// This makes it easy to change databases or add caching without touching business logic.

package auth

import (
    "context"
    "database/sql"
    "fmt"
    "time"
    
    "github.com/lib/pq"
)

// Repository interface defines all database operations for auth
// Using an interface makes testing easier - we can create mock implementations
// internal/auth/repository.go - ADD THESE METHODS TO THE INTERFACE

type Repository interface {
    // User operations
    CreateUser(ctx context.Context, user *User) error
    GetUserByID(ctx context.Context, id int64) (*User, error)
    GetUserByEmail(ctx context.Context, email string) (*User, error)
    GetUserByPhone(ctx context.Context, phone string) (*User, error)
    GetUserByUsername(ctx context.Context, username string) (*User, error)
    UpdateUser(ctx context.Context, user *User) error
    VerifyUser(ctx context.Context, userID int64) error
    
    // Validation helpers
    IsEmailTaken(ctx context.Context, email string) (bool, error)
    IsPhoneTaken(ctx context.Context, phone string) (bool, error)
    IsUsernameTaken(ctx context.Context, username string) (bool, error)
    
    // Session operations
    CreateSession(ctx context.Context, session *Session) error
    GetSessionByToken(ctx context.Context, token string) (*Session, error)
    GetSessionByRefreshToken(ctx context.Context, refreshToken string) (*Session, error)
    UpdateSession(ctx context.Context, session *Session) error
    DeleteSessionByToken(ctx context.Context, token string) error
    DeleteUserSessions(ctx context.Context, userID int64) error
}

// postgresRepository implements Repository using PostgreSQL
type postgresRepository struct {
    db *sql.DB
}

// NewPostgresRepository creates a new PostgreSQL repository
func NewPostgresRepository(db *sql.DB) Repository {
    return &postgresRepository{db: db}
}

// CreateUser inserts a new user into the database
func (r *postgresRepository) CreateUser(ctx context.Context, user *User) error {
    query := `
        INSERT INTO users (email, username, password_hash, phone, created_at, updated_at)
        VALUES ($1, $2, $3, $4, $5, $6)
        RETURNING id`
    
    // QueryRowContext executes the query and returns a single row
    // We use RETURNING to get the auto-generated ID
    err := r.db.QueryRowContext(
        ctx,
        query,
        user.Email,
        user.Username,
        user.PasswordHash,
        user.Phone,
        user.CreatedAt,
        user.UpdatedAt,
    ).Scan(&user.ID)
    
    if err != nil {
        // Check if it's a unique constraint violation
        if pgErr, ok := err.(*pq.Error); ok {
            if pgErr.Code == "23505" { // unique_violation
                return fmt.Errorf("user already exists")
            }
        }
        return fmt.Errorf("failed to create user: %w", err)
    }
    
    return nil
}

// GetUserByID retrieves a user by their ID
func (r *postgresRepository) GetUserByID(ctx context.Context, id int64) (*User, error) {
    user := &User{}
    query := `
        SELECT id, email, username, password_hash, phone, is_verified, 
               is_profile_complete, created_at, updated_at
        FROM users
        WHERE id = $1`
    
    err := r.db.QueryRowContext(ctx, query, id).Scan(
        &user.ID,
        &user.Email,
        &user.Username,
        &user.PasswordHash,
        &user.Phone,
        &user.IsVerified,
        &user.IsProfileComplete,
        &user.CreatedAt,
        &user.UpdatedAt,
    )
    
    if err == sql.ErrNoRows {
        return nil, fmt.Errorf("user not found")
    }
    if err != nil {
        return nil, fmt.Errorf("failed to get user: %w", err)
    }
    
    return user, nil
}

// GetUserByEmail retrieves a user by their email
func (r *postgresRepository) GetUserByEmail(ctx context.Context, email string) (*User, error) {
    user := &User{}
    query := `
        SELECT id, email, username, password_hash, phone, is_verified, 
               is_profile_complete, created_at, updated_at
        FROM users
        WHERE LOWER(email) = LOWER($1)`
    
    // Using LOWER() for case-insensitive comparison
    err := r.db.QueryRowContext(ctx, query, email).Scan(
        &user.ID,
        &user.Email,
        &user.Username,
        &user.PasswordHash,
        &user.Phone,
        &user.IsVerified,
        &user.IsProfileComplete,
        &user.CreatedAt,
        &user.UpdatedAt,
    )
    
    if err == sql.ErrNoRows {
        return nil, fmt.Errorf("user not found")
    }
    if err != nil {
        return nil, fmt.Errorf("failed to get user: %w", err)
    }
    
    return user, nil
}

// GetUserByUsername retrieves a user by their username
func (r *postgresRepository) GetUserByUsername(ctx context.Context, username string) (*User, error) {
    user := &User{}
    query := `
        SELECT id, email, username, password_hash, phone, is_verified, 
               is_profile_complete, created_at, updated_at
        FROM users
        WHERE LOWER(username) = LOWER($1)`
    
    err := r.db.QueryRowContext(ctx, query, username).Scan(
        &user.ID,
        &user.Email,
        &user.Username,
        &user.PasswordHash,
        &user.Phone,
        &user.IsVerified,
        &user.IsProfileComplete,
        &user.CreatedAt,
        &user.UpdatedAt,
    )
    
    if err == sql.ErrNoRows {
        return nil, fmt.Errorf("user not found")
    }
    if err != nil {
        return nil, fmt.Errorf("failed to get user: %w", err)
    }
    
    return user, nil
}

// In internal/auth/repository.go, add this method to the postgresRepository:
func (r *postgresRepository) GetUserByPhone(ctx context.Context, phone string) (*User, error) {
    var user User
    var email, passwordHash, providerID sql.NullString
    
    query := `
        SELECT id, username, email, phone, password_hash, provider, provider_id, 
               is_verified, created_at, updated_at
        FROM users
        WHERE phone = $1
        LIMIT 1
    `
    
    err := r.db.QueryRowContext(ctx, query, phone).Scan(
        &user.ID,
        &user.Username,
        &email,
        &user.Phone,
        &passwordHash,
        &user.Provider,
        &providerID,
        &user.IsVerified,
        &user.CreatedAt,
        &user.UpdatedAt,
    )
    
    if err == sql.ErrNoRows {
        return nil, ErrUserNotFound
    }
    if err != nil {
        return nil, fmt.Errorf("failed to get user by phone: %w", err)
    }
    
    // Handle nullable fields
    if email.Valid {
        user.Email = &email.String
    }
    if passwordHash.Valid {
        user.PasswordHash = &passwordHash.String
    }
    if providerID.Valid {
        user.ProviderID = &providerID.String
    }
    
    return &user, nil
}

// GetUserByEmailOrUsername allows login with either email or username
func (r *postgresRepository) GetUserByEmailOrUsername(ctx context.Context, emailOrUsername string) (*User, error) {
    user := &User{}
    query := `
        SELECT id, email, username, password_hash, phone, is_verified, 
               is_profile_complete, created_at, updated_at
        FROM users
        WHERE LOWER(email) = LOWER($1) OR LOWER(username) = LOWER($1)`
    
    err := r.db.QueryRowContext(ctx, query, emailOrUsername).Scan(
        &user.ID,
        &user.Email,
        &user.Username,
        &user.PasswordHash,
        &user.Phone,
        &user.IsVerified,
        &user.IsProfileComplete,
        &user.CreatedAt,
        &user.UpdatedAt,
    )
    
    if err == sql.ErrNoRows {
        return nil, fmt.Errorf("user not found")
    }
    if err != nil {
        return nil, fmt.Errorf("failed to get user: %w", err)
    }
    
    return user, nil
}

// UpdateUser updates user information
func (r *postgresRepository) UpdateUser(ctx context.Context, user *User) error {
    query := `
        UPDATE users 
        SET email = $1, username = $2, phone = $3, is_verified = $4, 
            is_profile_complete = $5, updated_at = $6
        WHERE id = $7`
    
    _, err := r.db.ExecContext(
        ctx,
        query,
        user.Email,
        user.Username,
        user.Phone,
        user.IsVerified,
        user.IsProfileComplete,
        time.Now(),
        user.ID,
    )
    
    if err != nil {
        return fmt.Errorf("failed to update user: %w", err)
    }
    
    return nil
}

// VerifyUser marks a user as verified
func (r *postgresRepository) VerifyUser(ctx context.Context, userID int64) error {
    query := `UPDATE users SET is_verified = true, updated_at = $1 WHERE id = $2`
    _, err := r.db.ExecContext(ctx, query, time.Now(), userID)
    if err != nil {
        return fmt.Errorf("failed to verify user: %w", err)
    }
    return nil
}

// CreateSession creates a new session
func (r *postgresRepository) CreateSession(ctx context.Context, session *Session) error {
    query := `
        INSERT INTO sessions (user_id, token, refresh_token, device_info, ip_address, expires_at, created_at)
        VALUES ($1, $2, $3, $4, $5, $6, $7)
        RETURNING id`
    
    err := r.db.QueryRowContext(
        ctx,
        query,
        session.UserID,
        session.Token,
        session.RefreshToken,
        session.DeviceInfo,
        session.IPAddress,
        session.ExpiresAt,
        session.CreatedAt,
    ).Scan(&session.ID)
    
    if err != nil {
        return fmt.Errorf("failed to create session: %w", err)
    }
    
    return nil
}

// GetSessionByToken retrieves a session by access token
func (r *postgresRepository) GetSessionByToken(ctx context.Context, token string) (*Session, error) {
    session := &Session{}
    query := `
        SELECT id, user_id, token, refresh_token, device_info, ip_address, expires_at, created_at
        FROM sessions
        WHERE token = $1 AND expires_at > NOW()`
    
    err := r.db.QueryRowContext(ctx, query, token).Scan(
        &session.ID,
        &session.UserID,
        &session.Token,
        &session.RefreshToken,
        &session.DeviceInfo,
        &session.IPAddress,
        &session.ExpiresAt,
        &session.CreatedAt,
    )
    
    if err == sql.ErrNoRows {
        return nil, fmt.Errorf("session not found or expired")
    }
    if err != nil {
        return nil, fmt.Errorf("failed to get session: %w", err)
    }
    
    return session, nil
}

// GetSessionByRefreshToken retrieves a session by refresh token
func (r *postgresRepository) GetSessionByRefreshToken(ctx context.Context, refreshToken string) (*Session, error) {
    session := &Session{}
    query := `
        SELECT id, user_id, token, refresh_token, device_info, ip_address, expires_at, created_at
        FROM sessions
        WHERE refresh_token = $1`
    
    err := r.db.QueryRowContext(ctx, query, refreshToken).Scan(
        &session.ID,
        &session.UserID,
        &session.Token,
        &session.RefreshToken,
        &session.DeviceInfo,
        &session.IPAddress,
        &session.ExpiresAt,
        &session.CreatedAt,
    )
    
    if err == sql.ErrNoRows {
        return nil, fmt.Errorf("session not found")
    }
    if err != nil {
        return nil, fmt.Errorf("failed to get session: %w", err)
    }
    
    return session, nil
}

// UpdateSession updates session tokens
func (r *postgresRepository) UpdateSession(ctx context.Context, session *Session) error {
    query := `
        UPDATE sessions 
        SET token = $1, refresh_token = $2, expires_at = $3
        WHERE id = $4`
    
    _, err := r.db.ExecContext(
        ctx,
        query,
        session.Token,
        session.RefreshToken,
        session.ExpiresAt,
        session.ID,
    )
    
    if err != nil {
        return fmt.Errorf("failed to update session: %w", err)
    }
    
    return nil
}

// DeleteSession deletes a session by ID
func (r *postgresRepository) DeleteSession(ctx context.Context, sessionID int64) error {
    query := `DELETE FROM sessions WHERE id = $1`
    
    _, err := r.db.ExecContext(ctx, query, sessionID)
    if err != nil {
        return fmt.Errorf("failed to delete session: %w", err)
    }
    
    return nil
}

// DeleteSessionByToken deletes a session by token (for logout)
func (r *postgresRepository) DeleteSessionByToken(ctx context.Context, token string) error {
    query := `DELETE FROM sessions WHERE token = $1`
    
    _, err := r.db.ExecContext(ctx, query, token)
    if err != nil {
        return fmt.Errorf("failed to delete session: %w", err)
    }
    
    return nil
}

// DeleteUserSessions deletes all sessions for a user (logout from all devices)
func (r *postgresRepository) DeleteUserSessions(ctx context.Context, userID int64) error {
    query := `DELETE FROM sessions WHERE user_id = $1`
    
    _, err := r.db.ExecContext(ctx, query, userID)
    if err != nil {
        return fmt.Errorf("failed to delete user sessions: %w", err)
    }
    
    return nil
}

// IsEmailTaken checks if an email is already registered
func (r *postgresRepository) IsEmailTaken(ctx context.Context, email string) (bool, error) {
    var exists bool
    query := `SELECT EXISTS(SELECT 1 FROM users WHERE LOWER(email) = LOWER($1))`
    
    err := r.db.QueryRowContext(ctx, query, email).Scan(&exists)
    if err != nil {
        return false, fmt.Errorf("failed to check email: %w", err)
    }
    
    return exists, nil
}

// IsPhoneTaken checks if a phone number is already registered
func (r *postgresRepository) IsPhoneTaken(ctx context.Context, phone string) (bool, error) {
    var exists bool
    query := `SELECT EXISTS(SELECT 1 FROM users WHERE phone = $1)`
    err := r.db.QueryRowContext(ctx, query, phone).Scan(&exists)
    if err != nil {
        return false, fmt.Errorf("failed to check phone: %w", err)
    }
    return exists, nil
}

// IsUsernameTaken checks if a username is already taken
func (r *postgresRepository) IsUsernameTaken(ctx context.Context, username string) (bool, error) {
    var exists bool
    query := `SELECT EXISTS(SELECT 1 FROM users WHERE LOWER(username) = LOWER($1))`
    
    err := r.db.QueryRowContext(ctx, query, username).Scan(&exists)
    if err != nil {
        return false, fmt.Errorf("failed to check username: %w", err)
    }
    
    return exists, nil
}