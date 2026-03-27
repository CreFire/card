package module

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	goredis "github.com/redis/go-redis/v9"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"

	mongostore "backend/src/persist/mongo"
	redisstore "backend/src/persist/redis"
)

const (
	accountCollectionName = "accounts"
	bindingCollectionName = "account_bindings"
	sessionKeyPrefix      = "auth:session:"
	accountSessionPrefix  = "auth:account_session:"
	loginTicketPrefix     = "auth:login_ticket:"
	connectTokenPrefix    = "auth:connect_token:"
)

type AccountRepository struct {
	accounts *mongo.Collection
	bindings *mongo.Collection
}

func NewAccountRepository(client *mongostore.Client) *AccountRepository {
	db := client.Database()
	return &AccountRepository{
		accounts: db.Collection(accountCollectionName),
		bindings: db.Collection(bindingCollectionName),
	}
}

func (r *AccountRepository) EnsureIndexes(ctx context.Context) error {
	_, err := r.bindings.Indexes().CreateOne(ctx, mongo.IndexModel{
		Keys: bson.D{
			{Key: "channel", Value: 1},
			{Key: "subject", Value: 1},
		},
		Options: options.Index().SetUnique(true).SetName("uniq_channel_subject"),
	})
	if err != nil {
		return fmt.Errorf("create binding index: %w", err)
	}
	return nil
}

func (r *AccountRepository) FindOrCreateBinding(ctx context.Context, channel, subject, deviceID string, now time.Time) (*AccountBinding, error) {
	accountID, err := newID("acc", 12)
	if err != nil {
		return nil, err
	}

	filter := bson.M{
		"channel": channel,
		"subject": subject,
	}
	update := bson.M{
		"$set": bson.M{
			"device_id":     deviceID,
			"updated_at":    now,
			"last_login_at": now,
		},
		"$setOnInsert": bson.M{
			"account_id":    accountID,
			"channel":       channel,
			"subject":       subject,
			"device_id":     deviceID,
			"created_at":    now,
			"updated_at":    now,
			"last_login_at": now,
		},
	}
	opts := options.FindOneAndUpdate().
		SetUpsert(true).
		SetReturnDocument(options.After)

	var binding AccountBinding
	if err := r.bindings.FindOneAndUpdate(ctx, filter, update, opts).Decode(&binding); err != nil {
		return nil, fmt.Errorf("find or create binding: %w", err)
	}
	return &binding, nil
}

func (r *AccountRepository) UpsertAccount(ctx context.Context, binding *AccountBinding, now time.Time) (*Account, error) {
	filter := bson.M{"_id": binding.AccountID}
	update := bson.M{
		"$set": bson.M{
			"status":         "active",
			"last_channel":   binding.Channel,
			"last_device_id": binding.DeviceID,
			"updated_at":     now,
			"last_login_at":  now,
		},
		"$setOnInsert": bson.M{
			"created_at": now,
		},
	}
	opts := options.FindOneAndUpdate().
		SetUpsert(true).
		SetReturnDocument(options.After)

	var account Account
	if err := r.accounts.FindOneAndUpdate(ctx, filter, update, opts).Decode(&account); err != nil {
		return nil, fmt.Errorf("upsert account: %w", err)
	}
	return &account, nil
}

type SessionRepository struct {
	redis goredis.UniversalClient
}

func NewSessionRepository(client *redisstore.Client) *SessionRepository {
	return &SessionRepository{redis: client.Raw()}
}

func (r *SessionRepository) SaveSession(ctx context.Context, session *Session, ttl time.Duration) error {
	oldSessionID, err := r.redis.Get(ctx, accountSessionKey(session.AccountID)).Result()
	if err != nil && !errors.Is(err, goredis.Nil) {
		return fmt.Errorf("get current account session: %w", err)
	}

	payload, err := json.Marshal(session)
	if err != nil {
		return fmt.Errorf("marshal session: %w", err)
	}

	pipe := r.redis.TxPipeline()
	pipe.Set(ctx, sessionKey(session.SessionID), payload, ttl)
	pipe.Set(ctx, accountSessionKey(session.AccountID), session.SessionID, ttl)
	if oldSessionID != "" && oldSessionID != session.SessionID {
		pipe.Del(ctx, sessionKey(oldSessionID))
	}
	if _, err := pipe.Exec(ctx); err != nil {
		return fmt.Errorf("save session: %w", err)
	}
	return nil
}

func (r *SessionRepository) GetSession(ctx context.Context, sessionID string) (*Session, error) {
	payload, err := r.redis.Get(ctx, sessionKey(sessionID)).Bytes()
	if errors.Is(err, goredis.Nil) {
		return nil, ErrSessionNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("get session: %w", err)
	}

	var session Session
	if err := json.Unmarshal(payload, &session); err != nil {
		return nil, fmt.Errorf("unmarshal session: %w", err)
	}
	return &session, nil
}

func (r *SessionRepository) DeleteSession(ctx context.Context, sessionID string) error {
	session, err := r.GetSession(ctx, sessionID)
	if errors.Is(err, ErrSessionNotFound) {
		return nil
	}
	if err != nil {
		return err
	}

	currentSessionID, err := r.redis.Get(ctx, accountSessionKey(session.AccountID)).Result()
	if err != nil && !errors.Is(err, goredis.Nil) {
		return fmt.Errorf("get account session before delete: %w", err)
	}

	pipe := r.redis.TxPipeline()
	pipe.Del(ctx, sessionKey(sessionID))
	if currentSessionID == sessionID {
		pipe.Del(ctx, accountSessionKey(session.AccountID))
	}
	if _, err := pipe.Exec(ctx); err != nil {
		return fmt.Errorf("delete session: %w", err)
	}
	return nil
}

type TicketRepository struct {
	redis goredis.UniversalClient
}

func NewTicketRepository(client *redisstore.Client) *TicketRepository {
	return &TicketRepository{redis: client.Raw()}
}

func (r *TicketRepository) Save(ctx context.Context, ticket *LoginTicket, ttl time.Duration) error {
	payload, err := json.Marshal(ticket)
	if err != nil {
		return fmt.Errorf("marshal login ticket: %w", err)
	}
	if err := r.redis.Set(ctx, loginTicketKey(ticket.TicketID), payload, ttl).Err(); err != nil {
		return fmt.Errorf("save login ticket: %w", err)
	}
	return nil
}

func (r *TicketRepository) Consume(ctx context.Context, ticketID string) (*LoginTicket, error) {
	payload, err := r.redis.GetDel(ctx, loginTicketKey(ticketID)).Bytes()
	if errors.Is(err, goredis.Nil) {
		return nil, ErrTicketNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("consume login ticket: %w", err)
	}

	var ticket LoginTicket
	if err := json.Unmarshal(payload, &ticket); err != nil {
		return nil, fmt.Errorf("unmarshal login ticket: %w", err)
	}
	return &ticket, nil
}

func sessionKey(sessionID string) string {
	return sessionKeyPrefix + sessionID
}

func accountSessionKey(accountID string) string {
	return accountSessionPrefix + accountID
}

func loginTicketKey(ticketID string) string {
	return loginTicketPrefix + ticketID
}

type ConnectTokenRepository struct {
	redis goredis.UniversalClient
}

func NewConnectTokenRepository(client *redisstore.Client) *ConnectTokenRepository {
	return &ConnectTokenRepository{redis: client.Raw()}
}

func (r *ConnectTokenRepository) Save(ctx context.Context, token *ConnectToken, ttl time.Duration) error {
	payload, err := json.Marshal(token)
	if err != nil {
		return fmt.Errorf("marshal connect token: %w", err)
	}
	if err := r.redis.Set(ctx, connectTokenKey(token.TokenID), payload, ttl).Err(); err != nil {
		return fmt.Errorf("save connect token: %w", err)
	}
	return nil
}

func (r *ConnectTokenRepository) Consume(ctx context.Context, tokenID string) (*ConnectToken, error) {
	payload, err := r.redis.GetDel(ctx, connectTokenKey(tokenID)).Bytes()
	if errors.Is(err, goredis.Nil) {
		return nil, ErrConnectTokenNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("consume connect token: %w", err)
	}

	var token ConnectToken
	if err := json.Unmarshal(payload, &token); err != nil {
		return nil, fmt.Errorf("unmarshal connect token: %w", err)
	}
	return &token, nil
}

func connectTokenKey(tokenID string) string {
	return connectTokenPrefix + tokenID
}
