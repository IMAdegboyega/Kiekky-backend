package dating

import (
    "context"
    "database/sql"
    "encoding/json"
    "time"
    
    "github.com/jmoiron/sqlx"
)

type Repository interface {
    // Date Requests
    CreateDateRequest(ctx context.Context, req *DateRequest) error
    GetDateRequest(ctx context.Context, id int64) (*DateRequest, error)
    UpdateDateRequest(ctx context.Context, req *DateRequest) error
    GetUserDateRequests(ctx context.Context, userID int64, requestType string) ([]*DateRequest, error)
    HasPendingRequest(ctx context.Context, senderID, receiverID int64) (bool, error)
    GetUpcomingDates(ctx context.Context, userID int64) ([]*DateRequest, error)
    
    // Matches
    CreateMatch(ctx context.Context, match *Match) error
    GetMatch(ctx context.Context, id int64) (*Match, error)
    GetUserMatches(ctx context.Context, userID int64, active bool) ([]*Match, error)
    UpdateMatch(ctx context.Context, match *Match) error
    IsMatched(ctx context.Context, user1ID, user2ID int64) (bool, error)
    
    // Hotpicks
    CreateHotpick(ctx context.Context, hotpick *Hotpick) error
    GetUserHotpicks(ctx context.Context, userID int64, limit int, excludeViewed bool) ([]*Hotpick, error)
    UpdateHotpick(ctx context.Context, hotpick *Hotpick) error
    DeleteExpiredHotpicks(ctx context.Context) error
    HasTodayHotpicks(ctx context.Context, userID int64) (bool, error)
    
    // User Profiles for matching
    GetUserProfile(ctx context.Context, userID int64) (*UserProfile, error)
    GetActiveUsers(ctx context.Context, daysActive int) ([]*UserProfile, error)
    FindCandidates(ctx context.Context, userID int64, filters *CandidateFilters) ([]*UserProfile, error)
    
    // Safety
    GetUserReportCount(ctx context.Context, userID int64, days int) (int, error)
    GetRecentRequestCount(ctx context.Context, userID int64, duration time.Duration) (int, error)
    GetDeclineCount(ctx context.Context, senderID, receiverID int64) (int, error)
}

type postgresRepository struct {
    db *sqlx.DB
}

func NewPostgresRepository(db *sqlx.DB) Repository {
    return &postgresRepository{db: db}
}

// Date Request Methods

func (r *postgresRepository) CreateDateRequest(ctx context.Context, req *DateRequest) error {
    query := `
        INSERT INTO date_requests (
            sender_id, receiver_id, message, proposed_date, location,
            location_lat, location_lng, date_type, duration_minutes, status
        ) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10)
        RETURNING id, created_at, updated_at
    `
    
    err := r.db.QueryRowxContext(
        ctx, query,
        req.SenderID, req.ReceiverID, req.Message, req.ProposedDate,
        req.Location, req.LocationLat, req.LocationLng,
        req.DateType, req.DurationMinutes, req.Status,
    ).Scan(&req.ID, &req.CreatedAt, &req.UpdatedAt)
    
    return err
}

func (r *postgresRepository) GetDateRequest(ctx context.Context, id int64) (*DateRequest, error) {
    var req DateRequest
    query := `
        SELECT dr.*, 
               u1.username as sender_username, u1.display_name as sender_name,
               u2.username as receiver_username, u2.display_name as receiver_name
        FROM date_requests dr
        JOIN users u1 ON dr.sender_id = u1.id
        JOIN users u2 ON dr.receiver_id = u2.id
        WHERE dr.id = $1
    `
    
    row := r.db.QueryRowxContext(ctx, query, id)
    err := row.StructScan(&req)
    if err == sql.ErrNoRows {
        return nil, ErrRequestNotFound
    }
    
    return &req, err
}

func (r *postgresRepository) UpdateDateRequest(ctx context.Context, req *DateRequest) error {
    query := `
        UPDATE date_requests
        SET status = $2, response_message = $3, declined_reason = $4,
            responded_at = $5, updated_at = CURRENT_TIMESTAMP
        WHERE id = $1
    `
    
    _, err := r.db.ExecContext(
        ctx, query,
        req.ID, req.Status, req.ResponseMessage,
        req.DeclinedReason, req.RespondedAt,
    )
    
    return err
}

func (r *postgresRepository) GetUserDateRequests(ctx context.Context, userID int64, requestType string) ([]*DateRequest, error) {
    var requests []*DateRequest
    var query string
    
    baseQuery := `
        SELECT dr.*,
               u1.id as "sender.id", u1.username as "sender.username", 
               u1.display_name as "sender.display_name", u1.profile_picture as "sender.profile_picture",
               u2.id as "receiver.id", u2.username as "receiver.username",
               u2.display_name as "receiver.display_name", u2.profile_picture as "receiver.profile_picture"
        FROM date_requests dr
        JOIN users u1 ON dr.sender_id = u1.id
        JOIN users u2 ON dr.receiver_id = u2.id
    `
    
    switch requestType {
    case "sent":
        query = baseQuery + " WHERE dr.sender_id = $1 ORDER BY dr.created_at DESC"
    case "received":
        query = baseQuery + " WHERE dr.receiver_id = $1 ORDER BY dr.created_at DESC"
    default:
        query = baseQuery + " WHERE dr.sender_id = $1 OR dr.receiver_id = $1 ORDER BY dr.created_at DESC"
    }
    
    rows, err := r.db.QueryxContext(ctx, query, userID)
    if err != nil {
        return nil, err
    }
    defer rows.Close()
    
    for rows.Next() {
        var req DateRequest
        var sender, receiver UserInfo
        
        err := rows.Scan(
            &req.ID, &req.SenderID, &req.ReceiverID, &req.Message,
            &req.ProposedDate, &req.Location, &req.LocationLat, &req.LocationLng,
            &req.DateType, &req.DurationMinutes, &req.Status,
            &req.DeclinedReason, &req.ResponseMessage, &req.RespondedAt,
            &req.CreatedAt, &req.UpdatedAt,
            &sender.ID, &sender.Username, &sender.DisplayName, &sender.ProfilePicture,
            &receiver.ID, &receiver.Username, &receiver.DisplayName, &receiver.ProfilePicture,
        )
        if err != nil {
            continue
        }
        
        req.Sender = &sender
        req.Receiver = &receiver
        requests = append(requests, &req)
    }
    
    return requests, nil
}

func (r *postgresRepository) HasPendingRequest(ctx context.Context, senderID, receiverID int64) (bool, error) {
    var exists bool
    query := `
        SELECT EXISTS(
            SELECT 1 FROM date_requests
            WHERE sender_id = $1 AND receiver_id = $2 AND status = 'pending'
        )
    `
    
    err := r.db.GetContext(ctx, &exists, query, senderID, receiverID)
    return exists, err
}

// Match Methods

func (r *postgresRepository) CreateMatch(ctx context.Context, match *Match) error {
    // Ensure user1_id < user2_id for consistency
    if match.User1ID > match.User2ID {
        match.User1ID, match.User2ID = match.User2ID, match.User1ID
    }
    
    query := `
        INSERT INTO matches (
            user1_id, user2_id, match_type, compatibility_score
        ) VALUES ($1, $2, $3, $4)
        ON CONFLICT (user1_id, user2_id) 
        DO UPDATE SET 
            is_active = TRUE,
            unmatched_by = NULL,
            unmatched_at = NULL,
            matched_at = CURRENT_TIMESTAMP
        RETURNING id, matched_at
    `
    
    err := r.db.QueryRowxContext(
        ctx, query,
        match.User1ID, match.User2ID, match.MatchType, match.CompatibilityScore,
    ).Scan(&match.ID, &match.MatchedAt)
    
    return err
}

func (r *postgresRepository) GetUserMatches(ctx context.Context, userID int64, active bool) ([]*Match, error) {
    var matches []*Match
    
    query := `
        SELECT m.*,
               CASE 
                   WHEN m.user1_id = $1 THEN u2.id
                   ELSE u1.id
               END as "matched_user.id",
               CASE 
                   WHEN m.user1_id = $1 THEN u2.username
                   ELSE u1.username
               END as "matched_user.username",
               CASE 
                   WHEN m.user1_id = $1 THEN u2.display_name
                   ELSE u1.display_name
               END as "matched_user.display_name",
               CASE 
                   WHEN m.user1_id = $1 THEN u2.profile_picture
                   ELSE u1.profile_picture
               END as "matched_user.profile_picture"
        FROM matches m
        JOIN users u1 ON m.user1_id = u1.id
        JOIN users u2 ON m.user2_id = u2.id
        WHERE (m.user1_id = $1 OR m.user2_id = $1) AND m.is_active = $2
        ORDER BY m.matched_at DESC
    `
    
    rows, err := r.db.QueryxContext(ctx, query, userID, active)
    if err != nil {
        return nil, err
    }
    defer rows.Close()
    
    for rows.Next() {
        var match Match
        var matchedUser UserInfo
        
        err := rows.Scan(
            &match.ID, &match.User1ID, &match.User2ID, &match.MatchType,
            &match.CompatibilityScore, &match.InteractionCount,
            &match.LastInteraction, &match.IsActive,
            &match.UnmatchedBy, &match.UnmatchedAt, &match.MatchedAt,
            &matchedUser.ID, &matchedUser.Username,
            &matchedUser.DisplayName, &matchedUser.ProfilePicture,
        )
        if err != nil {
            continue
        }
        
        match.MatchedUser = &matchedUser
        matches = append(matches, &match)
    }
    
    return matches, nil
}

func (r *postgresRepository) IsMatched(ctx context.Context, user1ID, user2ID int64) (bool, error) {
    // Ensure consistent ordering
    if user1ID > user2ID {
        user1ID, user2ID = user2ID, user1ID
    }
    
    var exists bool
    query := `
        SELECT EXISTS(
            SELECT 1 FROM matches
            WHERE user1_id = $1 AND user2_id = $2 AND is_active = TRUE
        )
    `
    
    err := r.db.GetContext(ctx, &exists, query, user1ID, user2ID)
    return exists, err
}

// Hotpicks Methods

func (r *postgresRepository) CreateHotpick(ctx context.Context, hotpick *Hotpick) error {
    factorsJSON, _ := json.Marshal(hotpick.Factors)
    
    query := `
        INSERT INTO hotpicks (
            user_id, recommended_user_id, score, reason, factors, expires_at
        ) VALUES ($1, $2, $3, $4, $5, $6)
        ON CONFLICT (user_id, recommended_user_id, DATE(created_at))
        DO UPDATE SET score = $3, reason = $4, factors = $5
        RETURNING id, created_at
    `
    
    err := r.db.QueryRowxContext(
        ctx, query,
        hotpick.UserID, hotpick.RecommendedUserID,
        hotpick.Score, hotpick.Reason, factorsJSON, hotpick.ExpiresAt,
    ).Scan(&hotpick.ID, &hotpick.CreatedAt)
    
    return err
}

func (r *postgresRepository) GetUserHotpicks(ctx context.Context, userID int64, limit int, excludeViewed bool) ([]*Hotpick, error) {
    var hotpicks []*Hotpick
    
    query := `
        SELECT h.*,
               u.id as "recommended_user.id",
               u.username as "recommended_user.username",
               u.display_name as "recommended_user.display_name",
               u.profile_picture as "recommended_user.profile_picture",
               u.bio as "recommended_user.bio",
               EXTRACT(YEAR FROM AGE(u.birth_date)) as "recommended_user.age"
        FROM hotpicks h
        JOIN users u ON h.recommended_user_id = u.id
        WHERE h.user_id = $1 
              AND (h.expires_at IS NULL OR h.expires_at > NOW())
    `
    
    if excludeViewed {
        query += " AND h.is_seen = FALSE"
    }
    
    query += " ORDER BY h.score DESC, h.created_at DESC LIMIT $2"
    
    rows, err := r.db.QueryxContext(ctx, query, userID, limit)
    if err != nil {
        return nil, err
    }
    defer rows.Close()
    
    for rows.Next() {
        var hotpick Hotpick
        var user UserInfo
        
        err := rows.Scan(
            &hotpick.ID, &hotpick.UserID, &hotpick.RecommendedUserID,
            &hotpick.Score, &hotpick.Reason, &hotpick.Factors,
            &hotpick.IsSeen, &hotpick.IsActedOn, &hotpick.ActionType,
            &hotpick.ExpiresAt, &hotpick.CreatedAt,
            &user.ID, &user.Username, &user.DisplayName,
            &user.ProfilePicture, &user.Bio, &user.Age,
        )
        if err != nil {
            continue
        }
        
        hotpick.RecommendedUser = &user
        hotpicks = append(hotpicks, &hotpick)
    }
    
    return hotpicks, nil
}

func (r *postgresRepository) DeleteExpiredHotpicks(ctx context.Context) error {
    query := `
        DELETE FROM hotpicks
        WHERE expires_at < NOW() OR created_at < NOW() - INTERVAL '7 days'
    `
    
    _, err := r.db.ExecContext(ctx, query)
    return err
}