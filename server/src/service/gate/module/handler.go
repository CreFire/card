package module

import (
	"encoding/json"
	"errors"
	"net/http"
	"strings"

	"backend/deps/xlog"
)

type Handler struct {
	service *GateService
}

type apiResponse struct {
	Code    string `json:"code"`
	Message string `json:"message,omitempty"`
	Data    any    `json:"data,omitempty"`
}

func NewHandler(service *GateService) *Handler {
	return &Handler{service: service}
}

func (h *Handler) RegisterHTTP(mux *http.ServeMux) {
	mux.HandleFunc("/gate/stats", h.handleStats)
	mux.HandleFunc("/gate/gamers", h.handleGamers)
	mux.HandleFunc("/gate/connections/open", h.handleOpenConnection)
	mux.HandleFunc("/gate/login/token", h.handleLoginByToken)
	mux.HandleFunc("/gate/login/ticket", h.handleLoginByTicket)
	mux.HandleFunc("/gate/login/session", h.handleLoginBySession)
	mux.HandleFunc("/gate/logout", h.handleLogout)
	mux.HandleFunc("/gate/connections/", h.handleGetConnection)
}

func (h *Handler) handleStats(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeAPIError(w, http.StatusMethodNotAllowed, "method_not_allowed", "only GET is supported")
		return
	}
	writeAPISuccess(w, h.service.Stats(r.Context()))
}

func (h *Handler) handleGamers(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeAPIError(w, http.StatusMethodNotAllowed, "method_not_allowed", "only GET is supported")
		return
	}
	writeAPISuccess(w, h.service.ListGamers(r.Context()))
}

func (h *Handler) handleOpenConnection(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeAPIError(w, http.StatusMethodNotAllowed, "method_not_allowed", "only POST is supported")
		return
	}

	var input OpenConnectionInput
	if err := decodeJSONBody(r, &input); err != nil {
		writeServiceError(w, err)
		return
	}

	gamer, err := h.service.OpenConnection(r.Context(), input)
	if err != nil {
		writeServiceError(w, err)
		return
	}
	writeAPISuccess(w, gamer)
}

func (h *Handler) handleLoginByTicket(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeAPIError(w, http.StatusMethodNotAllowed, "method_not_allowed", "only POST is supported")
		return
	}

	var input AttachTicketInput
	if err := decodeJSONBody(r, &input); err != nil {
		writeServiceError(w, err)
		return
	}

	gamer, err := h.service.LoginByTicket(r.Context(), input)
	if err != nil {
		writeServiceError(w, err)
		return
	}
	writeAPISuccess(w, gamer)
}

func (h *Handler) handleLoginByToken(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeAPIError(w, http.StatusMethodNotAllowed, "method_not_allowed", "only POST is supported")
		return
	}

	var input AttachTokenInput
	if err := decodeJSONBody(r, &input); err != nil {
		writeServiceError(w, err)
		return
	}

	gamer, err := h.service.LoginByToken(r.Context(), input)
	if err != nil {
		writeServiceError(w, err)
		return
	}
	writeAPISuccess(w, gamer)
}

func (h *Handler) handleLoginBySession(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeAPIError(w, http.StatusMethodNotAllowed, "method_not_allowed", "only POST is supported")
		return
	}

	var input AttachSessionInput
	if err := decodeJSONBody(r, &input); err != nil {
		writeServiceError(w, err)
		return
	}

	gamer, err := h.service.LoginBySession(r.Context(), input)
	if err != nil {
		writeServiceError(w, err)
		return
	}
	writeAPISuccess(w, gamer)
}

func (h *Handler) handleLogout(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeAPIError(w, http.StatusMethodNotAllowed, "method_not_allowed", "only POST is supported")
		return
	}

	var input DisconnectInput
	if err := decodeJSONBody(r, &input); err != nil {
		writeServiceError(w, err)
		return
	}

	if err := h.service.Logout(r.Context(), input); err != nil {
		writeServiceError(w, err)
		return
	}
	writeAPISuccess(w, map[string]bool{"ok": true})
}

func (h *Handler) handleGetConnection(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeAPIError(w, http.StatusMethodNotAllowed, "method_not_allowed", "only GET is supported")
		return
	}

	connID := strings.TrimPrefix(r.URL.Path, "/gate/connections/")
	gamer, err := h.service.GetGamer(r.Context(), connID)
	if err != nil {
		writeServiceError(w, err)
		return
	}
	writeAPISuccess(w, gamer)
}

func decodeJSONBody(r *http.Request, dst any) error {
	defer r.Body.Close()

	if r.Body == nil || r.ContentLength == 0 {
		return nil
	}

	decoder := json.NewDecoder(r.Body)
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(dst); err != nil {
		return ErrInvalidArgument
	}
	return nil
}

func writeAPISuccess(w http.ResponseWriter, data any) {
	writeAPIResponse(w, http.StatusOK, apiResponse{
		Code: "ok",
		Data: data,
	})
}

func writeServiceError(w http.ResponseWriter, err error) {
	switch {
	case errors.Is(err, ErrInvalidArgument):
		writeAPIError(w, http.StatusBadRequest, "invalid_argument", err.Error())
	case errors.Is(err, ErrConnNotFound):
		writeAPIError(w, http.StatusNotFound, "connection_not_found", err.Error())
	default:
		xlog.Errorf("gate handler failed: %v", err)
		writeAPIError(w, http.StatusBadGateway, "upstream_or_internal_error", err.Error())
	}
}

func writeAPIError(w http.ResponseWriter, status int, code, message string) {
	writeAPIResponse(w, status, apiResponse{
		Code:    code,
		Message: message,
	})
}

func writeAPIResponse(w http.ResponseWriter, status int, payload apiResponse) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	if err := json.NewEncoder(w).Encode(payload); err != nil {
		xlog.Errorf("write gate api response: %v", err)
	}
}
