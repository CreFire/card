package module

type LoginCaseInput struct {
	Channel    string `json:"channel"`
	Subject    string `json:"subject"`
	DeviceID   string `json:"device_id"`
	RemoteAddr string `json:"remote_addr"`
}

type AuthLoginInput struct {
	Channel  string `json:"channel"`
	Subject  string `json:"subject"`
	DeviceID string `json:"device_id"`
}

type AuthLoginResult struct {
	AccountID         string `json:"account_id"`
	PlayerID          string `json:"player_id,omitempty"`
	SessionID         string `json:"session_id"`
	LoginTicket       string `json:"login_ticket"`
	ConnectToken      string `json:"connect_token"`
	SessionTTLSeconds int64  `json:"session_ttl_seconds"`
	TicketTTLSeconds  int64  `json:"ticket_ttl_seconds"`
	ConnectTTLSeconds int64  `json:"connect_ttl_seconds"`
}

type OpenConnectionInput struct {
	RemoteAddr string `json:"remote_addr"`
}

type GateGamer struct {
	ConnID     string `json:"conn_id"`
	AccountID  string `json:"account_id,omitempty"`
	PlayerID   string `json:"player_id,omitempty"`
	SessionID  string `json:"session_id,omitempty"`
	Channel    string `json:"channel,omitempty"`
	DeviceID   string `json:"device_id,omitempty"`
	RemoteAddr string `json:"remote_addr,omitempty"`
}

type GateLoginByTokenInput struct {
	ConnID string `json:"conn_id"`
	Token  string `json:"token"`
}

type LoginCaseResult struct {
	Auth   *AuthLoginResult `json:"auth"`
	Opened *GateGamer       `json:"opened"`
	Bound  *GateGamer       `json:"bound"`
}
