package stories

import (
    "encoding/json"
    "net/http"
    "strconv"
    
    "github.com/gorilla/mux"
    "github.com/imadgeboyega/kiekky-backend/internal/common/utils"
)

type Handler struct {
    service Service
}

func NewHandler(service Service) *Handler {
    return &Handler{service: service}
}

// CreateStory handles story creation
func (h *Handler) CreateStory(w http.ResponseWriter, r *http.Request) {
    userID := r.Context().Value("userID").(int64)
    
    var req CreateStoryRequest
    if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
        utils.RespondWithError(w, http.StatusBadRequest, "Invalid request payload")
        return
    }
    
    // Validate request
    if req.MediaURL == "" || req.MediaType == "" {
        utils.RespondWithError(w, http.StatusBadRequest, "Media URL and type are required")
        return
    }
    
    if req.MediaType != "image" && req.MediaType != "video" {
        utils.RespondWithError(w, http.StatusBadRequest, "Invalid media type")
        return
    }
    
    story, err := h.service.CreateStory(r.Context(), userID, &req)
    if err != nil {
        utils.RespondWithError(w, http.StatusInternalServerError, "Failed to create story")
        return
    }
    
    utils.RespondWithJSON(w, http.StatusCreated, story)
}

// GetStory retrieves a specific story
func (h *Handler) GetStory(w http.ResponseWriter, r *http.Request) {
    userID := r.Context().Value("userID").(int64)
    vars := mux.Vars(r)
    
    storyID, err := strconv.ParseInt(vars["id"], 10, 64)
    if err != nil {
        utils.RespondWithError(w, http.StatusBadRequest, "Invalid story ID")
        return
    }
    
    story, err := h.service.GetStory(r.Context(), storyID, userID)
    if err != nil {
        if err == ErrStoryNotFound {
            utils.RespondWithError(w, http.StatusNotFound, "Story not found")
        } else if err == ErrStoryExpired {
            utils.RespondWithError(w, http.StatusGone, "Story has expired")
        } else {
            utils.RespondWithError(w, http.StatusInternalServerError, "Failed to get story")
        }
        return
    }
    
    utils.RespondWithJSON(w, http.StatusOK, story)
}

// GetActiveStories retrieves active stories feed
func (h *Handler) GetActiveStories(w http.ResponseWriter, r *http.Request) {
    userID := r.Context().Value("userID").(int64)
    
    // Parse query parameters
    limit, _ := strconv.Atoi(r.URL.Query().Get("limit"))
    offset, _ := strconv.Atoi(r.URL.Query().Get("offset"))
    
    if limit == 0 {
        limit = 20
    }
    
    response, err := h.service.GetActiveStories(r.Context(), userID, limit, offset)
    if err != nil {
        utils.RespondWithError(w, http.StatusInternalServerError, "Failed to get stories")
        return
    }
    
    utils.RespondWithJSON(w, http.StatusOK, response)
}

// GetUserStories retrieves all stories for a specific user
func (h *Handler) GetUserStories(w http.ResponseWriter, r *http.Request) {
    viewerID := r.Context().Value("userID").(int64)
    vars := mux.Vars(r)
    
    userID, err := strconv.ParseInt(vars["userId"], 10, 64)
    if err != nil {
        utils.RespondWithError(w, http.StatusBadRequest, "Invalid user ID")
        return
    }
    
    stories, err := h.service.GetUserStories(r.Context(), userID, viewerID)
    if err != nil {
        utils.RespondWithError(w, http.StatusInternalServerError, "Failed to get user stories")
        return
    }
    
    utils.RespondWithJSON(w, http.StatusOK, stories)
}

// DeleteStory handles story deletion
func (h *Handler) DeleteStory(w http.ResponseWriter, r *http.Request) {
    userID := r.Context().Value("userID").(int64)
    vars := mux.Vars(r)
    
    storyID, err := strconv.ParseInt(vars["id"], 10, 64)
    if err != nil {
        utils.RespondWithError(w, http.StatusBadRequest, "Invalid story ID")
        return
    }
    
    if err := h.service.DeleteStory(r.Context(), storyID, userID); err != nil {
        if err == ErrStoryNotFound {
            utils.RespondWithError(w, http.StatusNotFound, "Story not found")
        } else if err == ErrUnauthorized {
            utils.RespondWithError(w, http.StatusForbidden, "Unauthorized")
        } else {
            utils.RespondWithError(w, http.StatusInternalServerError, "Failed to delete story")
        }
        return
    }
    
    utils.RespondWithJSON(w, http.StatusOK, map[string]string{"message": "Story deleted successfully"})
}

// ViewStory marks a story as viewed
func (h *Handler) ViewStory(w http.ResponseWriter, r *http.Request) {
    userID := r.Context().Value("userID").(int64)
    vars := mux.Vars(r)
    
    storyID, err := strconv.ParseInt(vars["id"], 10, 64)
    if err != nil {
        utils.RespondWithError(w, http.StatusBadRequest, "Invalid story ID")
        return
    }
    
    if err := h.service.ViewStory(r.Context(), storyID, userID); err != nil {
        if err == ErrStoryNotFound {
            utils.RespondWithError(w, http.StatusNotFound, "Story not found")
        } else if err == ErrStoryExpired {
            utils.RespondWithError(w, http.StatusGone, "Story has expired")
        } else {
            utils.RespondWithError(w, http.StatusInternalServerError, "Failed to record view")
        }
        return
    }
    
    utils.RespondWithJSON(w, http.StatusOK, map[string]string{"message": "View recorded"})
}

// ReplyToStory handles story replies
func (h *Handler) ReplyToStory(w http.ResponseWriter, r *http.Request) {
    userID := r.Context().Value("userID").(int64)
    vars := mux.Vars(r)
    
    storyID, err := strconv.ParseInt(vars["id"], 10, 64)
    if err != nil {
        utils.RespondWithError(w, http.StatusBadRequest, "Invalid story ID")
        return
    }
    
    var req StoryReplyRequest
    if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
        utils.RespondWithError(w, http.StatusBadRequest, "Invalid request payload")
        return
    }
    
    reply, err := h.service.ReplyToStory(r.Context(), storyID, userID, &req)
    if err != nil {
        if err == ErrStoryNotFound {
            utils.RespondWithError(w, http.StatusNotFound, "Story not found")
        } else if err == ErrStoryExpired {
            utils.RespondWithError(w, http.StatusGone, "Story has expired")
        } else if err == ErrInvalidReply {
            utils.RespondWithError(w, http.StatusBadRequest, "Message or reaction is required")
        } else {
            utils.RespondWithError(w, http.StatusInternalServerError, "Failed to send reply")
        }
        return
    }
    
    utils.RespondWithJSON(w, http.StatusCreated, reply)
}

// GetStoryViews retrieves story views
func (h *Handler) GetStoryViews(w http.ResponseWriter, r *http.Request) {
    userID := r.Context().Value("userID").(int64)
    vars := mux.Vars(r)
    
    storyID, err := strconv.ParseInt(vars["id"], 10, 64)
    if err != nil {
        utils.RespondWithError(w, http.StatusBadRequest, "Invalid story ID")
        return
    }
    
    views, err := h.service.GetStoryViews(r.Context(), storyID, userID)
    if err != nil {
        if err == ErrStoryNotFound {
            utils.RespondWithError(w, http.StatusNotFound, "Story not found")
        } else if err == ErrUnauthorized {
            utils.RespondWithError(w, http.StatusForbidden, "Unauthorized")
        } else {
            utils.RespondWithError(w, http.StatusInternalServerError, "Failed to get views")
        }
        return
    }
    
    utils.RespondWithJSON(w, http.StatusOK, views)
}

// GetStoryReplies retrieves story replies
func (h *Handler) GetStoryReplies(w http.ResponseWriter, r *http.Request) {
    userID := r.Context().Value("userID").(int64)
    vars := mux.Vars(r)
    
    storyID, err := strconv.ParseInt(vars["id"], 10, 64)
    if err != nil {
        utils.RespondWithError(w, http.StatusBadRequest, "Invalid story ID")
        return
    }
    
    replies, err := h.service.GetStoryReplies(r.Context(), storyID, userID)
    if err != nil {
        if err == ErrStoryNotFound {
            utils.RespondWithError(w, http.StatusNotFound, "Story not found")
        } else if err == ErrUnauthorized {
            utils.RespondWithError(w, http.StatusForbidden, "Unauthorized")
        } else {
            utils.RespondWithError(w, http.StatusInternalServerError, "Failed to get replies")
        }
        return
    }
    
    utils.RespondWithJSON(w, http.StatusOK, replies)
}

// MarkReplyAsRead marks a reply as read
func (h *Handler) MarkReplyAsRead(w http.ResponseWriter, r *http.Request) {
    userID := r.Context().Value("userID").(int64)
    vars := mux.Vars(r)
    
    replyID, err := strconv.ParseInt(vars["replyId"], 10, 64)
    if err != nil {
        utils.RespondWithError(w, http.StatusBadRequest, "Invalid reply ID")
        return
    }
    
    if err := h.service.MarkReplyAsRead(r.Context(), replyID, userID); err != nil {
        if err == ErrUnauthorized {
            utils.RespondWithError(w, http.StatusForbidden, "Unauthorized")
        } else {
            utils.RespondWithError(w, http.StatusInternalServerError, "Failed to mark as read")
        }
        return
    }
    
    utils.RespondWithJSON(w, http.StatusOK, map[string]string{"message": "Reply marked as read"})
}

// CreateHighlight creates a story highlight
func (h *Handler) CreateHighlight(w http.ResponseWriter, r *http.Request) {
    userID := r.Context().Value("userID").(int64)
    
    var req CreateHighlightRequest
    if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
        utils.RespondWithError(w, http.StatusBadRequest, "Invalid request payload")
        return
    }
    
    if req.Title == "" || len(req.StoryIDs) == 0 {
        utils.RespondWithError(w, http.StatusBadRequest, "Title and story IDs are required")
        return
    }
    
    highlight, err := h.service.CreateHighlight(r.Context(), userID, &req)
    if err != nil {
        if err == ErrUnauthorized {
            utils.RespondWithError(w, http.StatusForbidden, "Unauthorized")
        } else {
            utils.RespondWithError(w, http.StatusInternalServerError, "Failed to create highlight")
        }
        return
    }
    
    utils.RespondWithJSON(w, http.StatusCreated, highlight)
}

// GetHighlights retrieves user highlights
func (h *Handler) GetHighlights(w http.ResponseWriter, r *http.Request) {
    userID := r.Context().Value("userID").(int64)
    
    // Check if specific user ID is provided
    if userIDStr := r.URL.Query().Get("user_id"); userIDStr != "" {
        var err error
        userID, err = strconv.ParseInt(userIDStr, 10, 64)
        if err != nil {
            utils.RespondWithError(w, http.StatusBadRequest, "Invalid user ID")
            return
        }
    }
    
    highlights, err := h.service.GetUserHighlights(r.Context(), userID)
    if err != nil {
        utils.RespondWithError(w, http.StatusInternalServerError, "Failed to get highlights")
        return
    }
    
    utils.RespondWithJSON(w, http.StatusOK, highlights)
}

// DeleteHighlight deletes a highlight
func (h *Handler) DeleteHighlight(w http.ResponseWriter, r *http.Request) {
    userID := r.Context().Value("userID").(int64)
    vars := mux.Vars(r)
    
    highlightID, err := strconv.ParseInt(vars["id"], 10, 64)
    if err != nil {
        utils.RespondWithError(w, http.StatusBadRequest, "Invalid highlight ID")
        return
    }
    
    if err := h.service.DeleteHighlight(r.Context(), highlightID, userID); err != nil {
        if err == ErrUnauthorized {
            utils.RespondWithError(w, http.StatusForbidden, "Unauthorized")
        } else {
            utils.RespondWithError(w, http.StatusInternalServerError, "Failed to delete highlight")
        }
        return
    }
    
    utils.RespondWithJSON(w, http.StatusOK, map[string]string{"message": "Highlight deleted successfully"})
}

// UploadMedia handles media file upload for stories
func (h *Handler) UploadMedia(w http.ResponseWriter, r *http.Request) {
    userID := r.Context().Value("userID").(int64)
    
    // Parse multipart form
    err := r.ParseMultipartForm(100 << 20) // 100MB max
    if err != nil {
        utils.RespondWithError(w, http.StatusBadRequest, "Failed to parse form")
        return
    }
    
    file, header, err := r.FormFile("media")
    if err != nil {
        utils.RespondWithError(w, http.StatusBadRequest, "Failed to get file")
        return
    }
    defer file.Close()
    
    url, err := h.service.UploadStoryMedia(r.Context(), userID, file, header)
    if err != nil {
        if err == ErrInvalidMedia {
            utils.RespondWithError(w, http.StatusBadRequest, "Invalid media file")
        } else {
            utils.RespondWithError(w, http.StatusInternalServerError, "Failed to upload media")
        }
        return
    }
    
    utils.RespondWithJSON(w, http.StatusOK, map[string]string{
        "media_url": url,
        "message":   "Media uploaded successfully",
    })
}