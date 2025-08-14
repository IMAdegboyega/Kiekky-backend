package dating

import (
    "context"
    "errors"
    "time"
)

var (
    ErrRequestNotFound = errors.New("date request not found")
    ErrAlreadyRequested = errors.New("date request already sent")
    ErrCannotRequestSelf = errors.New("cannot send date request to yourself")
    ErrAlreadyMatched = errors.New("already matched with this user")
    ErrNotMatched = errors.New("not matched with this user")
    ErrUnauthorized = errors.New("unauthorized to perform this action")
)

type Service interface {
    // Date Requests
    CreateDateRequest(ctx context.Context, userID int64, dto *CreateDateRequestDTO) (*DateRequest, error)
    RespondToDateRequest(ctx context.Context, requestID int64, userID int64, dto *RespondDateRequestDTO) (*DateRequest, error)
    GetDateRequests(ctx context.Context, userID int64, requestType string) ([]*DateRequest, error)
    CancelDateRequest(ctx context.Context, requestID int64, userID int64) error
    GetUpcomingDates(ctx context.Context, userID int64) ([]*DateRequest, error)
    
    // Matching
    CreateMatch(ctx context.Context, user1ID, user2ID int64, matchType string) (*Match, error)
    GetMatches(ctx context.Context, userID int64, active bool) ([]*Match, error)
    UnmatchUser(ctx context.Context, matchID int64, userID int64) error
    IsMatched(ctx context.Context, user1ID, user2ID int64) (bool, error)
    
    // Hotpicks & Recommendations
    GenerateHotpicks(ctx context.Context, userID int64) error
    GetHotpicks(ctx context.Context, userID int64, params *GetHotpicksParams) ([]*Hotpick, error)
    RecordHotpickAction(ctx context.Context, hotpickID int64, action string) error
    
    // Matching Algorithm
    CalculateCompatibility(ctx context.Context, user1ID, user2ID int64) (float64, *CompatibilityFactors, error)
    FindPotentialMatches(ctx context.Context, userID int64, filters *MatchFilters) ([]*UserInfo, error)
    
    // Scheduled Jobs
    GenerateDailyHotpicks(ctx context.Context) error
    SendDateReminders(ctx context.Context) error
    CleanupExpiredHotpicks(ctx context.Context) error
}

type service struct {
    repo            Repository
    matchingEngine  MatchingEngine
    profileService  interface{}
    notifyService   interface{}
}

func (s *service) CreateDateRequest(ctx context.Context, userID int64, dto *CreateDateRequestDTO) (*DateRequest, error) {
    // Validation
    if userID == dto.ReceiverID {
        return nil, ErrCannotRequestSelf
    }
    
    // Check for existing pending request
    hasPending, err := s.repo.HasPendingRequest(ctx, userID, dto.ReceiverID)
    if err != nil {
        return nil, err
    }
    if hasPending {
        return nil, ErrAlreadyRequested
    }
    
    // Create request
    request := &DateRequest{
        SenderID:        userID,
        ReceiverID:      dto.ReceiverID,
        Message:         &dto.Message,
        Status:          "pending",
        DurationMinutes: dto.DurationMinutes,
    }
    
    if dto.ProposedDate != "" {
        t, _ := time.Parse(time.RFC3339, dto.ProposedDate)
        request.ProposedDate = &t
    }
    
    if dto.Location != "" {
        request.Location = &dto.Location
        request.LocationLat = &dto.LocationLat
        request.LocationLng = &dto.LocationLng
    }
    
    if dto.DateType != "" {
        request.DateType = &dto.DateType
    }
    
    err = s.repo.CreateDateRequest(ctx, request)
    if err != nil {
        return nil, err
    }
    
    // Send notification
    // s.notifyService.SendDateRequestNotification(dto.ReceiverID, request)
    
    return request, nil
}

func (s *service) RespondToDateRequest(ctx context.Context, requestID int64, userID int64, dto *RespondDateRequestDTO) (*DateRequest, error) {
    request, err := s.repo.GetDateRequest(ctx, requestID)
    if err != nil {
        return nil, err
    }
    
    if request.ReceiverID != userID {
        return nil, ErrUnauthorized
    }
    
    if request.Status != "pending" {
        return nil, errors.New("request already responded")
    }
    
    now := time.Now()
    request.Status = dto.Status
    request.RespondedAt = &now
    
    if dto.ResponseMessage != "" {
        request.ResponseMessage = &dto.ResponseMessage
    }
    
    if dto.Status == "declined" && dto.DeclinedReason != "" {
        request.DeclinedReason = &dto.DeclinedReason
    }
    
    err = s.repo.UpdateDateRequest(ctx, request)
    if err != nil {
        return nil, err
    }
    
    // If accepted, create a match
    if dto.Status == "accepted" {
        _, err = s.CreateMatch(ctx, request.SenderID, request.ReceiverID, "date_accepted")
        if err != nil {
            // Log error but don't fail the response
        }
    }
    
    return request, nil
}

func (s *service) GetUpcomingDates(ctx context.Context, userID int64) ([]*DateRequest, error) {
    query := `
        SELECT * FROM date_requests
        WHERE (sender_id = $1 OR receiver_id = $1)
        AND status = 'accepted'
        AND proposed_date > NOW()
        ORDER BY proposed_date ASC
    `
    
    var dates []*DateRequest
    err := s.repo.db.SelectContext(ctx, &dates, query, userID)
    return dates, err
}

func (s *service) CreateMatch(ctx context.Context, user1ID, user2ID int64, matchType string) (*Match, error) {
    // Calculate compatibility score
    score, _, err := s.matchingEngine.CalculateCompatibility(ctx, user1ID, user2ID)
    if err != nil {
        score = 0.5 // Default score on error
    }
    
    match := &Match{
        User1ID:            user1ID,
        User2ID:            user2ID,
        MatchType:          matchType,
        CompatibilityScore: &score,
        IsActive:           true,
    }
    
    err = s.repo.CreateMatch(ctx, match)
    if err != nil {
        return nil, err
    }
    
    // Record metric
    RecordMatch()
    
    // Notify users via WebSocket if available
    // s.hub.NotifyMatch(user1ID, user2ID, match)
    
    return match, nil
}

func (s *service) UnmatchUser(ctx context.Context, matchID int64, userID int64) error {
    match, err := s.repo.GetMatch(ctx, matchID)
    if err != nil {
        return err
    }
    
    // Verify user is part of the match
    if match.User1ID != userID && match.User2ID != userID {
        return ErrUnauthorized
    }
    
    match.IsActive = false
    match.UnmatchedBy = &userID
    now := time.Now()
    match.UnmatchedAt = &now
    
    return s.repo.UpdateMatch(ctx, match)
}

func (s *service) GenerateDailyHotpicks(ctx context.Context) error {
    engine := NewRecommendationEngine(s, s.matchingEngine, s.repo)
    return engine.GenerateDailyHotpicks(ctx)
}

func (s *service) SendDateReminders(ctx context.Context) error {
    // Get upcoming dates in next 2 hours
    query := `
        SELECT * FROM date_requests
        WHERE status = 'accepted'
        AND proposed_date BETWEEN NOW() AND NOW() + INTERVAL '2 hours'
        AND id NOT IN (
            SELECT date_request_id FROM date_schedules 
            WHERE reminder_sent = TRUE
        )
    `
    
    var dates []*DateRequest
    err := s.repo.db.SelectContext(ctx, &dates, query)
    if err != nil {
        return err
    }
    
    for _, date := range dates {
        // Send reminder notifications
        // s.notifyService.SendDateReminder(date.SenderID, date)
        // s.notifyService.SendDateReminder(date.ReceiverID, date)
        
        // Mark as sent
        scheduleQuery := `
            INSERT INTO date_schedules (date_request_id, scheduled_date, reminder_sent, reminder_time)
            VALUES ($1, $2, TRUE, NOW())
            ON CONFLICT (date_request_id) 
            DO UPDATE SET reminder_sent = TRUE, reminder_time = NOW()
        `
        s.repo.db.ExecContext(ctx, scheduleQuery, date.ID, date.ProposedDate)
    }
    
    return nil
}

func (s *service) CleanupExpiredHotpicks(ctx context.Context) error {
    return s.repo.DeleteExpiredHotpicks(ctx)
}