package module

import (
	"encoding/json"
	"errors"
	"net/http"

	"backend/deps/xlog"
)

type Handler struct {
	service *RobotService
}

type apiResponse struct {
	Code    string `json:"code"`
	Message string `json:"message,omitempty"`
	Data    any    `json:"data,omitempty"`
}

func NewHandler(service *RobotService) *Handler {
	return &Handler{service: service}
}

func (h *Handler) RegisterHTTP(mux *http.ServeMux) {
	mux.HandleFunc("/robot/auth/login", h.handleAuthLogin)
	mux.HandleFunc("/robot/cases/login", h.handleLoginCase)
}

func (h *Handler) handleAuthLogin(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeAPIError(w, http.StatusMethodNotAllowed, "method_not_allowed", "only POST is supported")
		return
	}

	var input AuthLoginInput
	if err := decodeJSONBody(r, &input); err != nil {
		writeServiceError(w, err)
		return
	}

	result, err := h.service.AuthLogin(r.Context(), input)
	if err != nil {
		writeServiceError(w, err)
		return
	}
	writeAPISuccess(w, result)
}

func (h *Handler) handleLoginCase(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeAPIError(w, http.StatusMethodNotAllowed, "method_not_allowed", "only POST is supported")
		return
	}

	var input LoginCaseInput
	if err := decodeJSONBody(r, &input); err != nil {
		writeServiceError(w, err)
		return
	}

	result, err := h.service.LoginCase(r.Context(), input)
	if err != nil {
		writeServiceError(w, err)
		return
	}
	writeAPISuccess(w, result)
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
	default:
		xlog.Errorf("robot handler failed: %v", err)
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
		xlog.Errorf("write robot api response: %v", err)
	}
}
