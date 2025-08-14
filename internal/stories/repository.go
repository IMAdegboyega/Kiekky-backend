package stories

import (
    "context"
    "database/sql"
    "fmt"
    "time"
    
    "github.com/jmoiron/sqlx"
    "github.com/lib/pq"
)

type Repository interface {
    // Story CRUD
    CreateStory(ctx context.Context, story *Story) error
    GetStory(ctx context.Context, storyID int64) (*Story, error)
    GetUserStories(ctx context.Context, userID int64, includeExpired bool) ([]*Story, error)
    GetActiveStories(ctx context.Context, excludeUserID int64, limit int, offset int) ([]*Story, error)
    GetActiveStoriesCount(ctx context.Context, excludeUserID int64) (int, error)
    DeleteStory(ctx context.Context, storyID int64) error
    GetStoryWithUser(ctx context.Context, storyID int64, viewerID int64) (*Story, error)
    
    // Views and replies
    RecordView(ctx context.Context, storyID int64, viewerID int64) error
    HasViewed(ctx context.Context, storyID int64, viewerID int64) (bool, error)
    GetStoryViewCount(ctx context.Context, storyID int64) (int, error)
    GetStoryViews(ctx context.Context, storyID int64) ([]*StoryView, error)
    CreateReply(ctx context.Context, reply *StoryReply) error
    GetStoryReplies(ctx context.Context, storyID int64) ([]*StoryReply, error)
    MarkReplyAsRead(ctx context.Context, replyID int64) error
    GetReply(ctx context.Context, replyID int64) (*StoryReply, error)
    
    // Highlights
    CreateHighlight(ctx context.Context, highlight *StoryHighlight) error
    GetUserHighlights(ctx context.Context, userID int64) ([]*StoryHighlight, error)
    GetHighlight(ctx context.Context, highlightID int64) (*StoryHighlight, error)
    DeleteHighlight(ctx context.Context, highlightID int64) error
    UpdateHighlight(ctx context.Context, highlight *StoryHighlight) error
    
    // Cleanup
    DeleteExpiredStories(ctx context.Context, before time.Time) error
    GetExpiredStoryMedia(ctx context.Context, before time.Time) ([]string, error)
    
    // User info
    GetStoryUser(ctx context.Context, userID int64) (*StoryUser, error)
}

type postgresRepository struct {
    db *sqlx.DB
}

func NewPostgresRepository(db *sqlx.DB) Repository {
    return &postgresRepository{db: db}
}

// CreateStory creates a new story
func (r *postgresRepository) CreateStory(ctx context.Context, story *Story) error {
    query := `
        INSERT INTO stories (user_id, media_url, media_type, thumbnail_url, caption, 
                           duration, is_highlighted, highlight_title, expires_at)
        VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9)
        RETURNING id, created_at, updated_at`
    
    err := r.db.QueryRowContext(ctx, query,
        story.UserID, story.MediaURL, story.MediaType, story.ThumbnailURL,
        story.Caption, story.Duration, story.IsHighlighted, story.HighlightTitle,
        story.ExpiresAt,
    ).Scan(&story.ID, &story.CreatedAt, &story.UpdatedAt)
    
    return err
}

// GetStory retrieves a story by ID
func (r *postgresRepository) GetStory(ctx context.Context, storyID int64) (*Story, error) {
    var story Story
    query := `
        SELECT id, user_id, media_url, media_type, thumbnail_url, caption,
               duration, is_highlighted, highlight_title, expires_at, created_at, updated_at
        FROM stories
        WHERE id = $1`
    
    err := r.db.GetContext(ctx, &story, query, storyID)
    if err == sql.ErrNoRows {
        return nil, ErrStoryNotFound
    }
    
    story.IsExpired = time.Now().After(story.ExpiresAt)
    return &story, err
}

// GetStoryWithUser retrieves a story with user info and view status
func (r *postgresRepository) GetStoryWithUser(ctx context.Context, storyID int64, viewerID int64) (*Story, error) {
    story, err := r.GetStory(ctx, storyID)
    if err != nil {
        return nil, err
    }
    
    // Get user info
    user, err := r.GetStoryUser(ctx, story.UserID)
    if err == nil {
        story.User = user
    }
    
    // Get view count
    story.ViewCount, _ = r.GetStoryViewCount(ctx, storyID)
    
    // Check if viewer has viewed
    if viewerID > 0 {
        story.HasViewed, _ = r.HasViewed(ctx, storyID, viewerID)
    }
    
    return story, nil
}

// GetUserStories retrieves all stories for a user
func (r *postgresRepository) GetUserStories(ctx context.Context, userID int64, includeExpired bool) ([]*Story, error) {
    query := `
        SELECT s.*, 
               COUNT(DISTINCT sv.viewer_id) as view_count
        FROM stories s
        LEFT JOIN story_views sv ON s.id = sv.story_id
        WHERE s.user_id = $1`
    
    if !includeExpired {
        query += " AND s.expires_at > NOW()"
    }
    
    query += `
        GROUP BY s.id
        ORDER BY s.created_at DESC`
    
    rows, err := r.db.QueryContext(ctx, query, userID)
    if err != nil {
        return nil, err
    }
    defer rows.Close()
    
    var stories []*Story
    for rows.Next() {
        var story Story
        err := rows.Scan(
            &story.ID, &story.UserID, &story.MediaURL, &story.MediaType,
            &story.ThumbnailURL, &story.Caption, &story.Duration,
            &story.IsHighlighted, &story.HighlightTitle, &story.ExpiresAt,
            &story.CreatedAt, &story.UpdatedAt, &story.ViewCount,
        )
        if err != nil {
            return nil, err
        }
        story.IsExpired = time.Now().After(story.ExpiresAt)
        stories = append(stories, &story)
    }
    
    return stories, nil
}

// GetActiveStories retrieves active stories from followed users
func (r *postgresRepository) GetActiveStories(ctx context.Context, excludeUserID int64, limit int, offset int) ([]*Story, error) {
    query := `
        SELECT DISTINCT ON (s.user_id) 
               s.*, u.username, u.display_name, u.profile_picture,
               COUNT(DISTINCT sv.viewer_id) as view_count,
               EXISTS(SELECT 1 FROM story_views WHERE story_id = s.id AND viewer_id = $1) as has_viewed
        FROM stories s
        INNER JOIN users u ON s.user_id = u.id
        LEFT JOIN story_views sv ON s.id = sv.story_id
        WHERE s.expires_at > NOW() AND s.user_id != $1
        GROUP BY s.id, u.id
        ORDER BY s.user_id, s.created_at DESC
        LIMIT $2 OFFSET $3`
    
    rows, err := r.db.QueryContext(ctx, query, excludeUserID, limit, offset)
    if err != nil {
        return nil, err
    }
    defer rows.Close()
    
    var stories []*Story
    for rows.Next() {
        var story Story
        var user StoryUser
        err := rows.Scan(
            &story.ID, &story.UserID, &story.MediaURL, &story.MediaType,
            &story.ThumbnailURL, &story.Caption, &story.Duration,
            &story.IsHighlighted, &story.HighlightTitle, &story.ExpiresAt,
            &story.CreatedAt, &story.UpdatedAt,
            &user.Username, &user.DisplayName, &user.ProfilePicture,
            &story.ViewCount, &story.HasViewed,
        )
        if err != nil {
            return nil, err
        }
        user.ID = story.UserID
        story.User = &user
        story.IsExpired = false
        stories = append(stories, &story)
    }
    
    return stories, nil
}

// GetActiveStoriesCount gets total count of active stories
func (r *postgresRepository) GetActiveStoriesCount(ctx context.Context, excludeUserID int64) (int, error) {
    var count int
    query := `
        SELECT COUNT(DISTINCT user_id) 
        FROM stories 
        WHERE expires_at > NOW() AND user_id != $1`
    
    err := r.db.GetContext(ctx, &count, query, excludeUserID)
    return count, err
}

// DeleteStory deletes a story
func (r *postgresRepository) DeleteStory(ctx context.Context, storyID int64) error {
    _, err := r.db.ExecContext(ctx, "DELETE FROM stories WHERE id = $1", storyID)
    return err
}

// RecordView records a story view
func (r *postgresRepository) RecordView(ctx context.Context, storyID int64, viewerID int64) error {
    query := `
        INSERT INTO story_views (story_id, viewer_id)
        VALUES ($1, $2)
        ON CONFLICT (story_id, viewer_id) DO NOTHING`
    
    _, err := r.db.ExecContext(ctx, query, storyID, viewerID)
    return err
}

// HasViewed checks if a user has viewed a story
func (r *postgresRepository) HasViewed(ctx context.Context, storyID int64, viewerID int64) (bool, error) {
    var exists bool
    query := `SELECT EXISTS(SELECT 1 FROM story_views WHERE story_id = $1 AND viewer_id = $2)`
    err := r.db.GetContext(ctx, &exists, query, storyID, viewerID)
    return exists, err
}

// GetStoryViewCount gets the view count for a story
func (r *postgresRepository) GetStoryViewCount(ctx context.Context, storyID int64) (int, error) {
    var count int
    query := `SELECT COUNT(*) FROM story_views WHERE story_id = $1`
    err := r.db.GetContext(ctx, &count, query, storyID)
    return count, err
}

// GetStoryViews retrieves all views for a story
func (r *postgresRepository) GetStoryViews(ctx context.Context, storyID int64) ([]*StoryView, error) {
    query := `
        SELECT sv.*, u.username, u.display_name, u.profile_picture
        FROM story_views sv
        INNER JOIN users u ON sv.viewer_id = u.id
        WHERE sv.story_id = $1
        ORDER BY sv.viewed_at DESC`
    
    rows, err := r.db.QueryContext(ctx, query, storyID)
    if err != nil {
        return nil, err
    }
    defer rows.Close()
    
    var views []*StoryView
    for rows.Next() {
        var view StoryView
        var user StoryUser
        err := rows.Scan(
            &view.StoryID, &view.ViewerID, &view.ViewedAt,
            &user.Username, &user.DisplayName, &user.ProfilePicture,
        )
        if err != nil {
            return nil, err
        }
        user.ID = view.ViewerID
        view.Viewer = &user
        views = append(views, &view)
    }
    
    return views, nil
}

// CreateReply creates a reply to a story
func (r *postgresRepository) CreateReply(ctx context.Context, reply *StoryReply) error {
    query := `
        INSERT INTO story_replies (story_id, user_id, message, reaction)
        VALUES ($1, $2, $3, $4)
        RETURNING id, created_at`
    
    err := r.db.QueryRowContext(ctx, query,
        reply.StoryID, reply.UserID, reply.Message, reply.Reaction,
    ).Scan(&reply.ID, &reply.CreatedAt)
    
    return err
}

// GetStoryReplies retrieves all replies for a story
func (r *postgresRepository) GetStoryReplies(ctx context.Context, storyID int64) ([]*StoryReply, error) {
    query := `
        SELECT sr.*, u.username, u.display_name, u.profile_picture
        FROM story_replies sr
        INNER JOIN users u ON sr.user_id = u.id
        WHERE sr.story_id = $1
        ORDER BY sr.created_at DESC`
    
    rows, err := r.db.QueryContext(ctx, query, storyID)
    if err != nil {
        return nil, err
    }
    defer rows.Close()
    
    var replies []*StoryReply
    for rows.Next() {
        var reply StoryReply
        var user StoryUser
        err := rows.Scan(
            &reply.ID, &reply.StoryID, &reply.UserID,
            &reply.Message, &reply.Reaction, &reply.IsRead, &reply.CreatedAt,
            &user.Username, &user.DisplayName, &user.ProfilePicture,
        )
        if err != nil {
            return nil, err
        }
        user.ID = reply.UserID
        reply.User = &user
        replies = append(replies, &reply)
    }
    
    return replies, nil
}

// GetReply retrieves a single reply
func (r *postgresRepository) GetReply(ctx context.Context, replyID int64) (*StoryReply, error) {
    var reply StoryReply
    query := `SELECT * FROM story_replies WHERE id = $1`
    err := r.db.GetContext(ctx, &reply, query, replyID)
    return &reply, err
}

// MarkReplyAsRead marks a reply as read
func (r *postgresRepository) MarkReplyAsRead(ctx context.Context, replyID int64) error {
    _, err := r.db.ExecContext(ctx,
        "UPDATE story_replies SET is_read = true WHERE id = $1", replyID)
    return err
}

// CreateHighlight creates a new highlight
func (r *postgresRepository) CreateHighlight(ctx context.Context, highlight *StoryHighlight) error {
    query := `
        INSERT INTO story_highlights (user_id, title, cover_image, story_ids)
        VALUES ($1, $2, $3, $4)
        RETURNING id, created_at, updated_at`
    
    err := r.db.QueryRowContext(ctx, query,
        highlight.UserID, highlight.Title, highlight.CoverImage,
        pq.Array(highlight.StoryIDs),
    ).Scan(&highlight.ID, &highlight.CreatedAt, &highlight.UpdatedAt)
    
    return err
}

// GetUserHighlights retrieves all highlights for a user
func (r *postgresRepository) GetUserHighlights(ctx context.Context, userID int64) ([]*StoryHighlight, error) {
    query := `
        SELECT * FROM story_highlights
        WHERE user_id = $1
        ORDER BY created_at DESC`
    
    var highlights []*StoryHighlight
    err := r.db.SelectContext(ctx, &highlights, query, userID)
    return highlights, err
}

// GetHighlight retrieves a single highlight
func (r *postgresRepository) GetHighlight(ctx context.Context, highlightID int64) (*StoryHighlight, error) {
    var highlight StoryHighlight
    query := `SELECT * FROM story_highlights WHERE id = $1`
    err := r.db.GetContext(ctx, &highlight, query, highlightID)
    return &highlight, err
}

// UpdateHighlight updates a highlight
func (r *postgresRepository) UpdateHighlight(ctx context.Context, highlight *StoryHighlight) error {
    query := `
        UPDATE story_highlights 
        SET title = $2, cover_image = $3, story_ids = $4, updated_at = NOW()
        WHERE id = $1`
    
    _, err := r.db.ExecContext(ctx, query,
        highlight.ID, highlight.Title, highlight.CoverImage,
        pq.Array(highlight.StoryIDs),
    )
    return err
}

// DeleteHighlight deletes a highlight
func (r *postgresRepository) DeleteHighlight(ctx context.Context, highlightID int64) error {
    _, err := r.db.ExecContext(ctx, "DELETE FROM story_highlights WHERE id = $1", highlightID)
    return err
}

// DeleteExpiredStories deletes expired stories
func (r *postgresRepository) DeleteExpiredStories(ctx context.Context, before time.Time) error {
    _, err := r.db.ExecContext(ctx,
        "DELETE FROM stories WHERE expires_at < $1 AND is_highlighted = false", before)
    return err
}

// GetExpiredStoryMedia retrieves media URLs of expired stories for cleanup
func (r *postgresRepository) GetExpiredStoryMedia(ctx context.Context, before time.Time) ([]string, error) {
    query := `
        SELECT media_url FROM stories 
        WHERE expires_at < $1 AND is_highlighted = false`
    
    var urls []string
    err := r.db.SelectContext(ctx, &urls, query, before)
    return urls, err
}

// GetStoryUser retrieves user info for stories
func (r *postgresRepository) GetStoryUser(ctx context.Context, userID int64) (*StoryUser, error) {
    var user StoryUser
    query := `
        SELECT id, username, display_name, profile_picture
        FROM users WHERE id = $1`
    
    err := r.db.GetContext(ctx, &user, query, userID)
    return &user, err
}