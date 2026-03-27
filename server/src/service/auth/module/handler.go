package module

import (
	"encoding/json"
	"errors"
	"net/http"

	"backend/deps/xlog"
)

type Handler struct {
	service *AuthService
}

type apiResponse struct {
	Code    string `json:"code"`
	Message string `json:"message,omitempty"`
	Data    any    `json:"data,omitempty"`
}

func NewHandler(service *AuthService) *Handler {
	return &Handler{service: service}
}

func (h *Handler) RegisterHTTP(mux *http.ServeMux) {
	mux.HandleFunc("/auth/login", h.handleLogin)
	mux.HandleFunc("/auth/tickets/consume", h.handleConsumeTicket)
	mux.HandleFunc("/auth/tokens/consume", h.handleConsumeToken)
	mux.HandleFunc("/auth/sessions/validate", h.handleValidateSession)
}

func (h *Handler) handleLogin(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeAPIError(w, http.StatusMethodNotAllowed, "method_not_allowed", "only POST is supported")
		return
	}

	var input LoginInput
	if err := decodeJSONBody(r, &input); err != nil {
		writeServiceError(w, err)
		return
	}

	result, err := h.service.Login(r.Context(), input)
	if err != nil {
		writeServiceError(w, err)
		return
	}
	writeAPISuccess(w, result)
}

func (h *Handler) handleConsumeTicket(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeAPIError(w, http.StatusMethodNotAllowed, "method_not_allowed", "only POST is supported")
		return
	}

	var input ConsumeTicketInput
	if err := decodeJSONBody(r, &input); err != nil {
		writeServiceError(w, err)
		return
	}

	session, err := h.service.ConsumeLoginTicket(r.Context(), input)
	if err != nil {
		writeServiceError(w, err)
		return
	}
	writeAPISuccess(w, session)
}

func (h *Handler) handleConsumeToken(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeAPIError(w, http.StatusMethodNotAllowed, "method_not_allowed", "only POST is supported")
		return
	}

	var input ConsumeConnectTokenInput
	if err := decodeJSONBody(r, &input); err != nil {
		writeServiceError(w, err)
		return
	}

	session, err := h.service.ConsumeConnectToken(r.Context(), input)
	if err != nil {
		writeServiceError(w, err)
		return
	}
	writeAPISuccess(w, session)
}

func (h *Handler) handleValidateSession(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeAPIError(w, http.StatusMethodNotAllowed, "method_not_allowed", "only POST is supported")
		return
	}

	var input ValidateSessionInput
	if err := decodeJSONBody(r, &input); err != nil {
		writeServiceError(w, err)
		return
	}

	session, err := h.service.ValidateSession(r.Context(), input)
	if err != nil {
		writeServiceError(w, err)
		return
	}
	writeAPISuccess(w, session)
}

func decodeJSONBody(r *http.Request, dst any) error {
	defer r.Body.Close()

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
	case errors.Is(err, ErrTicketNotFound):
		writeAPIError(w, http.StatusUnauthorized, "ticket_not_found", err.Error())
	case errors.Is(err, ErrConnectTokenNotFound):
		writeAPIError(w, http.StatusUnauthorized, "connect_token_not_found", err.Error())
	case errors.Is(err, ErrSessionNotFound):
		writeAPIError(w, http.StatusUnauthorized, "session_not_found", err.Error())
	default:
		if !isPublicError(err) {
			xlog.Errorf("auth handler failed: %v", err)
		}
		writeAPIError(w, http.StatusInternalServerError, "internal_error", "internal server error")
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
		xlog.Errorf("write auth api response: %v", err)
	}
}
