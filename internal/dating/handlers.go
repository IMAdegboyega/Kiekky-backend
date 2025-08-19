// internal/dating/handlers.go

package dating

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

func (h *Handler) CreateDateRequest(w http.ResponseWriter, r *http.Request) {
    userID := r.Context().Value("userID").(int64)
    
    var dto CreateDateRequestDTO
    if err := json.NewDecoder(r.Body).Decode(&dto); err != nil {
        utils.RespondWithError(w, http.StatusBadRequest, "Invalid request payload")
        return
    }
    
    request, err := h.service.CreateDateRequest(r.Context(), userID, &dto)
    if err != nil {
        if err == ErrAlreadyRequested {
            utils.RespondWithError(w, http.StatusConflict, err.Error())
            return
        }
        utils.RespondWithError(w, http.StatusInternalServerError, "Failed to create date request")
        return
    }
    
    utils.RespondWithJSON(w, http.StatusCreated, request)
}

func (h *Handler) GetDateRequests(w http.ResponseWriter, r *http.Request) {
    userID := r.Context().Value("userID").(int64)
    requestType := r.URL.Query().Get("type")
    
    if requestType == "" {
        requestType = "all"
    }
    
    requests, err := h.service.GetDateRequests(r.Context(), userID, requestType)
    if err != nil {
        utils.RespondWithError(w, http.StatusInternalServerError, "Failed to get date requests")
        return
    }
    
    utils.RespondWithJSON(w, http.StatusOK, requests)
}

func (h *Handler) RespondToRequest(w http.ResponseWriter, r *http.Request) {
    userID := r.Context().Value("userID").(int64)
    
    vars := mux.Vars(r)
    requestID, err := strconv.ParseInt(vars["id"], 10, 64)
    if err != nil {
        utils.RespondWithError(w, http.StatusBadRequest, "Invalid request ID")
        return
    }
    
    var dto RespondDateRequestDTO
    if err := json.NewDecoder(r.Body).Decode(&dto); err != nil {
        utils.RespondWithError(w, http.StatusBadRequest, "Invalid request payload")
        return
    }
    
    request, err := h.service.RespondToDateRequest(r.Context(), requestID, userID, &dto)
    if err != nil {
        if err == ErrUnauthorized {
            utils.RespondWithError(w, http.StatusForbidden, err.Error())
            return
        }
        utils.RespondWithError(w, http.StatusInternalServerError, "Failed to respond to request")
        return
    }
    
    utils.RespondWithJSON(w, http.StatusOK, request)
}

func (h *Handler) GetHotpicks(w http.ResponseWriter, r *http.Request) {
    userID := r.Context().Value("userID").(int64)
    
    params := &GetHotpicksParams{
        Limit:         10,
        ExcludeViewed: true,
    }
    
    if limit := r.URL.Query().Get("limit"); limit != "" {
        if l, err := strconv.Atoi(limit); err == nil {
            params.Limit = l
        }
    }
    
    hotpicks, err := h.service.GetHotpicks(r.Context(), userID, params)
    if err != nil {
        utils.RespondWithError(w, http.StatusInternalServerError, "Failed to get hotpicks")
        return
    }
    
    utils.RespondWithJSON(w, http.StatusOK, hotpicks)
}

func (h *Handler) GetMatches(w http.ResponseWriter, r *http.Request) {
    userID := r.Context().Value("userID").(int64)
    
    active := true
    if activeStr := r.URL.Query().Get("active"); activeStr == "false" {
        active = false
    }
    
    matches, err := h.service.GetMatches(r.Context(), userID, active)
    if err != nil {
        utils.RespondWithError(w, http.StatusInternalServerError, "Failed to get matches")
        return
    }
    
    utils.RespondWithJSON(w, http.StatusOK, matches)
}

func (h *Handler) CancelRequest(w http.ResponseWriter, r *http.Request) {
    userID := r.Context().Value("userID").(int64)
    
    vars := mux.Vars(r)
    requestID, err := strconv.ParseInt(vars["id"], 10, 64)
    if err != nil {
        utils.RespondWithError(w, http.StatusBadRequest, "Invalid request ID")
        return
    }
    
    err = h.service.CancelDateRequest(r.Context(), requestID, userID)
    if err != nil {
        if err == ErrUnauthorized {
            utils.RespondWithError(w, http.StatusForbidden, err.Error())
            return
        }
        utils.RespondWithError(w, http.StatusInternalServerError, "Failed to cancel request")
        return
    }
    
    utils.RespondWithJSON(w, http.StatusOK, map[string]string{"message": "Request cancelled"})
}

func (h *Handler) GetUpcomingDates(w http.ResponseWriter, r *http.Request) {
    userID := r.Context().Value("userID").(int64)
    
    dates, err := h.service.GetUpcomingDates(r.Context(), userID)
    if err != nil {
        utils.RespondWithError(w, http.StatusInternalServerError, "Failed to get upcoming dates")
        return
    }
    
    utils.RespondWithJSON(w, http.StatusOK, dates)
}

func (h *Handler) Unmatch(w http.ResponseWriter, r *http.Request) {
    userID := r.Context().Value("userID").(int64)
    
    vars := mux.Vars(r)
    matchID, err := strconv.ParseInt(vars["id"], 10, 64)
    if err != nil {
        utils.RespondWithError(w, http.StatusBadRequest, "Invalid match ID")
        return
    }
    
    err = h.service.UnmatchUser(r.Context(), matchID, userID)
    if err != nil {
        if err == ErrUnauthorized {
            utils.RespondWithError(w, http.StatusForbidden, err.Error())
            return
        }
        utils.RespondWithError(w, http.StatusInternalServerError, "Failed to unmatch")
        return
    }
    
    utils.RespondWithJSON(w, http.StatusOK, map[string]string{"message": "Unmatched successfully"})
}

func (h *Handler) CheckMatch(w http.ResponseWriter, r *http.Request) {
    userID := r.Context().Value("userID").(int64)
    
    vars := mux.Vars(r)
    otherUserID, err := strconv.ParseInt(vars["userId"], 10, 64)
    if err != nil {
        utils.RespondWithError(w, http.StatusBadRequest, "Invalid user ID")
        return
    }
    
    isMatched, err := h.service.IsMatched(r.Context(), userID, otherUserID)
    if err != nil {
        utils.RespondWithError(w, http.StatusInternalServerError, "Failed to check match status")
        return
    }
    
    utils.RespondWithJSON(w, http.StatusOK, map[string]bool{"is_matched": isMatched})
}

func (h *Handler) RecordAction(w http.ResponseWriter, r *http.Request) {
    vars := mux.Vars(r)
    hotpickID, err := strconv.ParseInt(vars["id"], 10, 64)
    if err != nil {
        utils.RespondWithError(w, http.StatusBadRequest, "Invalid hotpick ID")
        return
    }
    
    var dto struct {
        Action string `json:"action"`
    }
    
    if err := json.NewDecoder(r.Body).Decode(&dto); err != nil {
        utils.RespondWithError(w, http.StatusBadRequest, "Invalid request payload")
        return
    }
    
    err = h.service.RecordHotpickAction(r.Context(), hotpickID, dto.Action)
    if err != nil {
        utils.RespondWithError(w, http.StatusInternalServerError, "Failed to record action")
        return
    }
    
    utils.RespondWithJSON(w, http.StatusOK, map[string]string{"message": "Action recorded"})
}

func (h *Handler) GenerateHotpicks(w http.ResponseWriter, r *http.Request) {
    userID := r.Context().Value("userID").(int64)
    
    err := h.service.GenerateHotpicks(r.Context(), userID)
    if err != nil {
        utils.RespondWithError(w, http.StatusInternalServerError, "Failed to generate hotpicks")
        return
    }
    
    utils.RespondWithJSON(w, http.StatusOK, map[string]string{"message": "Hotpicks generated"})
}

func (h *Handler) GetCompatibility(w http.ResponseWriter, r *http.Request) {
    userID := r.Context().Value("userID").(int64)
    
    vars := mux.Vars(r)
    otherUserID, err := strconv.ParseInt(vars["userId"], 10, 64)
    if err != nil {
        utils.RespondWithError(w, http.StatusBadRequest, "Invalid user ID")
        return
    }
    
    score, factors, err := h.service.CalculateCompatibility(r.Context(), userID, otherUserID)
    if err != nil {
        utils.RespondWithError(w, http.StatusInternalServerError, "Failed to calculate compatibility")
        return
    }
    
    response := map[string]interface{}{
        "score":   score,
        "factors": factors,
    }
    
    utils.RespondWithJSON(w, http.StatusOK, response)
}

func (h *Handler) DiscoverMatches(w http.ResponseWriter, r *http.Request) {
    userID := r.Context().Value("userID").(int64)
    
    // Parse query parameters for filters
    filters := &MatchFilters{
        MinAge:    18,
        MaxAge:    100,
        MaxDistance: 100,
        Limit:     20,
    }
    
    if minAge := r.URL.Query().Get("min_age"); minAge != "" {
        if age, err := strconv.Atoi(minAge); err == nil {
            filters.MinAge = age
        }
    }
    
    if maxAge := r.URL.Query().Get("max_age"); maxAge != "" {
        if age, err := strconv.Atoi(maxAge); err == nil {
            filters.MaxAge = age
        }
    }
    
    matches, err := h.service.FindPotentialMatches(r.Context(), userID, filters)
    if err != nil {
        utils.RespondWithError(w, http.StatusInternalServerError, "Failed to discover matches")
        return
    }
    
    utils.RespondWithJSON(w, http.StatusOK, matches)
}