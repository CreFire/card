package mongo

import (
	"context"
	"fmt"

	"backend/src/common/configdoc"

	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
	"go.mongodb.org/mongo-driver/mongo/readpref"
)

type Client struct {
	raw      *mongo.Client
	database *mongo.Database
}

func NewFromConfig(cfg *configdoc.ConfigBase) (*Client, error) {
	clientOptions := options.Client().
		ApplyURI(cfg.MongoDB.URI).
		SetConnectTimeout(cfg.MongoConnectTimeout()).
		SetServerSelectionTimeout(cfg.MongoServerSelectionTimeout()).
		SetMaxPoolSize(cfg.MongoDB.MaxPoolSize).
		SetMinPoolSize(cfg.MongoDB.MinPoolSize)

	ctx, cancel := context.WithTimeout(context.Background(), cfg.MongoConnectTimeout())
	defer cancel()

	client, err := mongo.Connect(ctx, clientOptions)
	if err != nil {
		return nil, fmt.Errorf("connect mongodb: %w", err)
	}

	if err := client.Ping(ctx, readpref.Primary()); err != nil {
		_ = client.Disconnect(context.Background())
		return nil, fmt.Errorf("ping mongodb: %w", err)
	}

	return &Client{
		raw:      client,
		database: client.Database(cfg.MongoDB.Database),
	}, nil
}

func (c *Client) Raw() *mongo.Client {
	return c.raw
}

func (c *Client) Database() *mongo.Database {
	return c.database
}

func (c *Client) Ping(ctx context.Context) error {
	return c.raw.Ping(ctx, readpref.Primary())
}

func (c *Client) Close(ctx context.Context) error {
	return c.raw.Disconnect(ctx)
}
