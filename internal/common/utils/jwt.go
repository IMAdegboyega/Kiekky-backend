// internal/common/utils/jwt.go
// JWT token generation and validation
// FIXED: Removed import of auth package to avoid cycle

package utils

import (
    "errors"
    "fmt"
    "strconv"
    
    "github.com/golang-jwt/jwt/v4"
)

// JWTClaims moved here to avoid import cycle
type JWTClaims struct {
    UserID   int64  `json:"user_id"`
    Email    string `json:"email"`
    Username string `json:"username"`
    Type     string `json:"type"` // "access" or "refresh"
    // Standard JWT claims
    ExpiresAt int64  `json:"exp"`
    IssuedAt  int64  `json:"iat"`
    NotBefore int64  `json:"nbf"`
    Issuer    string `json:"iss"`
    Subject   string `json:"sub"`
}

// GenerateJWT creates a new JWT token
func GenerateJWT(claims *JWTClaims, secret string) (string, error) {
    // Create token with claims
    token := jwt.NewWithClaims(jwt.SigningMethodHS256, jwt.MapClaims{
        "user_id":  fmt.Sprintf("%d", claims.UserID), // Convert to string
        "email":    claims.Email,
        "username": claims.Username,
        "type":     claims.Type,
        "exp":      claims.ExpiresAt,
        "iat":      claims.IssuedAt,
        "nbf":      claims.NotBefore,
        "iss":      claims.Issuer,
        "sub":      claims.Subject,
    })
    
    // Sign token with secret
    tokenString, err := token.SignedString([]byte(secret))
    if err != nil {
        return "", fmt.Errorf("failed to sign token: %w", err)
    }
    
    return tokenString, nil
}

// ValidateJWT validates a JWT token and returns claims
func ValidateJWT(tokenString string, secret string) (*JWTClaims, error) {
    // Parse token
    token, err := jwt.Parse(tokenString, func(token *jwt.Token) (interface{}, error) {
        // Verify signing method
        if _, ok := token.Method.(*jwt.SigningMethodHMAC); !ok {
            return nil, fmt.Errorf("unexpected signing method: %v", token.Header["alg"])
        }
        return []byte(secret), nil
    })
    
    if err != nil {
        return nil, err
    }
    
    // Extract claims
    if claims, ok := token.Claims.(jwt.MapClaims); ok && token.Valid {
        // Parse user ID
        userIDStr, ok := claims["user_id"].(string)
        if !ok {
            return nil, errors.New("invalid user_id in token")
        }
        
        userID, err := strconv.ParseInt(userIDStr, 10, 64)
        if err != nil {
            return nil, errors.New("invalid user_id format")
        }
        
        return &JWTClaims{
            UserID:    userID,
            Email:     getStringClaim(claims, "email"),
            Username:  getStringClaim(claims, "username"),
            Type:      getStringClaim(claims, "type"),
            ExpiresAt: getInt64Claim(claims, "exp"),
            IssuedAt:  getInt64Claim(claims, "iat"),
            NotBefore: getInt64Claim(claims, "nbf"),
            Issuer:    getStringClaim(claims, "iss"),
            Subject:   getStringClaim(claims, "sub"),
        }, nil
    }
    
    return nil, errors.New("invalid token")
}

// Helper functions to safely extract claims
func getStringClaim(claims jwt.MapClaims, key string) string {
    if val, ok := claims[key].(string); ok {
        return val
    }
    return ""
}

func getInt64Claim(claims jwt.MapClaims, key string) int64 {
    if val, ok := claims[key].(float64); ok {
        return int64(val)
    }
    return 0
}