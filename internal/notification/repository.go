// internal/notification/repository.go

package notifications

import (
    "context"
    "database/sql"
    "encoding/json"
    "time"
    
    "github.com/jmoiron/sqlx"
    "github.com/lib/pq"
)

type Repository interface {
    // Notifications CRUD
    CreateNotification(ctx context.Context, notification *Notification) error
    GetNotification(ctx context.Context, notificationID int64) (*Notification, error)
    GetUserNotifications(ctx context.Context, userID int64, limit, offset int, unreadOnly bool) ([]*Notification, error)
    GetUserNotificationCount(ctx context.Context, userID int64, unreadOnly bool) (int, error)
    MarkAsRead(ctx context.Context, notificationID int64, userID int64) error
    MarkAllAsRead(ctx context.Context, userID int64) error
    DeleteNotification(ctx context.Context, notificationID int64, userID int64) error
    DeleteOldNotifications(ctx context.Context, before time.Time) error
    
    // Push tokens
    SavePushToken(ctx context.Context, token *PushToken) error
    GetUserPushTokens(ctx context.Context, userID int64, platform *Platform) ([]*PushToken, error)
    DeletePushToken(ctx context.Context, token string) error
    DeactivatePushToken(ctx context.Context, token string) error
    GetAllActivePushTokens(ctx context.Context, userIDs []int64) ([]*PushToken, error)
    
    // Preferences
    GetUserPreferences(ctx context.Context, userID int64) (*NotificationPreferences, error)
    SaveUserPreferences(ctx context.Context, prefs *NotificationPreferences) error
    UpdateUserPreferences(ctx context.Context, userID int64, updates map[string]interface{}) error
    
    // Scheduled notifications
    CreateScheduledNotification(ctx context.Context, scheduled *ScheduledNotification) error
    GetPendingScheduledNotifications(ctx context.Context, before time.Time) ([]*ScheduledNotification, error)
    UpdateScheduledNotificationStatus(ctx context.Context, id int64, status string, sentAt *time.Time) error
    CancelScheduledNotification(ctx context.Context, id int64) error
    
    // Templates
    GetTemplate(ctx context.Context, notificationType NotificationType, language string) (*NotificationTemplate, error)
    CreateTemplate(ctx context.Context, template *NotificationTemplate) error
    UpdateTemplate(ctx context.Context, template *NotificationTemplate) error
    GetAllTemplates(ctx context.Context) ([]*NotificationTemplate, error)
    
    // Batch operations
    CreateBatchNotifications(ctx context.Context, notifications []*Notification) error
    GetUsersByPreference(ctx context.Context, preference string, enabled bool) ([]int64, error)
}

type postgresRepository struct {
    db *sqlx.DB
}

func NewPostgresRepository(db *sqlx.DB) Repository {
    return &postgresRepository{db: db}
}

// CreateNotification creates a new notification
func (r *postgresRepository) CreateNotification(ctx context.Context, notification *Notification) error {
    query := `
        INSERT INTO notifications (user_id, type, title, message, data, is_read)
        VALUES ($1, $2, $3, $4, $5, $6)
        RETURNING id, created_at`
    
    dataJSON, err := json.Marshal(notification.Data)
    if err != nil {
        return err
    }
    
    err = r.db.QueryRowContext(ctx, query,
        notification.UserID,
        notification.Type,
        notification.Title,
        notification.Message,
        dataJSON,
        notification.IsRead,
    ).Scan(&notification.ID, &notification.CreatedAt)
    
    return err
}

// GetNotification retrieves a notification by ID
func (r *postgresRepository) GetNotification(ctx context.Context, notificationID int64) (*Notification, error) {
    var notification Notification
    query := `
        SELECT id, user_id, type, title, message, data, is_read, read_at, created_at
        FROM notifications
        WHERE id = $1`
    
    err := r.db.GetContext(ctx, &notification, query, notificationID)
    if err == sql.ErrNoRows {
        return nil, nil
    }
    return &notification, err
}

// GetUserNotifications retrieves notifications for a user
func (r *postgresRepository) GetUserNotifications(ctx context.Context, userID int64, limit, offset int, unreadOnly bool) ([]*Notification, error) {
    query := `
        SELECT id, user_id, type, title, message, data, is_read, read_at, created_at
        FROM notifications
        WHERE user_id = $1`
    
    if unreadOnly {
        query += " AND is_read = false"
    }
    
    query += " ORDER BY created_at DESC LIMIT $2 OFFSET $3"
    
    rows, err := r.db.QueryContext(ctx, query, userID, limit, offset)
    if err != nil {
        return nil, err
    }
    defer rows.Close()
    
    var notifications []*Notification
    for rows.Next() {
        var n Notification
        var dataJSON []byte
        
        err := rows.Scan(
            &n.ID, &n.UserID, &n.Type, &n.Title, &n.Message,
            &dataJSON, &n.IsRead, &n.ReadAt, &n.CreatedAt,
        )
        if err != nil {
            return nil, err
        }
        
        if dataJSON != nil {
            json.Unmarshal(dataJSON, &n.Data)
        }
        
        notifications = append(notifications, &n)
    }
    
    return notifications, nil
}

// GetUserNotificationCount gets notification count for a user
func (r *postgresRepository) GetUserNotificationCount(ctx context.Context, userID int64, unreadOnly bool) (int, error) {
    query := `SELECT COUNT(*) FROM notifications WHERE user_id = $1`
    
    if unreadOnly {
        query += " AND is_read = false"
    }
    
    var count int
    err := r.db.GetContext(ctx, &count, query, userID)
    return count, err
}

// MarkAsRead marks a notification as read
func (r *postgresRepository) MarkAsRead(ctx context.Context, notificationID int64, userID int64) error {
    query := `
        UPDATE notifications 
        SET is_read = true, read_at = NOW()
        WHERE id = $1 AND user_id = $2`
    
    _, err := r.db.ExecContext(ctx, query, notificationID, userID)
    return err
}

// MarkAllAsRead marks all notifications as read for a user
func (r *postgresRepository) MarkAllAsRead(ctx context.Context, userID int64) error {
    query := `
        UPDATE notifications 
        SET is_read = true, read_at = NOW()
        WHERE user_id = $1 AND is_read = false`
    
    _, err := r.db.ExecContext(ctx, query, userID)
    return err
}

// DeleteNotification deletes a notification
func (r *postgresRepository) DeleteNotification(ctx context.Context, notificationID int64, userID int64) error {
    query := `DELETE FROM notifications WHERE id = $1 AND user_id = $2`
    _, err := r.db.ExecContext(ctx, query, notificationID, userID)
    return err
}

// DeleteOldNotifications deletes old notifications
func (r *postgresRepository) DeleteOldNotifications(ctx context.Context, before time.Time) error {
    query := `DELETE FROM notifications WHERE created_at < $1`
    _, err := r.db.ExecContext(ctx, query, before)
    return err
}

// SavePushToken saves or updates a push token
func (r *postgresRepository) SavePushToken(ctx context.Context, token *PushToken) error {
    query := `
        INSERT INTO push_tokens (user_id, platform, token, device_id, is_active)
        VALUES ($1, $2, $3, $4, $5)
        ON CONFLICT (user_id, device_id) 
        DO UPDATE SET token = $3, platform = $2, is_active = $5, updated_at = NOW()
        RETURNING id, created_at, updated_at`
    
    err := r.db.QueryRowContext(ctx, query,
        token.UserID, token.Platform, token.Token, token.DeviceID, true,
    ).Scan(&token.ID, &token.CreatedAt, &token.UpdatedAt)
    
    return err
}

// GetUserPushTokens retrieves push tokens for a user
func (r *postgresRepository) GetUserPushTokens(ctx context.Context, userID int64, platform *Platform) ([]*PushToken, error) {
    query := `
        SELECT id, user_id, platform, token, device_id, is_active, created_at, updated_at
        FROM push_tokens
        WHERE user_id = $1 AND is_active = true`
    
    args := []interface{}{userID}
    
    if platform != nil {
        query += " AND platform = $2"
        args = append(args, *platform)
    }
    
    var tokens []*PushToken
    err := r.db.SelectContext(ctx, &tokens, query, args...)
    return tokens, err
}

// DeletePushToken deletes a push token
func (r *postgresRepository) DeletePushToken(ctx context.Context, token string) error {
    query := `DELETE FROM push_tokens WHERE token = $1`
    _, err := r.db.ExecContext(ctx, query, token)
    return err
}

// DeactivatePushToken deactivates a push token
func (r *postgresRepository) DeactivatePushToken(ctx context.Context, token string) error {
    query := `UPDATE push_tokens SET is_active = false WHERE token = $1`
    _, err := r.db.ExecContext(ctx, query, token)
    return err
}

// GetAllActivePushTokens gets all active push tokens for given users
func (r *postgresRepository) GetAllActivePushTokens(ctx context.Context, userIDs []int64) ([]*PushToken, error) {
    if len(userIDs) == 0 {
        query := `
            SELECT id, user_id, platform, token, device_id, is_active, created_at, updated_at
            FROM push_tokens
            WHERE is_active = true`
        
        var tokens []*PushToken
        err := r.db.SelectContext(ctx, &tokens, query)
        return tokens, err
    }
    
    query := `
        SELECT id, user_id, platform, token, device_id, is_active, created_at, updated_at
        FROM push_tokens
        WHERE user_id = ANY($1) AND is_active = true`
    
    var tokens []*PushToken
    err := r.db.SelectContext(ctx, &tokens, query, pq.Array(userIDs))
    return tokens, err
}

// GetUserPreferences retrieves user notification preferences
func (r *postgresRepository) GetUserPreferences(ctx context.Context, userID int64) (*NotificationPreferences, error) {
    var prefs NotificationPreferences
    query := `
        SELECT * FROM notification_preferences
        WHERE user_id = $1`
    
    err := r.db.GetContext(ctx, &prefs, query, userID)
    if err == sql.ErrNoRows {
        // Return default preferences if not found
        return &NotificationPreferences{
            UserID:       userID,
            PushEnabled:  true,
            EmailEnabled: true,
            SMSEnabled:   false,
            Likes:        true,
            Comments:     true,
            Follows:      true,
            Messages:     true,
            Matches:      true,
            StoryViews:   true,
            StoryReplies: true,
            Mentions:     true,
            Promotions:   true,
        }, nil
    }
    return &prefs, err
}

// SaveUserPreferences saves user notification preferences
func (r *postgresRepository) SaveUserPreferences(ctx context.Context, prefs *NotificationPreferences) error {
    query := `
        INSERT INTO notification_preferences 
        (user_id, push_enabled, email_enabled, sms_enabled, likes, comments, 
         follows, messages, matches, story_views, story_replies, mentions, promotions)
        VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13)
        ON CONFLICT (user_id) DO UPDATE SET
            push_enabled = $2, email_enabled = $3, sms_enabled = $4,
            likes = $5, comments = $6, follows = $7, messages = $8,
            matches = $9, story_views = $10, story_replies = $11,
            mentions = $12, promotions = $13, updated_at = NOW()
        RETURNING id, updated_at`
    
    err := r.db.QueryRowContext(ctx, query,
        prefs.UserID, prefs.PushEnabled, prefs.EmailEnabled, prefs.SMSEnabled,
        prefs.Likes, prefs.Comments, prefs.Follows, prefs.Messages,
        prefs.Matches, prefs.StoryViews, prefs.StoryReplies,
        prefs.Mentions, prefs.Promotions,
    ).Scan(&prefs.ID, &prefs.UpdatedAt)
    
    return err
}

// UpdateUserPreferences updates specific user preferences
func (r *postgresRepository) UpdateUserPreferences(ctx context.Context, userID int64, updates map[string]interface{}) error {
    if len(updates) == 0 {
        return nil
    }
    
    query := "UPDATE notification_preferences SET updated_at = NOW()"
    args := []interface{}{userID}
    argCount := 1
    
    for key, value := range updates {
        argCount++
        query += ", " + key + " = $" + string(rune(argCount))
        args = append(args, value)
    }
    
    query += " WHERE user_id = $1"
    
    _, err := r.db.ExecContext(ctx, query, args...)
    return err
}

// CreateScheduledNotification creates a scheduled notification
func (r *postgresRepository) CreateScheduledNotification(ctx context.Context, scheduled *ScheduledNotification) error {
    query := `
        INSERT INTO scheduled_notifications 
        (user_id, type, title, message, data, channels, scheduled_for, status)
        VALUES ($1, $2, $3, $4, $5, $6, $7, $8)
        RETURNING id, created_at`
    
    dataJSON, _ := json.Marshal(scheduled.Data)
    channelsJSON, _ := json.Marshal(scheduled.Channels)
    
    err := r.db.QueryRowContext(ctx, query,
        scheduled.UserID, scheduled.Type, scheduled.Title, scheduled.Message,
        dataJSON, channelsJSON, scheduled.ScheduledFor, "pending",
    ).Scan(&scheduled.ID, &scheduled.CreatedAt)
    
    return err
}

// GetPendingScheduledNotifications gets pending scheduled notifications
func (r *postgresRepository) GetPendingScheduledNotifications(ctx context.Context, before time.Time) ([]*ScheduledNotification, error) {
    query := `
        SELECT * FROM scheduled_notifications
        WHERE status = 'pending' AND scheduled_for <= $1
        ORDER BY scheduled_for ASC`
    
    var notifications []*ScheduledNotification
    err := r.db.SelectContext(ctx, &notifications, query, before)
    return notifications, err
}

// UpdateScheduledNotificationStatus updates scheduled notification status
func (r *postgresRepository) UpdateScheduledNotificationStatus(ctx context.Context, id int64, status string, sentAt *time.Time) error {
    query := `
        UPDATE scheduled_notifications 
        SET status = $2, sent_at = $3
        WHERE id = $1`
    
    _, err := r.db.ExecContext(ctx, query, id, status, sentAt)
    return err
}

// CancelScheduledNotification cancels a scheduled notification
func (r *postgresRepository) CancelScheduledNotification(ctx context.Context, id int64) error {
    return r.UpdateScheduledNotificationStatus(ctx, id, "cancelled", nil)
}

// GetTemplate retrieves a notification template
func (r *postgresRepository) GetTemplate(ctx context.Context, notificationType NotificationType, language string) (*NotificationTemplate, error) {
    var template NotificationTemplate
    query := `
        SELECT * FROM notification_templates
        WHERE type = $1 AND language = $2`
    
    err := r.db.GetContext(ctx, &template, query, notificationType, language)
    if err == sql.ErrNoRows {
        // Try to get default English template
        err = r.db.GetContext(ctx, &template, query, notificationType, "en")
    }
    return &template, err
}

// CreateTemplate creates a notification template
func (r *postgresRepository) CreateTemplate(ctx context.Context, template *NotificationTemplate) error {
    query := `
        INSERT INTO notification_templates 
        (type, language, title_template, body_template, variables)
        VALUES ($1, $2, $3, $4, $5)
        RETURNING id, created_at, updated_at`
    
    variablesJSON, _ := json.Marshal(template.Variables)
    
    err := r.db.QueryRowContext(ctx, query,
        template.Type, template.Language, template.TitleTemplate,
        template.BodyTemplate, variablesJSON,
    ).Scan(&template.ID, &template.CreatedAt, &template.UpdatedAt)
    
    return err
}

// UpdateTemplate updates a notification template
func (r *postgresRepository) UpdateTemplate(ctx context.Context, template *NotificationTemplate) error {
    query := `
        UPDATE notification_templates 
        SET title_template = $3, body_template = $4, variables = $5, updated_at = NOW()
        WHERE type = $1 AND language = $2`
    
    variablesJSON, _ := json.Marshal(template.Variables)
    
    _, err := r.db.ExecContext(ctx, query,
        template.Type, template.Language, template.TitleTemplate,
        template.BodyTemplate, variablesJSON,
    )
    return err
}

// GetAllTemplates retrieves all notification templates
func (r *postgresRepository) GetAllTemplates(ctx context.Context) ([]*NotificationTemplate, error) {
    query := `SELECT * FROM notification_templates ORDER BY type, language`
    
    var templates []*NotificationTemplate
    err := r.db.SelectContext(ctx, &templates, query)
    return templates, err
}

// CreateBatchNotifications creates multiple notifications
func (r *postgresRepository) CreateBatchNotifications(ctx context.Context, notifications []*Notification) error {
    tx, err := r.db.BeginTxx(ctx, nil)
    if err != nil {
        return err
    }
    defer tx.Rollback()
    
    stmt, err := tx.PrepareContext(ctx, `
        INSERT INTO notifications (user_id, type, title, message, data, is_read)
        VALUES ($1, $2, $3, $4, $5, $6)
        RETURNING id, created_at`)
    if err != nil {
        return err
    }
    defer stmt.Close()
    
    for _, n := range notifications {
        dataJSON, _ := json.Marshal(n.Data)
        err := stmt.QueryRowContext(ctx,
            n.UserID, n.Type, n.Title, n.Message, dataJSON, false,
        ).Scan(&n.ID, &n.CreatedAt)
        if err != nil {
            return err
        }
    }
    
    return tx.Commit()
}

// GetUsersByPreference gets users by notification preference
func (r *postgresRepository) GetUsersByPreference(ctx context.Context, preference string, enabled bool) ([]int64, error) {
    query := `SELECT user_id FROM notification_preferences WHERE ` + preference + ` = $1`
    
    var userIDs []int64
    err := r.db.SelectContext(ctx, &userIDs, query, enabled)
    return userIDs, err
}