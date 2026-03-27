package module

import (
	"context"
	"errors"
	"io"
	"net"
	"net/http"

	"backend/deps/xlog"
	"golang.org/x/net/websocket"
)

type WebSocketServer struct {
	service *GateService
}

func NewWebSocketServer(service *GateService) *WebSocketServer {
	return &WebSocketServer{service: service}
}

func (s *WebSocketServer) RegisterHTTP(mux *http.ServeMux) {
	mux.Handle("/gate/ws", websocket.Handler(s.handleWS))
}

func (s *WebSocketServer) handleWS(ws *websocket.Conn) {
	defer func() {
		_ = ws.Close()
	}()

	gamer, err := s.service.OpenSocket(context.Background(), remoteAddrFromWS(ws), ws)
	if err != nil {
		writeWSResponse(ws, map[string]any{
			"code":    "connect_failed",
			"message": err.Error(),
		})
		return
	}

	defer func() {
		err := s.service.Disconnect(context.Background(), DisconnectInput{ConnID: gamer.ConnID})
		if err != nil && !errors.Is(err, ErrConnNotFound) {
			xlog.Errorf("[gate] disconnect websocket conn_id=%s: %v", gamer.ConnID, err)
		}
	}()

	writeWSResponse(ws, map[string]any{
		"code":    "connected",
		"conn_id": gamer.ConnID,
	})

	for {
		var req tcpRequest
		if err := websocket.JSON.Receive(ws, &req); err != nil {
			if !errors.Is(err, io.EOF) && !errors.Is(err, net.ErrClosed) {
				xlog.Errorf("[gate] websocket read conn_id=%s: %v", gamer.ConnID, err)
			}
			return
		}

		switch req.Op {
		case "ping":
			writeWSResponse(ws, map[string]any{
				"code":    "pong",
				"conn_id": gamer.ConnID,
			})
		case "auth.token":
			bound, err := s.service.LoginByToken(context.Background(), AttachTokenInput{
				ConnID: gamer.ConnID,
				Token:  req.Token,
			})
			if err != nil {
				writeWSResponse(ws, map[string]any{
					"code":    "auth_failed",
					"message": err.Error(),
				})
				continue
			}
			writeWSResponse(ws, map[string]any{
				"code":  "auth_ok",
				"gamer": bound,
			})
		case "login.token":
			bound, err := s.service.LoginByToken(context.Background(), AttachTokenInput{
				ConnID: gamer.ConnID,
				Token:  req.Token,
			})
			if err != nil {
				writeWSResponse(ws, map[string]any{
					"code":    "login_failed",
					"message": err.Error(),
				})
				continue
			}
			writeWSResponse(ws, map[string]any{
				"code":  "login_ok",
				"gamer": bound,
			})
		case "auth.ticket":
			bound, err := s.service.LoginByTicket(context.Background(), AttachTicketInput{
				ConnID: gamer.ConnID,
				Ticket: req.Ticket,
			})
			if err != nil {
				writeWSResponse(ws, map[string]any{
					"code":    "auth_failed",
					"message": err.Error(),
				})
				continue
			}
			writeWSResponse(ws, map[string]any{
				"code":  "auth_ok",
				"gamer": bound,
			})
		case "login.ticket":
			bound, err := s.service.LoginByTicket(context.Background(), AttachTicketInput{
				ConnID: gamer.ConnID,
				Ticket: req.Ticket,
			})
			if err != nil {
				writeWSResponse(ws, map[string]any{
					"code":    "login_failed",
					"message": err.Error(),
				})
				continue
			}
			writeWSResponse(ws, map[string]any{
				"code":  "login_ok",
				"gamer": bound,
			})
		case "auth.session":
			bound, err := s.service.LoginBySession(context.Background(), AttachSessionInput{
				ConnID:    gamer.ConnID,
				SessionID: req.SessionID,
			})
			if err != nil {
				writeWSResponse(ws, map[string]any{
					"code":    "auth_failed",
					"message": err.Error(),
				})
				continue
			}
			writeWSResponse(ws, map[string]any{
				"code":  "auth_ok",
				"gamer": bound,
			})
		case "login.session":
			bound, err := s.service.LoginBySession(context.Background(), AttachSessionInput{
				ConnID:    gamer.ConnID,
				SessionID: req.SessionID,
			})
			if err != nil {
				writeWSResponse(ws, map[string]any{
					"code":    "login_failed",
					"message": err.Error(),
				})
				continue
			}
			writeWSResponse(ws, map[string]any{
				"code":  "login_ok",
				"gamer": bound,
			})
		default:
			writeWSResponse(ws, map[string]any{
				"code":    "unsupported_op",
				"message": "supported ops: ping, login.token, login.ticket, login.session",
			})
		}
	}
}

func writeWSResponse(ws *websocket.Conn, payload any) {
	if err := websocket.JSON.Send(ws, payload); err != nil {
		xlog.Errorf("[gate] write websocket response: %v", err)
	}
}

func remoteAddrFromWS(ws *websocket.Conn) string {
	if ws.Request() != nil {
		return ws.Request().RemoteAddr
	}
	return ""
}
