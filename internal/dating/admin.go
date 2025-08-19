// internal/dating/admin.go

package dating

import (
    "context"
    "time"
)

type AdminService struct {
    repo Repository
}

type DatingStats struct {
    TotalUsers             int64     `json:"total_users"`
    ActiveUsers            int64     `json:"active_users"`
    TotalDateRequests      int64     `json:"total_date_requests"`
    AcceptanceRate         float64   `json:"acceptance_rate"`
    TotalMatches           int64     `json:"total_matches"`
    ActiveMatches          int64     `json:"active_matches"`
    AverageCompatibility   float64   `json:"average_compatibility"`
    DailyActiveUsers       int64     `json:"daily_active_users"`
    HotpicksEngagementRate float64   `json:"hotpicks_engagement_rate"`
    TopInterests           []string  `json:"top_interests"`
    PeakActivityHours      []int     `json:"peak_activity_hours"`
    LastUpdated            time.Time `json:"last_updated"`
}

type ReportedUser struct {
    UserID      int64     `json:"user_id"`
    Username    string    `json:"username"`
    ReportCount int       `json:"report_count"`
    Reasons     []string  `json:"reasons"`
    LastReport  time.Time `json:"last_report"`
    Status      string    `json:"status"`
}

func NewAdminService(repo Repository) *AdminService {
    return &AdminService{repo: repo}
}

func (a *AdminService) GetDatingStats(ctx context.Context) (*DatingStats, error) {
    stats := &DatingStats{
        LastUpdated: time.Now(),
    }
    
    // Get user stats
    userQuery := `
        SELECT 
            COUNT(*) as total,
            COUNT(CASE WHEN last_active > NOW() - INTERVAL '30 days' THEN 1 END) as active,
            COUNT(CASE WHEN last_active > NOW() - INTERVAL '1 day' THEN 1 END) as daily_active
        FROM users
    `
    err := a.repo.GetDB().QueryRowContext(ctx, userQuery).Scan(
        &stats.TotalUsers,
        &stats.ActiveUsers,
        &stats.DailyActiveUsers,
    )
    if err != nil {
        return nil, err
    }
    
    // Get date request stats
    requestQuery := `
        SELECT 
            COUNT(*) as total,
            COUNT(CASE WHEN status = 'accepted' THEN 1 END)::FLOAT / 
            NULLIF(COUNT(CASE WHEN status IN ('accepted', 'declined') THEN 1 END), 0) as acceptance_rate
        FROM date_requests
    `
    err = a.repo.GetDB().QueryRowContext(ctx, requestQuery).Scan(
        &stats.TotalDateRequests,
        &stats.AcceptanceRate,
    )
    if err != nil {
        return nil, err
    }
    
    // Get match stats
    matchQuery := `
        SELECT 
            COUNT(*) as total,
            COUNT(CASE WHEN is_active = TRUE THEN 1 END) as active,
            AVG(compatibility_score) as avg_compatibility
        FROM matches
    `
    err = a.repo.GetDB().QueryRowContext(ctx, matchQuery).Scan(
        &stats.TotalMatches,
        &stats.ActiveMatches,
        &stats.AverageCompatibility,
    )
    if err != nil {
        return nil, err
    }
    
    // Get hotpicks engagement
    hotpicksQuery := `
        SELECT 
            COUNT(CASE WHEN is_acted_on = TRUE THEN 1 END)::FLOAT / 
            NULLIF(COUNT(*), 0) as engagement_rate
        FROM hotpicks
        WHERE created_at > NOW() - INTERVAL '7 days'
    `
    err = a.repo.GetDB().GetContext(ctx, &stats.HotpicksEngagementRate, hotpicksQuery)
    if err != nil {
        return nil, err
    }
    
    return stats, nil
}

func (a *AdminService) ReviewReportedUsers(ctx context.Context) ([]*ReportedUser, error) {
    query := `
        SELECT 
            u.id,
            u.username,
            COUNT(r.id) as report_count,
            array_agg(DISTINCT r.reason) as reasons,
            MAX(r.created_at) as last_report
        FROM users u
        JOIN user_reports r ON u.id = r.reported_user_id
        WHERE r.created_at > NOW() - INTERVAL '30 days'
        GROUP BY u.id, u.username
        HAVING COUNT(r.id) >= 3
        ORDER BY COUNT(r.id) DESC
    `
    
    rows, err := a.repo.GetDB().QueryxContext(ctx, query)
    if err != nil {
        return nil, err
    }
    defer rows.Close()
    
    var reported []*ReportedUser
    for rows.Next() {
        var user ReportedUser
        err := rows.StructScan(&user)
        if err != nil {
            continue
        }
        user.Status = "pending_review"
        reported = append(reported, &user)
    }
    
    return reported, nil
}

func (a *AdminService) OverrideMatch(ctx context.Context, user1ID, user2ID int64) error {
    match := &Match{
        User1ID:   user1ID,
        User2ID:   user2ID,
        MatchType: "admin_override",
    }
    return a.repo.CreateMatch(ctx, match)
}

func (a *AdminService) SuspendUser(ctx context.Context, userID int64, reason string, duration time.Duration) error {
    // Implementation for suspending users
    return nil
}