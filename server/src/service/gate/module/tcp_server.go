package module

import (
	"bufio"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"sync"

	"backend/deps/xlog"
)

type TCPServer struct {
	addr     string
	service  *GateService
	listener net.Listener
	wg       sync.WaitGroup
}

type tcpRequest struct {
	Op        string `json:"op"`
	Token     string `json:"token,omitempty"`
	Ticket    string `json:"ticket,omitempty"`
	SessionID string `json:"session_id,omitempty"`
}

func NewTCPServer(addr string, service *GateService) *TCPServer {
	return &TCPServer{
		addr:    addr,
		service: service,
	}
}

func (s *TCPServer) Start() error {
	listener, err := net.Listen("tcp", s.addr)
	if err != nil {
		return fmt.Errorf("listen tcp on %s: %w", s.addr, err)
	}
	s.listener = listener
	xlog.Infof("[gate] tcp listening on %s", s.addr)

	s.wg.Add(1)
	go s.acceptLoop()
	return nil
}

func (s *TCPServer) Stop() error {
	if s.listener != nil {
		_ = s.listener.Close()
	}
	s.service.CloseAll()
	s.wg.Wait()
	return nil
}

func (s *TCPServer) acceptLoop() {
	defer s.wg.Done()

	for {
		conn, err := s.listener.Accept()
		if err != nil {
			if errors.Is(err, net.ErrClosed) {
				return
			}
			xlog.Errorf("[gate] accept tcp failed: %v", err)
			continue
		}

		s.wg.Add(1)
		go s.handleConn(conn)
	}
}

func (s *TCPServer) handleConn(conn net.Conn) {
	defer s.wg.Done()

	gamer, err := s.service.OpenSocket(context.Background(), conn.RemoteAddr().String(), conn)
	if err != nil {
		xlog.Errorf("[gate] open socket failed: %v", err)
		_ = conn.Close()
		return
	}

	defer func() {
		err := s.service.Disconnect(context.Background(), DisconnectInput{ConnID: gamer.ConnID})
		if err != nil && !errors.Is(err, ErrConnNotFound) {
			xlog.Errorf("[gate] disconnect conn_id=%s: %v", gamer.ConnID, err)
		}
	}()

	writeTCPResponse(conn, map[string]any{
		"code":    "connected",
		"conn_id": gamer.ConnID,
	})

	scanner := bufio.NewScanner(conn)
	scanner.Buffer(make([]byte, 1024), 1024*1024)

	for scanner.Scan() {
		var req tcpRequest
		if err := json.Unmarshal(scanner.Bytes(), &req); err != nil {
			writeTCPResponse(conn, map[string]any{
				"code":    "invalid_json",
				"message": "request must be newline-delimited json",
			})
			continue
		}

		switch req.Op {
		case "ping":
			writeTCPResponse(conn, map[string]any{
				"code":    "pong",
				"conn_id": gamer.ConnID,
			})
		case "auth.token":
			bound, err := s.service.LoginByToken(context.Background(), AttachTokenInput{
				ConnID: gamer.ConnID,
				Token:  req.Token,
			})
			if err != nil {
				writeTCPResponse(conn, map[string]any{
					"code":    "auth_failed",
					"message": err.Error(),
				})
				continue
			}
			writeTCPResponse(conn, map[string]any{
				"code":  "auth_ok",
				"gamer": bound,
			})
		case "login.token":
			bound, err := s.service.LoginByToken(context.Background(), AttachTokenInput{
				ConnID: gamer.ConnID,
				Token:  req.Token,
			})
			if err != nil {
				writeTCPResponse(conn, map[string]any{
					"code":    "login_failed",
					"message": err.Error(),
				})
				continue
			}
			writeTCPResponse(conn, map[string]any{
				"code":  "login_ok",
				"gamer": bound,
			})
		case "auth.ticket":
			bound, err := s.service.LoginByTicket(context.Background(), AttachTicketInput{
				ConnID: gamer.ConnID,
				Ticket: req.Ticket,
			})
			if err != nil {
				writeTCPResponse(conn, map[string]any{
					"code":    "auth_failed",
					"message": err.Error(),
				})
				continue
			}
			writeTCPResponse(conn, map[string]any{
				"code":  "auth_ok",
				"gamer": bound,
			})
		case "login.ticket":
			bound, err := s.service.LoginByTicket(context.Background(), AttachTicketInput{
				ConnID: gamer.ConnID,
				Ticket: req.Ticket,
			})
			if err != nil {
				writeTCPResponse(conn, map[string]any{
					"code":    "login_failed",
					"message": err.Error(),
				})
				continue
			}
			writeTCPResponse(conn, map[string]any{
				"code":  "login_ok",
				"gamer": bound,
			})
		case "auth.session":
			bound, err := s.service.LoginBySession(context.Background(), AttachSessionInput{
				ConnID:    gamer.ConnID,
				SessionID: req.SessionID,
			})
			if err != nil {
				writeTCPResponse(conn, map[string]any{
					"code":    "auth_failed",
					"message": err.Error(),
				})
				continue
			}
			writeTCPResponse(conn, map[string]any{
				"code":  "auth_ok",
				"gamer": bound,
			})
		case "login.session":
			bound, err := s.service.LoginBySession(context.Background(), AttachSessionInput{
				ConnID:    gamer.ConnID,
				SessionID: req.SessionID,
			})
			if err != nil {
				writeTCPResponse(conn, map[string]any{
					"code":    "login_failed",
					"message": err.Error(),
				})
				continue
			}
			writeTCPResponse(conn, map[string]any{
				"code":  "login_ok",
				"gamer": bound,
			})
		default:
			writeTCPResponse(conn, map[string]any{
				"code":    "unsupported_op",
				"message": "supported ops: ping, login.token, login.ticket, login.session",
			})
		}
	}

	if err := scanner.Err(); err != nil && !errors.Is(err, net.ErrClosed) {
		xlog.Errorf("[gate] tcp read loop conn_id=%s: %v", gamer.ConnID, err)
	}
}

func writeTCPResponse(conn net.Conn, payload any) {
	if err := json.NewEncoder(conn).Encode(payload); err != nil {
		xlog.Errorf("[gate] write tcp response: %v", err)
	}
}
