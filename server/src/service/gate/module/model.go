package module

import (
	"time"

	"backend/src/proto/pb"
)

type AuthSession struct {
	SessionID   string    `json:"session_id"`
	AccountID   string    `json:"account_id"`
	PlayerID    string    `json:"player_id,omitempty"`
	Channel     string    `json:"channel"`
	Subject     string    `json:"subject"`
	DeviceID    string    `json:"device_id"`
	CreatedAt   time.Time `json:"created_at"`
	ExpiresAt   time.Time `json:"expires_at"`
	LastLoginAt time.Time `json:"last_login_at"`
}

type Gamer struct {
	ConnID      string         `json:"conn_id"`
	AccountID   string         `json:"account_id,omitempty"`
	PlayerID    string         `json:"player_id,omitempty"`
	SessionID   string         `json:"session_id,omitempty"`
	Channel     string         `json:"channel,omitempty"`
	DeviceID    string         `json:"device_id,omitempty"`
	RemoteAddr  string         `json:"remote_addr,omitempty"`
	Status      pb.GamerStatus `json:"status"`
	Profile     *pb.Gamer      `json:"profile,omitempty"`
	ConnectedAt time.Time      `json:"connected_at"`
	LastSeenAt  time.Time      `json:"last_seen_at"`
}

type GateStats struct {
	Connections int `json:"connections"`
	Sessions    int `json:"sessions"`
}

type OpenConnectionInput struct {
	RemoteAddr string `json:"remote_addr"`
}

type AttachTicketInput struct {
	ConnID string `json:"conn_id"`
	Ticket string `json:"ticket"`
}

type AttachTokenInput struct {
	ConnID string `json:"conn_id"`
	Token  string `json:"token"`
}

type AttachSessionInput struct {
	ConnID    string `json:"conn_id"`
	SessionID string `json:"session_id"`
}

type DisconnectInput struct {
	ConnID string `json:"conn_id"`
}
