// internal/posts/repository.go
package posts

import (
	"database/sql"
	"fmt"
	"strings"
)

type Repository struct {
	db *sql.DB
}

func NewRepository(db *sql.DB) *Repository {
	return &Repository{db: db}
}

func (r *Repository) CreatePost(post *Post) error {
	query := `
		INSERT INTO posts (user_id, caption, location, visibility, created_at, updated_at)
		VALUES ($1, $2, $3, $4, NOW(), NOW())
		RETURNING id, created_at, updated_at`
	
	err := r.db.QueryRow(query, post.UserID, post.Caption, post.Location, post.Visibility).
		Scan(&post.ID, &post.CreatedAt, &post.UpdatedAt)
	return err
}

func (r *Repository) AddPostMedia(media []PostMedia) error {
	if len(media) == 0 {
		return nil
	}
	
	valueStrings := make([]string, 0, len(media))
	valueArgs := make([]interface{}, 0, len(media)*4)
	
	for i, m := range media {
		valueStrings = append(valueStrings, fmt.Sprintf("($%d, $%d, $%d, $%d)",
			i*4+1, i*4+2, i*4+3, i*4+4))
		valueArgs = append(valueArgs, m.PostID, m.MediaURL, m.MediaType, m.Position)
	}
	
	query := fmt.Sprintf(`
		INSERT INTO post_media (post_id, media_url, media_type, position)
		VALUES %s`, strings.Join(valueStrings, ","))
	
	_, err := r.db.Exec(query, valueArgs...)
	return err
}

func (r *Repository) GetPostByID(postID, userID int64) (*Post, error) {
	query := `
		SELECT 
			p.id, p.user_id, p.caption, p.location, p.visibility, 
			p.created_at, p.updated_at,
			u.username, 
			COALESCE(u.profile_picture, '') as profile_picture,  -- Handle NULL
			COUNT(DISTINCT l.user_id) as likes_count,
			COUNT(DISTINCT c.id) as comments_count,
			EXISTS(SELECT 1 FROM post_likes WHERE post_id = p.id AND user_id = $2) as is_liked
		FROM posts p
		JOIN users u ON p.user_id = u.id
		LEFT JOIN post_likes l ON p.id = l.post_id
		LEFT JOIN comments c ON p.id = c.post_id
		WHERE p.id = $1
		GROUP BY p.id, u.id, u.username, u.profile_picture --  Add to GROUP BY`
	
	post := &Post{User: &UserInfo{}}
	err := r.db.QueryRow(query, postID, userID).Scan(
		&post.ID, &post.UserID, &post.Caption, &post.Location, &post.Visibility,
		&post.CreatedAt, &post.UpdatedAt,
		&post.User.Username, &post.User.ProfilePicture,
		&post.LikesCount, &post.CommentsCount, &post.IsLiked,
	)
	
	if err != nil {
		return nil, err
	}
	
	post.User.ID = post.UserID
	
	// Get media
	media, err := r.GetPostMedia(postID)
	if err != nil {
		return nil, err
	}
	post.Media = media
	
	return post, nil
}

func (r *Repository) GetPostMedia(postID int64) ([]PostMedia, error) {
	query := `SELECT id, post_id, media_url, media_type, position 
			  FROM post_media WHERE post_id = $1 ORDER BY position`
	
	rows, err := r.db.Query(query, postID)
	if err != nil {
		return []PostMedia{}, nil // Return empty array on error
	}
	defer rows.Close()
	
	var media []PostMedia
	for rows.Next() {
		var m PostMedia
		err := rows.Scan(&m.ID, &m.PostID, &m.MediaURL, &m.MediaType, &m.Position)
		if err != nil {
			continue
		}
		media = append(media, m)
	}
	
	return media, nil
}

func (r *Repository) UpdatePost(postID int64, update *UpdatePostRequest) error {
	var setClauses []string
	var args []interface{}
	argCount := 1
	
	if update.Caption != "" {
		setClauses = append(setClauses, fmt.Sprintf("caption = $%d", argCount))
		args = append(args, update.Caption)
		argCount++
	}
	
	if update.Location != "" {
		setClauses = append(setClauses, fmt.Sprintf("location = $%d", argCount))
		args = append(args, update.Location)
		argCount++
	}
	
	if update.Visibility != "" {
		setClauses = append(setClauses, fmt.Sprintf("visibility = $%d", argCount))
		args = append(args, update.Visibility)
		argCount++
	}
	
	if len(setClauses) == 0 {
		return nil
	}
	
	setClauses = append(setClauses, "updated_at = NOW()")
	args = append(args, postID)
	
	query := fmt.Sprintf("UPDATE posts SET %s WHERE id = $%d",
		strings.Join(setClauses, ", "), argCount)
	
	_, err := r.db.Exec(query, args...)
	return err
}

func (r *Repository) DeletePost(postID int64) error {
	tx, err := r.db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()
	
	// Delete related data
	_, err = tx.Exec("DELETE FROM post_media WHERE post_id = $1", postID)
	if err != nil {
		return err
	}
	
	_, err = tx.Exec("DELETE FROM post_likes WHERE post_id = $1", postID)
	if err != nil {
		return err
	}
	
	_, err = tx.Exec("DELETE FROM comments WHERE post_id = $1", postID)
	if err != nil {
		return err
	}
	
	_, err = tx.Exec("DELETE FROM posts WHERE id = $1", postID)
	if err != nil {
		return err
	}
	
	return tx.Commit()
}

func (r *Repository) LikePost(postID, userID int64) error {
	query := `INSERT INTO post_likes (post_id, user_id, created_at) 
			  VALUES ($1, $2, NOW()) ON CONFLICT DO NOTHING`
	_, err := r.db.Exec(query, postID, userID)
	return err
}

func (r *Repository) UnlikePost(postID, userID int64) error {
	query := `DELETE FROM post_likes WHERE post_id = $1 AND user_id = $2`
	_, err := r.db.Exec(query, postID, userID)
	return err
}

func (r *Repository) GetPostLikes(postID int64, limit, offset int) ([]Like, int, error) {
	// Get total count
	var total int
	countQuery := `SELECT COUNT(*) FROM post_likes WHERE post_id = $1`
	err := r.db.QueryRow(countQuery, postID).Scan(&total)
	if err != nil {
		return nil, 0, err
	}
	
	// Get likes with user info - FIXED
	query := `
		SELECT l.post_id, l.user_id, l.created_at, 
		       u.username, 
		       COALESCE(u.profile_picture, '') as profile_picture  -- Handle NULL
		FROM post_likes l
		JOIN users u ON l.user_id = u.id
		WHERE l.post_id = $1
		ORDER BY l.created_at DESC
		LIMIT $2 OFFSET $3`
	
	rows, err := r.db.Query(query, postID, limit, offset)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()
	
	var likes []Like
	for rows.Next() {
		like := Like{User: &UserInfo{}}
		err := rows.Scan(&like.PostID, &like.UserID, &like.CreatedAt,
			&like.User.Username, &like.User.ProfilePicture)
		if err != nil {
			return nil, 0, err
		}
		like.User.ID = like.UserID
		likes = append(likes, like)
	}
	
	return likes, total, nil
}

func (r *Repository) CreateComment(comment *Comment) error {
	query := `
		INSERT INTO comments (post_id, user_id, parent_id, content, created_at)
		VALUES ($1, $2, $3, $4, NOW())
		RETURNING id, created_at`
	
	err := r.db.QueryRow(query, comment.PostID, comment.UserID, comment.ParentID, comment.Content).
		Scan(&comment.ID, &comment.CreatedAt)
	return err
}

func (r *Repository) GetPostComments(postID int64, limit, offset int) ([]Comment, int, error) {
	// Get total count
	var total int
	countQuery := `SELECT COUNT(*) FROM comments WHERE post_id = $1 AND parent_id IS NULL`
	err := r.db.QueryRow(countQuery, postID).Scan(&total)
	if err != nil {
		return nil, 0, err
	}
	
	// Get top-level comments - FIXED
	query := `
		SELECT c.id, c.post_id, c.user_id, c.content, c.created_at,
		       u.username, 
		       COALESCE(u.profile_picture, '') as profile_picture  -- Handle NULL
		FROM comments c
		JOIN users u ON c.user_id = u.id
		WHERE c.post_id = $1 AND c.parent_id IS NULL
		ORDER BY c.created_at DESC
		LIMIT $2 OFFSET $3`
	
	rows, err := r.db.Query(query, postID, limit, offset)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()
	
	var comments []Comment
	for rows.Next() {
		comment := Comment{User: &UserInfo{}}
		err := rows.Scan(&comment.ID, &comment.PostID, &comment.UserID,
			&comment.Content, &comment.CreatedAt,
			&comment.User.Username, &comment.User.ProfilePicture)
		if err != nil {
			return nil, 0, err
		}
		comment.User.ID = comment.UserID
		
		// Get replies for each comment
		replies, err := r.GetCommentReplies(comment.ID)
		if err != nil {
			return nil, 0, err
		}
		comment.Replies = replies
		
		comments = append(comments, comment)
	}
	
	return comments, total, nil
}

func (r *Repository) GetCommentReplies(parentID int64) ([]Comment, error) {
	query := `
		SELECT c.id, c.post_id, c.user_id, c.parent_id, c.content, c.created_at,
		       u.username, 
		       COALESCE(u.profile_picture, '') as profile_picture  -- Handle NULL
		FROM comments c
		JOIN users u ON c.user_id = u.id
		WHERE c.parent_id = $1
		ORDER BY c.created_at ASC`
	
	rows, err := r.db.Query(query, parentID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	
	var replies []Comment
	for rows.Next() {
		reply := Comment{User: &UserInfo{}}
		err := rows.Scan(&reply.ID, &reply.PostID, &reply.UserID, &reply.ParentID,
			&reply.Content, &reply.CreatedAt,
			&reply.User.Username, &reply.User.ProfilePicture)
		if err != nil {
			return nil, err
		}
		reply.User.ID = reply.UserID
		replies = append(replies, reply)
	}
	
	return replies, nil
}

func (r *Repository) GetFeed(userID int64, limit, offset int) ([]Post, int, error) {
	// Get total count
	var total int
	countQuery := `
		SELECT COUNT(DISTINCT p.id)
		FROM posts p
		JOIN follows f ON p.user_id = f.following_id
		WHERE f.follower_id = $1`
	
	err := r.db.QueryRow(countQuery, userID).Scan(&total)
	if err != nil {
		return []Post{}, 0, nil // Return empty instead of error
	}
	
	// Get feed posts with COALESCE for all nullable fields
	query := `
		SELECT 
			p.id, 
			p.user_id, 
			p.caption, 
			COALESCE(p.location, '') as location,  -- Handle NULL location
			p.visibility,
			p.created_at, 
			p.updated_at,
			u.username, 
			COALESCE(u.profile_picture, '') as profile_picture,  -- Handle NULL profile_picture
			COALESCE(COUNT(DISTINCT l.user_id), 0) as likes_count,
			COALESCE(COUNT(DISTINCT c.id), 0) as comments_count,
			EXISTS(SELECT 1 FROM post_likes WHERE post_id = p.id AND user_id = $1) as is_liked
		FROM posts p
		JOIN users u ON p.user_id = u.id
		JOIN follows f ON p.user_id = f.following_id
		LEFT JOIN post_likes l ON p.id = l.post_id
		LEFT JOIN comments c ON p.id = c.post_id
		WHERE f.follower_id = $1
		GROUP BY p.id, u.id, u.username, u.profile_picture, p.location
		ORDER BY p.created_at DESC
		LIMIT $2 OFFSET $3`
	
	rows, err := r.db.Query(query, userID, limit, offset)
	if err != nil {
		return []Post{}, 0, nil // Return empty instead of error
	}
	defer rows.Close()
	
	var posts []Post
	for rows.Next() {
		post := Post{User: &UserInfo{}}
		var locationStr string // Temporary string for location
		
		err := rows.Scan(
			&post.ID,
			&post.UserID,
			&post.Caption,
			&locationStr,  // Scan as string first
			&post.Visibility,
			&post.CreatedAt,
			&post.UpdatedAt,
			&post.User.Username,
			&post.User.ProfilePicture,
			&post.LikesCount,
			&post.CommentsCount,
			&post.IsLiked,
		)
		if err != nil {
			continue // Skip problematic posts instead of failing entirely
		}
		
		// Convert location string to sql.NullString
		if locationStr != "" {
			post.Location = sql.NullString{String: locationStr, Valid: true}
		} else {
			post.Location = sql.NullString{String: "", Valid: false}
		}
		
		post.User.ID = post.UserID
		
		// Get media (don't fail if media fetch fails)
		media, _ := r.GetPostMedia(post.ID)
		post.Media = media
		
		posts = append(posts, post)
	}
	
	return posts, total, nil
}

func (r *Repository) GetExplorePosts(userID int64, limit, offset int) ([]Post, int, error) {
	// Get total count
	var total int
	countQuery := `SELECT COUNT(*) FROM posts WHERE visibility = 'public'`
	err := r.db.QueryRow(countQuery).Scan(&total)
	if err != nil {
		return []Post{}, 0, nil
	}
	
	// Get explore posts with COALESCE
	query := `
		SELECT 
			p.id,
			p.user_id,
			p.caption,
			COALESCE(p.location, '') as location,
			p.visibility,
			p.created_at,
			p.updated_at,
			u.username,
			COALESCE(u.profile_picture, '') as profile_picture,
			COALESCE(COUNT(DISTINCT l.user_id), 0) as likes_count,
			COALESCE(COUNT(DISTINCT c.id), 0) as comments_count,
			EXISTS(SELECT 1 FROM post_likes WHERE post_id = p.id AND user_id = $1) as is_liked
		FROM posts p
		JOIN users u ON p.user_id = u.id
		LEFT JOIN post_likes l ON p.id = l.post_id
		LEFT JOIN comments c ON p.id = c.post_id
		WHERE p.visibility = 'public'
		GROUP BY p.id, u.id, u.username, u.profile_picture, p.location
		ORDER BY p.created_at DESC
		LIMIT $2 OFFSET $3`
	
	rows, err := r.db.Query(query, userID, limit, offset)
	if err != nil {
		return []Post{}, 0, nil
	}
	defer rows.Close()
	
	var posts []Post
	for rows.Next() {
		post := Post{User: &UserInfo{}}
		var locationStr string
		
		err := rows.Scan(
			&post.ID,
			&post.UserID,
			&post.Caption,
			&locationStr,
			&post.Visibility,
			&post.CreatedAt,
			&post.UpdatedAt,
			&post.User.Username,
			&post.User.ProfilePicture,
			&post.LikesCount,
			&post.CommentsCount,
			&post.IsLiked,
		)
		if err != nil {
			continue
		}
		
		// Convert location
		if locationStr != "" {
			post.Location = sql.NullString{String: locationStr, Valid: true}
		} else {
			post.Location = sql.NullString{String: "", Valid: false}
		}
		
		post.User.ID = post.UserID
		
		// Get media
		media, _ := r.GetPostMedia(post.ID)
		post.Media = media
		
		posts = append(posts, post)
	}
	
	return posts, total, nil
}

func (r *Repository) GetUserPosts(userID, requestingUserID int64, limit, offset int) ([]Post, int, error) {
	// Get total count
	var total int
	countQuery := `SELECT COUNT(*) FROM posts WHERE user_id = $1`
	err := r.db.QueryRow(countQuery, userID).Scan(&total)
	if err != nil {
		return nil, 0, err
	}
	
	// Get user posts - FIXED
	query := `
		SELECT 
			p.id, p.user_id, p.caption, p.location, p.visibility,
			p.created_at, p.updated_at,
			u.username, 
			COALESCE(u.profile_picture, '') as profile_picture,  -- Handle NULL
			COUNT(DISTINCT l.user_id) as likes_count,
			COUNT(DISTINCT c.id) as comments_count,
			EXISTS(SELECT 1 FROM post_likes WHERE post_id = p.id AND user_id = $2) as is_liked
		FROM posts p
		JOIN users u ON p.user_id = u.id
		LEFT JOIN post_likes l ON p.id = l.post_id
		LEFT JOIN comments c ON p.id = c.post_id
		WHERE p.user_id = $1
		GROUP BY p.id, u.id, u.username, u.profile_picture
		ORDER BY p.created_at DESC
		LIMIT $3 OFFSET $4`
	
	rows, err := r.db.Query(query, userID, requestingUserID, limit, offset)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()
	
	var posts []Post
	for rows.Next() {
		post := Post{User: &UserInfo{}}
		err := rows.Scan(
			&post.ID, &post.UserID, &post.Caption, &post.Location, &post.Visibility,
			&post.CreatedAt, &post.UpdatedAt,
			&post.User.Username, &post.User.ProfilePicture,
			&post.LikesCount, &post.CommentsCount, &post.IsLiked,
		)
		if err != nil {
			return nil, 0, err
		}
		post.User.ID = post.UserID
		
		// Get media for each post
		media, err := r.GetPostMedia(post.ID)
		if err != nil {
			return nil, 0, err
		}
		post.Media = media
		
		posts = append(posts, post)
	}
	
	return posts, total, nil
}

func (r *Repository) scanPosts(query string, args ...interface{}) ([]Post, error) {
	rows, err := r.db.Query(query, args...)
	if err != nil {
		return nil, fmt.Errorf("query failed: %w", err)
	}
	defer rows.Close()
	
	var posts []Post
	for rows.Next() {
		post := Post{User: &UserInfo{}}
		
		// Use sql.NullString for nullable fields
		var profilePic sql.NullString
		
		err := rows.Scan(
			&post.ID, 
			&post.UserID, 
			&post.Caption, 
			&post.Location,  // This is already sql.NullString in your model
			&post.Visibility,
			&post.CreatedAt, 
			&post.UpdatedAt,
			&post.User.Username, 
			&profilePic,  // Handle NULL profile_picture
			&post.LikesCount, 
			&post.CommentsCount, 
			&post.IsLiked,
		)
		if err != nil {
			// Log the actual error for debugging
			return nil, fmt.Errorf("scan failed: %w", err)
		}
		
		// Convert NULL to empty string
		if profilePic.Valid {
			post.User.ProfilePicture = profilePic.String
		} else {
			post.User.ProfilePicture = ""
		}
		
		post.User.ID = post.UserID
		
		// Get media for each post - with error handling
		media, err := r.GetPostMedia(post.ID)
		if err != nil {
			// Don't fail entirely, just skip media for this post
			post.Media = []PostMedia{}
		} else {
			post.Media = media
		}
		
		posts = append(posts, post)
	}
	
	if err = rows.Err(); err != nil {
		return nil, fmt.Errorf("rows iteration error: %w", err)
	}
	
	return posts, nil
}

func (r *Repository) IsPostOwner(postID, userID int64) (bool, error) {
	var exists bool
	query := `SELECT EXISTS(SELECT 1 FROM posts WHERE id = $1 AND user_id = $2)`
	err := r.db.QueryRow(query, postID, userID).Scan(&exists)
	return exists, err
}

func (r *Repository) scanPostsWithCoalesce(query string, args ...interface{}) ([]Post, error) {
	rows, err := r.db.Query(query, args...)
	if err != nil {
		return nil, fmt.Errorf("query failed: %w", err)
	}
	defer rows.Close()
	
	var posts []Post
	for rows.Next() {
		post := Post{User: &UserInfo{}}
		
		err := rows.Scan(
			&post.ID, 
			&post.UserID, 
			&post.Caption, 
			&post.Location,  // sql.NullString
			&post.Visibility,
			&post.CreatedAt, 
			&post.UpdatedAt,
			&post.User.Username, 
			&post.User.ProfilePicture,  // Now guaranteed to be non-NULL due to COALESCE
			&post.LikesCount, 
			&post.CommentsCount, 
			&post.IsLiked,
		)
		if err != nil {
			return nil, fmt.Errorf("scan failed at post ID %d: %w", post.ID, err)
		}
		
		post.User.ID = post.UserID
		
		// Get media for each post
		media, err := r.GetPostMedia(post.ID)
		if err != nil {
			// Don't fail, just set empty media
			post.Media = []PostMedia{}
		} else {
			post.Media = media
		}
		
		posts = append(posts, post)
	}
	
	if err = rows.Err(); err != nil {
		return nil, fmt.Errorf("rows iteration error: %w", err)
	}
	
	return posts, nil
}