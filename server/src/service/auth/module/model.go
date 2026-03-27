package module

import "time"

type Account struct {
	ID           string    `bson:"_id" json:"account_id"`
	Status       string    `bson:"status" json:"status"`
	LastChannel  string    `bson:"last_channel" json:"last_channel"`
	LastDeviceID string    `bson:"last_device_id" json:"last_device_id"`
	PlayerID     string    `bson:"player_id,omitempty" json:"player_id,omitempty"`
	CreatedAt    time.Time `bson:"created_at" json:"created_at"`
	UpdatedAt    time.Time `bson:"updated_at" json:"updated_at"`
	LastLoginAt  time.Time `bson:"last_login_at" json:"last_login_at"`
}

type AccountBinding struct {
	AccountID   string    `bson:"account_id" json:"account_id"`
	Channel     string    `bson:"channel" json:"channel"`
	Subject     string    `bson:"subject" json:"subject"`
	DeviceID    string    `bson:"device_id" json:"device_id"`
	CreatedAt   time.Time `bson:"created_at" json:"created_at"`
	UpdatedAt   time.Time `bson:"updated_at" json:"updated_at"`
	LastLoginAt time.Time `bson:"last_login_at" json:"last_login_at"`
}

type Session struct {
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

type LoginTicket struct {
	TicketID  string    `json:"ticket_id"`
	SessionID string    `json:"session_id"`
	AccountID string    `json:"account_id"`
	CreatedAt time.Time `json:"created_at"`
	ExpiresAt time.Time `json:"expires_at"`
}

type ConnectToken struct {
	TokenID   string    `json:"token"`
	SessionID string    `json:"session_id"`
	AccountID string    `json:"account_id"`
	DeviceID  string    `json:"device_id"`
	CreatedAt time.Time `json:"created_at"`
	ExpiresAt time.Time `json:"expires_at"`
}

type LoginInput struct {
	Channel  string `json:"channel"`
	Subject  string `json:"subject"`
	DeviceID string `json:"device_id"`
}

type LoginResult struct {
	AccountID         string    `json:"account_id"`
	PlayerID          string    `json:"player_id,omitempty"`
	SessionID         string    `json:"session_id"`
	LoginTicket       string    `json:"login_ticket"`
	ConnectToken      string    `json:"connect_token"`
	SessionExpiresAt  time.Time `json:"session_expires_at"`
	TicketExpiresAt   time.Time `json:"ticket_expires_at"`
	ConnectExpiresAt  time.Time `json:"connect_expires_at"`
	SessionTTLSeconds int64     `json:"session_ttl_seconds"`
	TicketTTLSeconds  int64     `json:"ticket_ttl_seconds"`
	ConnectTTLSeconds int64     `json:"connect_ttl_seconds"`
}

type ConsumeTicketInput struct {
	Ticket string `json:"ticket"`
}

type ConsumeConnectTokenInput struct {
	Token string `json:"token"`
}

type ValidateSessionInput struct {
	SessionID string `json:"session_id"`
}
