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