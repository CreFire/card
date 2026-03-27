package module

import (
	"io"
	"strconv"
	"sync"
	"time"

	"backend/src/proto/pb"
)

type GamerManager struct {
	mu        sync.RWMutex
	byConnID  map[string]*Gamer
	bySession map[string]*Gamer
	closers   map[string]io.Closer
}

func NewGamerManager() *GamerManager {
	return &GamerManager{
		byConnID:  make(map[string]*Gamer),
		bySession: make(map[string]*Gamer),
		closers:   make(map[string]io.Closer),
	}
}

func (m *GamerManager) Open(remoteAddr string, closer io.Closer) (*Gamer, error) {
	connID, err := newID("conn", 12)
	if err != nil {
		return nil, err
	}

	now := time.Now().UTC()
	gamer := &Gamer{
		ConnID:      connID,
		RemoteAddr:  remoteAddr,
		Status:      pb.GamerStatus_INCOMING,
		ConnectedAt: now,
		LastSeenAt:  now,
	}

	m.mu.Lock()
	defer m.mu.Unlock()
	m.byConnID[gamer.ConnID] = gamer
	if closer != nil {
		m.closers[gamer.ConnID] = closer
	}
	return cloneGamer(gamer), nil
}

func (m *GamerManager) BindSession(connID string, session *AuthSession) (*Gamer, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	gamer, ok := m.byConnID[connID]
	if !ok {
		return nil, ErrConnNotFound
	}

	if old := m.bySession[session.SessionID]; old != nil && old.ConnID != connID {
		m.closeLocked(old.ConnID)
		m.deleteLocked(old)
	}

	now := time.Now().UTC()
	gamer.AccountID = session.AccountID
	gamer.PlayerID = session.PlayerID
	gamer.SessionID = session.SessionID
	gamer.Channel = session.Channel
	gamer.DeviceID = session.DeviceID
	gamer.Status = pb.GamerStatus_ONLINE
	gamer.LastSeenAt = now
	gamer.Profile = buildGamerProfile(session)
	m.bySession[session.SessionID] = gamer

	return cloneGamer(gamer), nil
}

func (m *GamerManager) Disconnect(connID string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	gamer, ok := m.byConnID[connID]
	if !ok {
		return ErrConnNotFound
	}
	m.closeLocked(connID)
	m.deleteLocked(gamer)
	return nil
}

func (m *GamerManager) CloseAll() {
	m.mu.Lock()
	defer m.mu.Unlock()

	for connID := range m.byConnID {
		m.closeLocked(connID)
	}
	m.byConnID = make(map[string]*Gamer)
	m.bySession = make(map[string]*Gamer)
	m.closers = make(map[string]io.Closer)
}

func (m *GamerManager) Get(connID string) (*Gamer, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	gamer, ok := m.byConnID[connID]
	if !ok {
		return nil, ErrConnNotFound
	}
	return cloneGamer(gamer), nil
}

func (m *GamerManager) List() []*Gamer {
	m.mu.RLock()
	defer m.mu.RUnlock()

	items := make([]*Gamer, 0, len(m.byConnID))
	for _, gamer := range m.byConnID {
		items = append(items, cloneGamer(gamer))
	}
	return items
}

func (m *GamerManager) Stats() GateStats {
	m.mu.RLock()
	defer m.mu.RUnlock()

	return GateStats{
		Connections: len(m.byConnID),
		Sessions:    len(m.bySession),
	}
}

func (m *GamerManager) closeLocked(connID string) {
	closer, ok := m.closers[connID]
	if !ok || closer == nil {
		return
	}
	_ = closer.Close()
	delete(m.closers, connID)
}

func (m *GamerManager) deleteLocked(gamer *Gamer) {
	delete(m.byConnID, gamer.ConnID)
	if gamer.SessionID != "" {
		delete(m.bySession, gamer.SessionID)
	}
	delete(m.closers, gamer.ConnID)
}

func cloneGamer(g *Gamer) *Gamer {
	if g == nil {
		return nil
	}
	clone := *g
	if g.Profile != nil {
		profile := *g.Profile
		clone.Profile = &profile
	}
	return &clone
}

func buildGamerProfile(session *AuthSession) *pb.Gamer {
	profile := &pb.Gamer{
		Account:   session.AccountID,
		Session:   session.SessionID,
		LoginTime: session.LastLoginAt.Unix(),
	}

	if session.PlayerID == "" {
		return profile
	}

	playerID, err := strconv.ParseInt(session.PlayerID, 10, 64)
	if err != nil {
		return profile
	}
	profile.Id = playerID
	return profile
}
