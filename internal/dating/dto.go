// internal/dating/dto.go
package dating

import "time"

// DTOs for API requests/responses

type CreateDateRequestDTO struct {
    ReceiverID      int64   `json:"receiver_id" validate:"required"`
    Message         string  `json:"message" validate:"required,min=10,max=500"`
    ProposedDate    string  `json:"proposed_date,omitempty"`
    Location        string  `json:"location,omitempty"`
    LocationLat     float64 `json:"location_lat,omitempty"`
    LocationLng     float64 `json:"location_lng,omitempty"`
    DateType        string  `json:"date_type,omitempty" validate:"omitempty,oneof=coffee dinner lunch drinks activity"`
    DurationMinutes int     `json:"duration_minutes,omitempty" validate:"omitempty,min=30,max=480"`
}

type RespondDateRequestDTO struct {
    Status          string `json:"status" validate:"required,oneof=accepted declined"`
    ResponseMessage string `json:"response_message,omitempty" validate:"omitempty,max=500"`
    DeclinedReason  string `json:"declined_reason,omitempty" validate:"omitempty,max=200"`
}

type GetHotpicksParams struct {
    Limit         int  `json:"limit"`
    ExcludeViewed bool `json:"exclude_viewed"`
}

type MatchFilters struct {
    MinAge        int     `json:"min_age"`
    MaxAge        int     `json:"max_age"`
    Gender        string  `json:"gender"`
    MaxDistance   float64 `json:"max_distance"`
    LookingFor    string  `json:"looking_for"`
    HasPhoto      bool    `json:"has_photo"`
    IsVerified    bool    `json:"is_verified"`
    Interests     []string `json:"interests"`
    Limit         int     `json:"limit"`
}

type CandidateFilters struct {
    ExcludeMatched    bool    `json:"exclude_matched"`
    ExcludeBlocked    bool    `json:"exclude_blocked"`
    ExcludeDeclined   bool    `json:"exclude_declined"`
    Gender            string  `json:"gender"`
    MinAge            int     `json:"min_age"`
    MaxAge            int     `json:"max_age"`
    MaxDistance       float64 `json:"max_distance"`
    Limit             int     `json:"limit"`
}

// Supporting types

type UserInfo struct {
    ID             int64   `json:"id" db:"id"`
    Username       string  `json:"username" db:"username"`
    DisplayName    string  `json:"display_name" db:"display_name"`
    ProfilePicture *string `json:"profile_picture,omitempty" db:"profile_picture"`
    Bio            *string `json:"bio,omitempty" db:"bio"`
    Age            *int    `json:"age,omitempty" db:"age"`
}

type UserProfile struct {
    ID                int64     `json:"id" db:"id"`
    Username          string    `json:"username" db:"username"`
    DisplayName       string    `json:"display_name" db:"display_name"`
    Bio               *string   `json:"bio,omitempty" db:"bio"`
    BirthDate         time.Time `json:"birth_date" db:"birth_date"`
    Age               int       `json:"age"`
    Gender            string    `json:"gender" db:"gender"`
    ProfilePicture    *string   `json:"profile_picture,omitempty" db:"profile_picture"`
    CoverPhoto        *string   `json:"cover_photo,omitempty" db:"cover_photo"`
    
    // Location
    Latitude          float64   `json:"latitude" db:"location_lat"`
    Longitude         float64   `json:"longitude" db:"location_lng"`
    City              *string   `json:"city,omitempty" db:"city"`
    Country           *string   `json:"country,omitempty" db:"country"`
    
    // Preferences
    Interests         []string  `json:"interests" db:"interests"`
    LookingFor        string    `json:"looking_for" db:"looking_for"`
    PreferredGender   *string   `json:"preferred_gender,omitempty" db:"preferred_gender"`
    PreferredMinAge   *int      `json:"preferred_min_age,omitempty" db:"preferred_min_age"`
    PreferredMaxAge   *int      `json:"preferred_max_age,omitempty" db:"preferred_max_age"`
    PreferredDistance *float64  `json:"preferred_distance,omitempty" db:"preferred_distance"`
    
    // Activity & Status
    LastActive        time.Time `json:"last_active" db:"last_active"`
    ActivityPattern   *string   `json:"activity_pattern,omitempty" db:"activity_pattern"`
    ResponseRate      float64   `json:"response_rate" db:"response_rate"`
    ActiveDays        int       `json:"active_days" db:"active_days"`
    
    // Profile Quality
    IsVerified        bool      `json:"is_verified" db:"is_verified"`
    CompletionScore   float64   `json:"completion_score" db:"completion_score"`
    PhotoCount        int       `json:"photo_count" db:"photo_count"`
    
    CreatedAt         time.Time `json:"created_at" db:"created_at"`
    UpdatedAt         time.Time `json:"updated_at" db:"updated_at"`
}

type ScoredCandidate struct {
    UserID  int64                 `json:"user_id"`
    Profile *UserProfile          `json:"profile"`
    Score   float64               `json:"score"`
    Factors *CompatibilityFactors `json:"factors"`
    Reason  string                `json:"reason"`
}

type UserInteraction struct {
    UserID      int64     `json:"user_id"`
    TargetID    int64     `json:"target_id"`
    Type        string    `json:"type"` // "like", "pass", "message", "date_request"
    Successful  bool      `json:"successful"`
    Timestamp   time.Time `json:"timestamp"`
}

