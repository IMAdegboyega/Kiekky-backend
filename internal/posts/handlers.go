// internal/posts/handlers.go
package posts

import (
	"encoding/json"
	"net/http"
	"strconv"
	"strings"
	
	"github.com/gorilla/mux"
	"github.com/imadgeboyega/kiekky-backend/internal/common/utils"
)

type Handler struct {
	service *Service
}

func NewHandler(service *Service) *Handler {
	return &Handler{service: service}
}

func (h *Handler) CreatePost(w http.ResponseWriter, r *http.Request) {
	userID := r.Context().Value("userID").(int64)
	
	// Parse multipart form for file uploads
	err := r.ParseMultipartForm(10 << 20) // 10 MB max
	if err != nil && err != http.ErrNotMultipart {
		utils.ErrorResponse(w, "Failed to parse form", http.StatusBadRequest)
		return
	}
	
	var req CreatePostRequest
	
	// Handle JSON or form data
	contentType := r.Header.Get("Content-Type")
	if strings.Contains(contentType, "application/json") {
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			utils.ErrorResponse(w, "Invalid request body", http.StatusBadRequest)
			return
		}
	} else {
		// Handle form data
		req.Caption = r.FormValue("caption")
		req.Location = r.FormValue("location")
		req.Visibility = r.FormValue("visibility")
		
		// Handle file uploads
		if r.MultipartForm != nil && r.MultipartForm.File != nil {
			files := r.MultipartForm.File["media"]
			for _, fileHeader := range files {
				file, err := fileHeader.Open()
				if err != nil {
					continue
				}
				defer file.Close()
				
				// Upload file and get URL
				url, err := h.service.UploadMedia(file, fileHeader)
				if err != nil {
					utils.ErrorResponse(w, "Failed to upload media", http.StatusInternalServerError)
					return
				}
				req.MediaURLs = append(req.MediaURLs, url)
			}
		}
	}
	
	post, err := h.service.CreatePost(userID, &req)
	if err != nil {
		utils.ErrorResponse(w, err.Error(), http.StatusBadRequest)
		return
	}
	
	utils.SuccessResponse(w, post, http.StatusCreated)
}

func (h *Handler) GetPost(w http.ResponseWriter, r *http.Request) {
	userID := r.Context().Value("userID").(int64)
	
	vars := mux.Vars(r)
	postID, err := strconv.ParseInt(vars["id"], 10, 64)
	if err != nil {
		utils.ErrorResponse(w, "Invalid post ID", http.StatusBadRequest)
		return
	}
	
	post, err := h.service.GetPost(postID, userID)
	if err != nil {
		if err.Error() == "sql: no rows in result set" {
			utils.ErrorResponse(w, "Post not found", http.StatusNotFound)
		} else {
			utils.ErrorResponse(w, "Failed to get post", http.StatusInternalServerError)
		}
		return
	}
	
	utils.SuccessResponse(w, post, http.StatusOK)
}

func (h *Handler) UpdatePost(w http.ResponseWriter, r *http.Request) {
	userID := r.Context().Value("userID").(int64)
	
	vars := mux.Vars(r)
	postID, err := strconv.ParseInt(vars["id"], 10, 64)
	if err != nil {
		utils.ErrorResponse(w, "Invalid post ID", http.StatusBadRequest)
		return
	}
	
	var req UpdatePostRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		utils.ErrorResponse(w, "Invalid request body", http.StatusBadRequest)
		return
	}
	
	post, err := h.service.UpdatePost(postID, userID, &req)
	if err != nil {
		if err.Error() == "unauthorized to update this post" {
			utils.ErrorResponse(w, err.Error(), http.StatusForbidden)
		} else {
			utils.ErrorResponse(w, "Failed to update post", http.StatusInternalServerError)
		}
		return
	}
	
	utils.SuccessResponse(w, post, http.StatusOK)
}

func (h *Handler) DeletePost(w http.ResponseWriter, r *http.Request) {
	userID := r.Context().Value("userID").(int64)
	
	vars := mux.Vars(r)
	postID, err := strconv.ParseInt(vars["id"], 10, 64)
	if err != nil {
		utils.ErrorResponse(w, "Invalid post ID", http.StatusBadRequest)
		return
	}
	
	err = h.service.DeletePost(postID, userID)
	if err != nil {
		if err.Error() == "unauthorized to delete this post" {
			utils.ErrorResponse(w, err.Error(), http.StatusForbidden)
		} else {
			utils.ErrorResponse(w, "Failed to delete post", http.StatusInternalServerError)
		}
		return
	}
	
	utils.SuccessResponse(w, map[string]string{"message": "Post deleted successfully"}, http.StatusOK)
}

func (h *Handler) LikePost(w http.ResponseWriter, r *http.Request) {
	userID := r.Context().Value("userID").(int64)
	
	vars := mux.Vars(r)
	postID, err := strconv.ParseInt(vars["id"], 10, 64)
	if err != nil {
		utils.ErrorResponse(w, "Invalid post ID", http.StatusBadRequest)
		return
	}
	
	isLiked, err := h.service.ToggleLike(postID, userID)
	if err != nil {
		utils.ErrorResponse(w, "Failed to like post", http.StatusInternalServerError)
		return
	}
	
	response := map[string]interface{}{
		"is_liked": isLiked,
		"message":  "Success",
	}
	
	utils.SuccessResponse(w, response, http.StatusOK)
}

func (h *Handler) UnlikePost(w http.ResponseWriter, r *http.Request) {
	userID := r.Context().Value("userID").(int64)
	
	vars := mux.Vars(r)
	postID, err := strconv.ParseInt(vars["id"], 10, 64)
	if err != nil {
		utils.ErrorResponse(w, "Invalid post ID", http.StatusBadRequest)
		return
	}
	
	_, err = h.service.ToggleLike(postID, userID)
	if err != nil {
		utils.ErrorResponse(w, "Failed to unlike post", http.StatusInternalServerError)
		return
	}
	
	response := map[string]interface{}{
		"is_liked": false,
		"message":  "Success",
	}
	
	utils.SuccessResponse(w, response, http.StatusOK)
}

func (h *Handler) GetPostLikes(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	postID, err := strconv.ParseInt(vars["id"], 10, 64)
	if err != nil {
		utils.ErrorResponse(w, "Invalid post ID", http.StatusBadRequest)
		return
	}
	
	page, limit := h.getPagination(r)
	
	likes, pagination, err := h.service.GetPostLikes(postID, page, limit)
	if err != nil {
		utils.ErrorResponse(w, "Failed to get likes", http.StatusInternalServerError)
		return
	}
	
	response := map[string]interface{}{
		"likes":      likes,
		"pagination": pagination,
	}
	
	utils.SuccessResponse(w, response, http.StatusOK)
}

func (h *Handler) AddComment(w http.ResponseWriter, r *http.Request) {
	userID := r.Context().Value("userID").(int64)
	
	vars := mux.Vars(r)
	postID, err := strconv.ParseInt(vars["id"], 10, 64)
	if err != nil {
		utils.ErrorResponse(w, "Invalid post ID", http.StatusBadRequest)
		return
	}
	
	var req CommentRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		utils.ErrorResponse(w, "Invalid request body", http.StatusBadRequest)
		return
	}
	
	comment, err := h.service.AddComment(postID, userID, &req)
	if err != nil {
		utils.ErrorResponse(w, err.Error(), http.StatusBadRequest)
		return
	}
	
	utils.SuccessResponse(w, comment, http.StatusCreated)
}

func (h *Handler) GetPostComments(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	postID, err := strconv.ParseInt(vars["id"], 10, 64)
	if err != nil {
		utils.ErrorResponse(w, "Invalid post ID", http.StatusBadRequest)
		return
	}
	
	page, limit := h.getPagination(r)
	
	comments, pagination, err := h.service.GetPostComments(postID, page, limit)
	if err != nil {
		utils.ErrorResponse(w, "Failed to get comments", http.StatusInternalServerError)
		return
	}
	
	response := map[string]interface{}{
		"comments":   comments,
		"pagination": pagination,
	}
	
	utils.SuccessResponse(w, response, http.StatusOK)
}

func (h *Handler) GetFeed(w http.ResponseWriter, r *http.Request) {
	userID := r.Context().Value("userID").(int64)
	page, limit := h.getPagination(r)
	
	feed, err := h.service.GetFeed(userID, page, limit)
	if err != nil {
		utils.ErrorResponse(w, "Failed to get feed", http.StatusInternalServerError)
		return
	}
	
	utils.SuccessResponse(w, feed, http.StatusOK)
}

func (h *Handler) GetExplorePosts(w http.ResponseWriter, r *http.Request) {
	userID := r.Context().Value("userID").(int64)
	page, limit := h.getPagination(r)
	
	explore, err := h.service.GetExplorePosts(userID, page, limit)
	if err != nil {
		utils.ErrorResponse(w, "Failed to get explore posts", http.StatusInternalServerError)
		return
	}
	
	utils.SuccessResponse(w, explore, http.StatusOK)
}

func (h *Handler) GetUserPosts(w http.ResponseWriter, r *http.Request) {
	requestingUserID := r.Context().Value("userID").(int64)
	
	vars := mux.Vars(r)
	userID, err := strconv.ParseInt(vars["id"], 10, 64)
	if err != nil {
		utils.ErrorResponse(w, "Invalid user ID", http.StatusBadRequest)
		return
	}
	
	page, limit := h.getPagination(r)
	
	posts, err := h.service.GetUserPosts(userID, requestingUserID, page, limit)
	if err != nil {
		utils.ErrorResponse(w, "Failed to get user posts", http.StatusInternalServerError)
		return
	}
	
	utils.SuccessResponse(w, posts, http.StatusOK)
}

func (h *Handler) getPagination(r *http.Request) (int, int) {
	page := 1
	limit := 20
	
	if p := r.URL.Query().Get("page"); p != "" {
		if val, err := strconv.Atoi(p); err == nil && val > 0 {
			page = val
		}
	}
	
	if l := r.URL.Query().Get("limit"); l != "" {
		if val, err := strconv.Atoi(l); err == nil && val > 0 && val <= 100 {
			limit = val
		}
	}
	
	return page, limit
}