package dating

import (
    "time"
    "encoding/json"
)

type DateRequest struct {
    ID               int64      `json:"id" db:"id"`
    SenderID         int64      `json:"sender_id" db:"sender_id"`
    ReceiverID       int64      `json:"receiver_id" db:"receiver_id"`
    Message          *string    `json:"message,omitempty" db:"message"`
    ProposedDate     *time.Time `json:"proposed_date,omitempty" db:"proposed_date"`
    Location         *string    `json:"location,omitempty" db:"location"`
    LocationLat      *float64   `json:"location_lat,omitempty" db:"location_lat"`
    LocationLng      *float64   `json:"location_lng,omitempty" db:"location_lng"`
    DateType         *string    `json:"date_type,omitempty" db:"date_type"`
    DurationMinutes  int        `json:"duration_minutes" db:"duration_minutes"`
    Status           string     `json:"status" db:"status"`
    DeclinedReason   *string    `json:"declined_reason,omitempty" db:"declined_reason"`
    ResponseMessage  *string    `json:"response_message,omitempty" db:"response_message"`
    RespondedAt      *time.Time `json:"responded_at,omitempty" db:"responded_at"`
    CreatedAt        time.Time  `json:"created_at" db:"created_at"`
    UpdatedAt        time.Time  `json:"updated_at" db:"updated_at"`
    
    // Joined fields
    Sender           *UserInfo  `json:"sender,omitempty"`
    Receiver         *UserInfo  `json:"receiver,omitempty"`
}

type Match struct {
    ID                 int64      `json:"id" db:"id"`
    User1ID            int64      `json:"user1_id" db:"user1_id"`
    User2ID            int64      `json:"user2_id" db:"user2_id"`
    MatchType          string     `json:"match_type" db:"match_type"`
    CompatibilityScore *float64   `json:"compatibility_score,omitempty" db:"compatibility_score"`
    InteractionCount   int        `json:"interaction_count" db:"interaction_count"`
    LastInteraction    *time.Time `json:"last_interaction,omitempty" db:"last_interaction"`
    IsActive           bool       `json:"is_active" db:"is_active"`
    UnmatchedBy        *int64     `json:"unmatched_by,omitempty" db:"unmatched_by"`
    UnmatchedAt        *time.Time `json:"unmatched_at,omitempty" db:"unmatched_at"`
    MatchedAt          time.Time  `json:"matched_at" db:"matched_at"`
    MatchedUser        *UserInfo  `json:"matched_user,omitempty"`
}

type Hotpick struct {
    ID                int64           `json:"id" db:"id"`
    UserID            int64           `json:"user_id" db:"user_id"`
    RecommendedUserID int64           `json:"recommended_user_id" db:"recommended_user_id"`
    Score             float64         `json:"score" db:"score"`
    Reason            *string         `json:"reason,omitempty" db:"reason"`
    Factors           json.RawMessage `json:"factors,omitempty" db:"factors"`
    IsSeen            bool            `json:"is_seen" db:"is_seen"`
    IsActedOn         bool            `json:"is_acted_on" db:"is_acted_on"`
    ActionType        *string         `json:"action_type,omitempty" db:"action_type"`
    ExpiresAt         *time.Time      `json:"expires_at,omitempty" db:"expires_at"`
    CreatedAt         time.Time       `json:"created_at" db:"created_at"`
    RecommendedUser   *UserInfo       `json:"recommended_user,omitempty"`
}

type CompatibilityFactors struct {
    InterestsMatch      float64 `json:"interests_match"`
    LocationProximity   float64 `json:"location_proximity"`
    AgeCompatibility    float64 `json:"age_compatibility"`
    ActivityAlignment   float64 `json:"activity_alignment"`
    PreferencesMatch    float64 `json:"preferences_match"`
    ProfileCompleteness float64 `json:"profile_completeness"`
    EngagementLevel     float64 `json:"engagement_level"`
}