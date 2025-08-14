// internal/otp/repository.go

package otp

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	"github.com/jmoiron/sqlx"
)

// Repository defines the OTP repository interface
type Repository interface {
	CreateOTP(ctx context.Context, otp *OTP) error
	GetOTP(ctx context.Context, id int64) (*OTP, error)
	GetLatestOTP(ctx context.Context, userID int64, otpType OTPType) (*OTP, error)
	GetLatestOTPByRecipient(ctx context.Context, recipient string, otpType OTPType) (*OTP, error)
	UpdateOTPAttempts(ctx context.Context, id int64, attempts int) error
	MarkOTPAsVerified(ctx context.Context, id int64) error
	InvalidateOTPs(ctx context.Context, userID int64, otpType OTPType) error
	CountRecentOTPs(ctx context.Context, userID int64, window time.Duration) (int, error)
	DeleteExpiredOTPs(ctx context.Context, before time.Time) error
}

// postgresRepository implements Repository using PostgreSQL
type postgresRepository struct {
	db *sqlx.DB
}

// NewPostgresRepository creates a new PostgreSQL repository
func NewPostgresRepository(db *sqlx.DB) Repository {
	return &postgresRepository{db: db}
}

// CreateOTP creates a new OTP record
func (r *postgresRepository) CreateOTP(ctx context.Context, otp *OTP) error {
	query := `
		INSERT INTO otps (user_id, code, type, method, recipient, attempts, verified, expires_at, created_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9)
		RETURNING id`

	err := r.db.QueryRowContext(
		ctx,
		query,
		otp.UserID,
		otp.Code,
		otp.Type,
		otp.Method,
		otp.Recipient,
		otp.Attempts,
		otp.Verified,
		otp.ExpiresAt,
		otp.CreatedAt,
	).Scan(&otp.ID)

	if err != nil {
		return fmt.Errorf("failed to create OTP: %w", err)
	}

	return nil
}

// GetOTP retrieves an OTP by ID
func (r *postgresRepository) GetOTP(ctx context.Context, id int64) (*OTP, error) {
	var otp OTP
	query := `
		SELECT id, user_id, code, type, method, recipient, attempts, verified, expires_at, verified_at, created_at
		FROM otps
		WHERE id = $1`

	err := r.db.GetContext(ctx, &otp, query, id)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, fmt.Errorf("OTP not found")
		}
		return nil, fmt.Errorf("failed to get OTP: %w", err)
	}

	return &otp, nil
}

// GetLatestOTP retrieves the latest OTP for a user and type
func (r *postgresRepository) GetLatestOTP(ctx context.Context, userID int64, otpType OTPType) (*OTP, error) {
	var otp OTP
	query := `
		SELECT id, user_id, code, type, method, recipient, attempts, verified, expires_at, verified_at, created_at
		FROM otps
		WHERE user_id = $1 AND type = $2 AND verified = false
		ORDER BY created_at DESC
		LIMIT 1`

	err := r.db.GetContext(ctx, &otp, query, userID, otpType)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, fmt.Errorf("no active OTP found")
		}
		return nil, fmt.Errorf("failed to get latest OTP: %w", err)
	}

	return &otp, nil
}

// GetLatestOTPByRecipient retrieves the latest OTP by recipient (email/phone)
func (r *postgresRepository) GetLatestOTPByRecipient(ctx context.Context, recipient string, otpType OTPType) (*OTP, error) {
	var otp OTP
	query := `
		SELECT id, user_id, code, type, method, recipient, attempts, verified, expires_at, verified_at, created_at
		FROM otps
		WHERE recipient = $1 AND type = $2 AND verified = false
		ORDER BY created_at DESC
		LIMIT 1`

	err := r.db.GetContext(ctx, &otp, query, recipient, otpType)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, fmt.Errorf("no active OTP found for recipient")
		}
		return nil, fmt.Errorf("failed to get OTP by recipient: %w", err)
	}

	return &otp, nil
}

// UpdateOTPAttempts updates the attempts count for an OTP
func (r *postgresRepository) UpdateOTPAttempts(ctx context.Context, id int64, attempts int) error {
	query := `UPDATE otps SET attempts = $1 WHERE id = $2`
	
	result, err := r.db.ExecContext(ctx, query, attempts, id)
	if err != nil {
		return fmt.Errorf("failed to update OTP attempts: %w", err)
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("failed to get rows affected: %w", err)
	}

	if rowsAffected == 0 {
		return fmt.Errorf("OTP not found")
	}

	return nil
}

// MarkOTPAsVerified marks an OTP as verified
func (r *postgresRepository) MarkOTPAsVerified(ctx context.Context, id int64) error {
	query := `UPDATE otps SET verified = true, verified_at = $1 WHERE id = $2`
	
	result, err := r.db.ExecContext(ctx, query, time.Now(), id)
	if err != nil {
		return fmt.Errorf("failed to mark OTP as verified: %w", err)
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("failed to get rows affected: %w", err)
	}

	if rowsAffected == 0 {
		return fmt.Errorf("OTP not found")
	}

	return nil
}

// InvalidateOTPs invalidates all unverified OTPs of a specific type for a user
func (r *postgresRepository) InvalidateOTPs(ctx context.Context, userID int64, otpType OTPType) error {
	query := `
		UPDATE otps 
		SET verified = true, verified_at = $1 
		WHERE user_id = $2 AND type = $3 AND verified = false`
	
	_, err := r.db.ExecContext(ctx, query, time.Now(), userID, otpType)
	if err != nil {
		return fmt.Errorf("failed to invalidate OTPs: %w", err)
	}

	return nil
}

// CountRecentOTPs counts OTPs created within a time window for rate limiting
func (r *postgresRepository) CountRecentOTPs(ctx context.Context, userID int64, window time.Duration) (int, error) {
	var count int
	since := time.Now().Add(-window)
	
	query := `
		SELECT COUNT(*) 
		FROM otps 
		WHERE user_id = $1 AND created_at > $2`
	
	err := r.db.GetContext(ctx, &count, query, userID, since)
	if err != nil {
		return 0, fmt.Errorf("failed to count recent OTPs: %w", err)
	}

	return count, nil
}

// DeleteExpiredOTPs deletes OTPs that have expired
func (r *postgresRepository) DeleteExpiredOTPs(ctx context.Context, before time.Time) error {
	query := `DELETE FROM otps WHERE expires_at < $1 OR (verified = true AND verified_at < $2)`
	
	// Delete verified OTPs older than 24 hours
	verifiedBefore := before.Add(-24 * time.Hour)
	
	_, err := r.db.ExecContext(ctx, query, before, verifiedBefore)
	if err != nil {
		return fmt.Errorf("failed to delete expired OTPs: %w", err)
	}

	return nil
}