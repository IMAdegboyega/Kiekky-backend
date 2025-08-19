// internal/messaging/postgres.go - COMPLETE IMPLEMENTATION

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

// Conversations
func (r *postgresRepository) CreateConversation(ctx context.Context, conv *Conversation) error {
    query := `
        INSERT INTO conversations (
            type, name, avatar_url, created_by, is_active, 
            metadata, created_at, updated_at
        ) VALUES ($1, $2, $3, $4, $5, $6, $7, $8)
        RETURNING id`
    
    err := r.db.QueryRowContext(
        ctx, query,
        conv.Type, conv.Name, conv.AvatarURL, conv.CreatedBy,
        conv.IsActive, conv.Metadata, conv.CreatedAt, conv.UpdatedAt,
    ).Scan(&conv.ID)
    
    return err
}

func (r *postgresRepository) GetConversation(ctx context.Context, id int64) (*Conversation, error) {
    query := `
        SELECT * FROM conversations WHERE id = $1`
    
    var conv Conversation
    err := r.db.GetContext(ctx, &conv, query, id)
    if err != nil {
        return nil, err
    }
    
    return &conv, nil
}

func (r *postgresRepository) GetUserConversations(ctx context.Context, userID int64, limit, offset int) ([]*Conversation, error) {
    query := `
        SELECT c.*, COUNT(m.id) FILTER (WHERE m.sender_id != $1 AND mr.read_at IS NULL) as unread_count
        FROM conversations c
        INNER JOIN conversation_participants cp ON c.id = cp.conversation_id
        LEFT JOIN messages m ON c.id = m.conversation_id
        LEFT JOIN message_receipts mr ON m.id = mr.message_id AND mr.user_id = $1
        WHERE cp.user_id = $1 AND cp.left_at IS NULL
        GROUP BY c.id
        ORDER BY c.last_message_at DESC NULLS LAST
        LIMIT $2 OFFSET $3`
    
    var conversations []*Conversation
    err := r.db.SelectContext(ctx, &conversations, query, userID, limit, offset)
    return conversations, err
}

func (r *postgresRepository) UpdateConversation(ctx context.Context, id int64, updates map[string]interface{}) error {
    // Build dynamic update query
    query := `UPDATE conversations SET updated_at = NOW()`
    args := []interface{}{id}
    argCount := 1
    
    for key, value := range updates {
        argCount++
        query += `, ` + key + ` = $` + string(rune(argCount))
        args = append(args, value)
    }
    
    query += ` WHERE id = $1`
    
    _, err := r.db.ExecContext(ctx, query, args...)
    return err
}

func (r *postgresRepository) DeleteConversation(ctx context.Context, id int64) error {
    query := `UPDATE conversations SET is_active = false WHERE id = $1`
    _, err := r.db.ExecContext(ctx, query, id)
    return err
}

func (r *postgresRepository) GetDirectConversation(ctx context.Context, user1ID, user2ID int64) (*Conversation, error) {
    query := `
        SELECT c.* FROM conversations c
        WHERE c.type = 'direct' 
        AND EXISTS (
            SELECT 1 FROM conversation_participants cp1
            WHERE cp1.conversation_id = c.id AND cp1.user_id = $1
        )
        AND EXISTS (
            SELECT 1 FROM conversation_participants cp2
            WHERE cp2.conversation_id = c.id AND cp2.user_id = $2
        )
        LIMIT 1`
    
    var conv Conversation
    err := r.db.GetContext(ctx, &conv, query, user1ID, user2ID)
    if err == sql.ErrNoRows {
        return nil, nil
    }
    return &conv, err
}

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

// Participants
func (r *postgresRepository) AddParticipant(ctx context.Context, participant *Participant) error {
    query := `
        INSERT INTO conversation_participants (
            conversation_id, user_id, role, joined_at, notification_preference
        ) VALUES ($1, $2, $3, $4, $5)
        RETURNING id`
    
    err := r.db.QueryRowContext(
        ctx, query,
        participant.ConversationID, participant.UserID, participant.Role,
        participant.JoinedAt, participant.NotificationPreference,
    ).Scan(&participant.ID)
    
    return err
}

func (r *postgresRepository) RemoveParticipant(ctx context.Context, convID, userID int64) error {
    query := `
        UPDATE conversation_participants 
        SET left_at = NOW() 
        WHERE conversation_id = $1 AND user_id = $2`
    
    _, err := r.db.ExecContext(ctx, query, convID, userID)
    return err
}

func (r *postgresRepository) GetConversationParticipants(ctx context.Context, convID int64) ([]*Participant, error) {
    query := `
        SELECT cp.*, u.id, u.username, u.display_name, u.profile_picture, u.is_online, u.last_seen
        FROM conversation_participants cp
        LEFT JOIN users u ON cp.user_id = u.id
        WHERE cp.conversation_id = $1 AND cp.left_at IS NULL`
    
    rows, err := r.db.QueryContext(ctx, query, convID)
    if err != nil {
        return nil, err
    }
    defer rows.Close()
    
    var participants []*Participant
    for rows.Next() {
        var p Participant
        var u UserInfo
        
        err := rows.Scan(
            &p.ID, &p.ConversationID, &p.UserID, &p.Role, &p.JoinedAt,
            &p.LastReadAt, &p.LastReadMessageID, &p.IsMuted, &p.MutedUntil,
            &p.IsArchived, &p.NotificationPreference, &p.UnreadCount,
            &p.IsTyping, &p.TypingStartedAt,
            &u.ID, &u.Username, &u.DisplayName, &u.ProfilePicture, &u.IsOnline, &u.LastSeen,
        )
        if err != nil {
            continue
        }
        
        p.User = &u
        participants = append(participants, &p)
    }
    
    return participants, nil
}

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

func (r *postgresRepository) UpdateLastRead(ctx context.Context, convID, userID, messageID int64) error {
    query := `
        UPDATE conversation_participants 
        SET last_read_at = NOW(), 
            last_read_message_id = $3,
            unread_count = 0
        WHERE conversation_id = $1 AND user_id = $2`
    
    _, err := r.db.ExecContext(ctx, query, convID, userID, messageID)
    return err
}

func (r *postgresRepository) IncrementUnreadCount(ctx context.Context, convID, userID int64) error {
    query := `
        UPDATE conversation_participants 
        SET unread_count = unread_count + 1
        WHERE conversation_id = $1 AND user_id = $2`
    
    _, err := r.db.ExecContext(ctx, query, convID, userID)
    return err
}

func (r *postgresRepository) ResetUnreadCount(ctx context.Context, convID, userID int64) error {
    query := `
        UPDATE conversation_participants 
        SET unread_count = 0
        WHERE conversation_id = $1 AND user_id = $2`
    
    _, err := r.db.ExecContext(ctx, query, convID, userID)
    return err
}

func (r *postgresRepository) UpdateTypingStatus(ctx context.Context, convID, userID int64, isTyping bool) error {
    query := `
        UPDATE conversation_participants 
        SET is_typing = $3, typing_started_at = CASE WHEN $3 THEN NOW() ELSE NULL END
        WHERE conversation_id = $1 AND user_id = $2`
    
    _, err := r.db.ExecContext(ctx, query, convID, userID, isTyping)
    return err
}

// Messages
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

func (r *postgresRepository) GetMessage(ctx context.Context, id int64) (*Message, error) {
    query := `SELECT * FROM messages WHERE id = $1`
    
    var msg Message
    err := r.db.GetContext(ctx, &msg, query, id)
    return &msg, err
}

func (r *postgresRepository) GetConversationMessages(ctx context.Context, convID int64, limit, offset int) ([]*Message, error) {
    query := `
        SELECT 
            m.*,
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

func (r *postgresRepository) GetUndeliveredMessages(ctx context.Context, userID int64) ([]*Message, error) {
    query := `
        SELECT m.* FROM messages m
        JOIN conversation_participants cp ON m.conversation_id = cp.conversation_id
        LEFT JOIN message_receipts mr ON m.id = mr.message_id AND mr.user_id = $1
        WHERE cp.user_id = $1 
        AND m.sender_id != $1
        AND mr.delivered_at IS NULL
        ORDER BY m.created_at ASC`
    
    var messages []*Message
    err := r.db.SelectContext(ctx, &messages, query, userID)
    return messages, err
}

func (r *postgresRepository) UpdateMessage(ctx context.Context, id int64, content string) error {
    query := `
        UPDATE messages 
        SET content = $2, is_edited = true, edited_at = NOW()
        WHERE id = $1`
    
    _, err := r.db.ExecContext(ctx, query, id, content)
    return err
}

func (r *postgresRepository) DeleteMessage(ctx context.Context, id int64) error {
    query := `
        UPDATE messages 
        SET is_deleted = true, deleted_at = NOW()
        WHERE id = $1`
    
    _, err := r.db.ExecContext(ctx, query, id)
    return err
}

func (r *postgresRepository) SearchMessages(ctx context.Context, userID int64, searchQuery string, limit int) ([]*Message, error) {
    query := `
        SELECT m.* FROM messages m
        JOIN conversation_participants cp ON m.conversation_id = cp.conversation_id
        WHERE cp.user_id = $1 
        AND m.content ILIKE $2
        AND m.is_deleted = false
        ORDER BY m.created_at DESC
        LIMIT $3`
    
    var messages []*Message
    err := r.db.SelectContext(ctx, &messages, query, userID, "%"+searchQuery+"%", limit)
    return messages, err
}

func (r *postgresRepository) MarkMessageDelivered(ctx context.Context, messageID, userID int64) error {
    query := `
        INSERT INTO message_receipts (message_id, user_id, delivered_at)
        VALUES ($1, $2, NOW())
        ON CONFLICT (message_id, user_id) 
        DO UPDATE SET delivered_at = NOW()`
    
    _, err := r.db.ExecContext(ctx, query, messageID, userID)
    return err
}

// Receipts
func (r *postgresRepository) CreateReceipt(ctx context.Context, receipt *Receipt) error {
    query := `
        INSERT INTO message_receipts (message_id, user_id, delivered_at, read_at)
        VALUES ($1, $2, $3, $4)
        ON CONFLICT (message_id, user_id) 
        DO UPDATE SET 
            delivered_at = COALESCE(message_receipts.delivered_at, $3),
            read_at = COALESCE(message_receipts.read_at, $4)
        RETURNING id`
    
    err := r.db.QueryRowContext(
        ctx, query,
        receipt.MessageID, receipt.UserID, receipt.DeliveredAt, receipt.ReadAt,
    ).Scan(&receipt.ID)
    
    return err
}

func (r *postgresRepository) UpdateReceipt(ctx context.Context, receipt *Receipt) error {
    query := `
        UPDATE message_receipts 
        SET delivered_at = $3, read_at = $4
        WHERE message_id = $1 AND user_id = $2`
    
    _, err := r.db.ExecContext(ctx, query, receipt.MessageID, receipt.UserID, receipt.DeliveredAt, receipt.ReadAt)
    return err
}

func (r *postgresRepository) GetMessageReceipts(ctx context.Context, messageID int64) ([]*Receipt, error) {
    query := `
        SELECT mr.*, u.id, u.username, u.display_name, u.profile_picture
        FROM message_receipts mr
        LEFT JOIN users u ON mr.user_id = u.id
        WHERE mr.message_id = $1`
    
    var receipts []*Receipt
    err := r.db.SelectContext(ctx, &receipts, query, messageID)
    return receipts, err
}

// Reactions
func (r *postgresRepository) AddReaction(ctx context.Context, reaction *Reaction) error {
    query := `
        INSERT INTO message_reactions (message_id, user_id, reaction, created_at)
        VALUES ($1, $2, $3, $4)
        ON CONFLICT (message_id, user_id, reaction) DO NOTHING
        RETURNING id`
    
    err := r.db.QueryRowContext(
        ctx, query,
        reaction.MessageID, reaction.UserID, reaction.Emoji, reaction.CreatedAt,
    ).Scan(&reaction.ID)
    
    if err == sql.ErrNoRows {
        // Already exists
        return nil
    }
    
    return err
}

func (r *postgresRepository) RemoveReaction(ctx context.Context, messageID, userID int64, reaction string) error {
    query := `
        DELETE FROM message_reactions 
        WHERE message_id = $1 AND user_id = $2 AND reaction = $3`
    
    _, err := r.db.ExecContext(ctx, query, messageID, userID, reaction)
    return err
}

func (r *postgresRepository) GetMessageReactions(ctx context.Context, messageID int64) ([]*Reaction, error) {
    query := `
        SELECT mr.*, u.id, u.username, u.display_name, u.profile_picture
        FROM message_reactions mr
        LEFT JOIN users u ON mr.user_id = u.id
        WHERE mr.message_id = $1`
    
    var reactions []*Reaction
    err := r.db.SelectContext(ctx, &reactions, query, messageID)
    return reactions, err
}

// Push tokens
func (r *postgresRepository) SavePushToken(ctx context.Context, userID int64, token, platform, deviceID string) error {
    query := `
        INSERT INTO push_tokens (user_id, token, platform, device_id, created_at, updated_at)
        VALUES ($1, $2, $3, $4, NOW(), NOW())
        ON CONFLICT (token) 
        DO UPDATE SET 
            user_id = $1,
            platform = $3,
            device_id = $4,
            is_active = true,
            updated_at = NOW()`
    
    _, err := r.db.ExecContext(ctx, query, userID, token, platform, deviceID)
    return err
}

func (r *postgresRepository) DeletePushToken(ctx context.Context, token string) error {
    query := `UPDATE push_tokens SET is_active = false WHERE token = $1`
    _, err := r.db.ExecContext(ctx, query, token)
    return err
}

func (r *postgresRepository) GetUserPushTokens(ctx context.Context, userID int64) ([]*PushToken, error) {
    query := `
        SELECT * FROM push_tokens 
        WHERE user_id = $1 AND is_active = true`
    
    var tokens []*PushToken
    err := r.db.SelectContext(ctx, &tokens, query, userID)
    return tokens, err
}

// Blocking
func (r *postgresRepository) BlockUser(ctx context.Context, userID, blockedUserID int64) error {
    query := `
        INSERT INTO blocked_conversations (user_id, blocked_user_id, blocked_at)
        VALUES ($1, $2, NOW())
        ON CONFLICT (user_id, blocked_user_id) DO NOTHING`
    
    _, err := r.db.ExecContext(ctx, query, userID, blockedUserID)
    return err
}

func (r *postgresRepository) UnblockUser(ctx context.Context, userID, blockedUserID int64) error {
    query := `
        DELETE FROM blocked_conversations 
        WHERE user_id = $1 AND blocked_user_id = $2`
    
    _, err := r.db.ExecContext(ctx, query, userID, blockedUserID)
    return err
}

func (r *postgresRepository) IsBlocked(ctx context.Context, userID, targetUserID int64) (bool, error) {
    query := `
        SELECT EXISTS(
            SELECT 1 FROM blocked_conversations 
            WHERE (user_id = $1 AND blocked_user_id = $2)
            OR (user_id = $2 AND blocked_user_id = $1)
        )`
    
    var exists bool
    err := r.db.QueryRowContext(ctx, query, userID, targetUserID).Scan(&exists)
    return exists, err
}

// User info
func (r *postgresRepository) GetUserInfo(ctx context.Context, userID int64) (*UserInfo, error) {
    query := `
        SELECT id, username, 
               COALESCE(display_name, username) as display_name, 
               profile_picture, is_online, last_seen
        FROM users 
        WHERE id = $1`
    
    var user UserInfo
    err := r.db.GetContext(ctx, &user, query, userID)
    return &user, err
}

func (r *postgresRepository) GetUserContacts(ctx context.Context, userID int64) ([]int64, error) {
    query := `
        SELECT DISTINCT 
            CASE 
                WHEN cp1.user_id = $1 THEN cp2.user_id
                ELSE cp1.user_id
            END as contact_id
        FROM conversation_participants cp1
        JOIN conversation_participants cp2 ON cp1.conversation_id = cp2.conversation_id
        WHERE (cp1.user_id = $1 OR cp2.user_id = $1)
        AND cp1.user_id != cp2.user_id`
    
    var contacts []int64
    err := r.db.SelectContext(ctx, &contacts, query, userID)
    return contacts, err
}

func (r *postgresRepository) UpdateUserOnlineStatus(ctx context.Context, userID int64, isOnline bool, lastSeen time.Time) error {
    query := `
        UPDATE users 
        SET is_online = $1, last_seen = $2 
        WHERE id = $3`
    
    _, err := r.db.ExecContext(ctx, query, isOnline, lastSeen, userID)
    return err
}

func (r *postgresRepository) GetTypingUsers(ctx context.Context, conversationID int64) ([]int64, error) {
    query := `
        SELECT user_id 
        FROM conversation_participants 
        WHERE conversation_id = $1 
        AND is_typing = true 
        AND typing_started_at > NOW() - INTERVAL '10 seconds'`
    
    var users []int64
    err := r.db.SelectContext(ctx, &users, query, conversationID)
    return users, err
}

func (r *postgresRepository) DeleteExpiredMessages(ctx context.Context) error {
    query := `
        DELETE FROM messages 
        WHERE expires_at IS NOT NULL 
        AND expires_at < NOW()`
    
    _, err := r.db.ExecContext(ctx, query)
    return err
}

func (r *postgresRepository) DeleteOldReceipts(ctx context.Context, age time.Duration) error {
    query := `
        DELETE FROM message_receipts 
        WHERE delivered_at < $1 OR read_at < $1`
    
    cutoff := time.Now().Add(-age)
    _, err := r.db.ExecContext(ctx, query, cutoff)
    return err
}