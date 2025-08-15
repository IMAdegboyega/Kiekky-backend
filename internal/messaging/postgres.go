// internal/messaging/postgres.go

package messaging

import (
    "context"
    "database/sql"
    "time"
    
    "github.com/jmoiron/sqlx"
)

type postgresRepository struct {
    db *sqlx.DB
}

func NewPostgresRepository(db *sqlx.DB) Repository {
    return &postgresRepository{db: db}
}

// CreateMessage creates a new message
func (r *postgresRepository) CreateMessage(ctx context.Context, message *Message) error {
    query := `
        INSERT INTO messages (
            conversation_id, sender_id, parent_message_id, content,
            message_type, media_url, media_thumbnail_url, media_size,
            media_duration, metadata, created_at
        ) VALUES (
            $1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11
        ) RETURNING id`
    
    err := r.db.QueryRowContext(
        ctx, query,
        message.ConversationID, message.SenderID, message.ParentMessageID,
        message.Content, message.MessageType, message.MediaURL,
        message.MediaThumbnailURL, message.MediaSize, message.MediaDuration,
        message.Metadata, message.CreatedAt,
    ).Scan(&message.ID)
    
    return err
}

// GetConversationMessages retrieves messages for a conversation
func (r *postgresRepository) GetConversationMessages(ctx context.Context, convID int64, limit, offset int) ([]*Message, error) {
    query := `
        SELECT 
            m.id, m.conversation_id, m.sender_id, m.parent_message_id,
            m.content, m.message_type, m.media_url, m.media_thumbnail_url,
            m.media_size, m.media_duration, m.metadata, m.is_edited,
            m.edited_at, m.is_deleted, m.deleted_at, m.delivered_at,
            m.created_at,
            u.id, u.username, u.display_name, u.profile_picture
        FROM messages m
        LEFT JOIN users u ON m.sender_id = u.id
        WHERE m.conversation_id = $1 AND m.is_deleted = false
        ORDER BY m.created_at DESC
        LIMIT $2 OFFSET $3`
    
    rows, err := r.db.QueryContext(ctx, query, convID, limit, offset)
    if err != nil {
        return nil, err
    }
    defer rows.Close()
    
    var messages []*Message
    for rows.Next() {
        var msg Message
        var sender UserInfo
        
        err := rows.Scan(
            &msg.ID, &msg.ConversationID, &msg.SenderID, &msg.ParentMessageID,
            &msg.Content, &msg.MessageType, &msg.MediaURL, &msg.MediaThumbnailURL,
            &msg.MediaSize, &msg.MediaDuration, &msg.Metadata, &msg.IsEdited,
            &msg.EditedAt, &msg.IsDeleted, &msg.DeletedAt, &msg.DeliveredAt,
            &msg.CreatedAt,
            &sender.ID, &sender.Username, &sender.DisplayName, &sender.ProfilePicture,
        )
        if err != nil {
            continue
        }
        
        msg.Sender = &sender
        messages = append(messages, &msg)
    }
    
    return messages, nil
}

// UpdateConversationLastMessage updates the last message of a conversation
func (r *postgresRepository) UpdateConversationLastMessage(ctx context.Context, convID, messageID int64, preview *string) error {
    query := `
        UPDATE conversations 
        SET last_message_at = CURRENT_TIMESTAMP,
            last_message_preview = $1,
            updated_at = CURRENT_TIMESTAMP
        WHERE id = $2`
    
    _, err := r.db.ExecContext(ctx, query, preview, convID)
    return err
}

// IncrementUnreadCount increments unread count for a user in a conversation
func (r *postgresRepository) IncrementUnreadCount(ctx context.Context, convID, userID int64) error {
    query := `
        UPDATE conversation_participants 
        SET unread_count = unread_count + 1
        WHERE conversation_id = $1 AND user_id = $2`
    
    _, err := r.db.ExecContext(ctx, query, convID, userID)
    return err
}

// ResetUnreadCount resets unread count for a user in a conversation
func (r *postgresRepository) ResetUnreadCount(ctx context.Context, convID, userID int64) error {
    query := `
        UPDATE conversation_participants 
        SET unread_count = 0
        WHERE conversation_id = $1 AND user_id = $2`
    
    _, err := r.db.ExecContext(ctx, query, convID, userID)
    return err
}

// IsUserInConversation checks if a user is a participant in a conversation
func (r *postgresRepository) IsUserInConversation(ctx context.Context, userID, convID int64) (bool, error) {
    query := `
        SELECT EXISTS(
            SELECT 1 FROM conversation_participants 
            WHERE conversation_id = $1 AND user_id = $2 AND left_at IS NULL
        )`
    
    var exists bool
    err := r.db.QueryRowContext(ctx, query, convID, userID).Scan(&exists)
    return exists, err
}