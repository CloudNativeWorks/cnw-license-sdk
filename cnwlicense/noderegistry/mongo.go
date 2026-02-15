package noderegistry

import (
	"context"
	"fmt"
	"regexp"
	"time"

	"go.mongodb.org/mongo-driver/v2/bson"
	"go.mongodb.org/mongo-driver/v2/mongo"
	"go.mongodb.org/mongo-driver/v2/mongo/options"
)

const defaultMongoCollection = "cnw_license_nodes"

// validCollectionName matches safe MongoDB collection names.
var validCollectionName = regexp.MustCompile(`^[a-zA-Z_][a-zA-Z0-9_]*$`)

// MongoOption configures a MongoRegistry.
type MongoOption func(*MongoRegistry)

// WithCollectionName sets the MongoDB collection name. Default: "cnw_license_nodes".
func WithCollectionName(name string) MongoOption {
	return func(r *MongoRegistry) {
		r.collectionName = name
	}
}

// MongoRegistry implements NodeRegistry using MongoDB.
type MongoRegistry struct {
	collection     *mongo.Collection
	collectionName string
}

// NewMongoRegistry creates a new MongoDB-backed node registry.
// It creates the necessary indexes on initialization.
func NewMongoRegistry(ctx context.Context, db *mongo.Database, opts ...MongoOption) (*MongoRegistry, error) {
	r := &MongoRegistry{
		collectionName: defaultMongoCollection,
	}
	for _, opt := range opts {
		opt(r)
	}
	if !validCollectionName.MatchString(r.collectionName) {
		return nil, fmt.Errorf("invalid collection name %q: must match [a-zA-Z_][a-zA-Z0-9_]*", r.collectionName)
	}
	r.collection = db.Collection(r.collectionName)

	if err := r.ensureIndexes(ctx); err != nil {
		return nil, fmt.Errorf("create indexes: %w", err)
	}
	return r, nil
}

func (r *MongoRegistry) ensureIndexes(ctx context.Context) error {
	indexes := []mongo.IndexModel{
		{
			Keys:    bson.D{{Key: "fingerprint", Value: 1}},
			Options: options.Index().SetUnique(true),
		},
		{
			Keys: bson.D{
				{Key: "license_key", Value: 1},
				{Key: "last_seen_at", Value: 1},
			},
		},
	}
	_, err := r.collection.Indexes().CreateMany(ctx, indexes)
	return err
}

func (r *MongoRegistry) Register(ctx context.Context, node NodeInfo) (*NodeInfo, error) {
	now := time.Now()
	filter := bson.M{"fingerprint": node.Fingerprint}
	update := bson.M{
		"$set": bson.M{
			"hostname":     node.Hostname,
			"ip":           node.IP,
			"os":           node.OS,
			"license_key":  node.LicenseKey,
			"last_seen_at": now,
		},
		"$setOnInsert": bson.M{
			"registered_at": now,
		},
	}

	// FindOneAndUpdate with ReturnDocument=After gives us the actual DB values,
	// ensuring registered_at is correct for existing nodes.
	opts := options.FindOneAndUpdate().
		SetUpsert(true).
		SetReturnDocument(options.After)
	var result NodeInfo
	err := r.collection.FindOneAndUpdate(ctx, filter, update, opts).Decode(&result)
	if err != nil {
		return nil, fmt.Errorf("register node: %w", err)
	}
	return &result, nil
}

func (r *MongoRegistry) Deregister(ctx context.Context, fingerprint string) error {
	_, err := r.collection.DeleteOne(ctx, bson.M{"fingerprint": fingerprint})
	if err != nil {
		return fmt.Errorf("deregister node: %w", err)
	}
	return nil
}

func (r *MongoRegistry) Count(ctx context.Context, licenseKey string) (int, error) {
	count, err := r.collection.CountDocuments(ctx, bson.M{"license_key": licenseKey})
	if err != nil {
		return 0, fmt.Errorf("count nodes: %w", err)
	}
	return int(count), nil
}

func (r *MongoRegistry) List(ctx context.Context, licenseKey string) ([]NodeInfo, error) {
	cursor, err := r.collection.Find(ctx, bson.M{"license_key": licenseKey})
	if err != nil {
		return nil, fmt.Errorf("list nodes: %w", err)
	}
	var nodes []NodeInfo
	if err := cursor.All(ctx, &nodes); err != nil {
		return nil, fmt.Errorf("decode nodes: %w", err)
	}
	return nodes, nil
}

func (r *MongoRegistry) Ping(ctx context.Context, fingerprint string) error {
	_, err := r.collection.UpdateOne(ctx,
		bson.M{"fingerprint": fingerprint},
		bson.M{"$set": bson.M{"last_seen_at": time.Now()}},
	)
	if err != nil {
		return fmt.Errorf("ping node: %w", err)
	}
	return nil
}

func (r *MongoRegistry) Prune(ctx context.Context, licenseKey string, olderThan time.Duration) (int, error) {
	cutoff := time.Now().Add(-olderThan)
	result, err := r.collection.DeleteMany(ctx, bson.M{
		"license_key":  licenseKey,
		"last_seen_at": bson.M{"$lt": cutoff},
	})
	if err != nil {
		return 0, fmt.Errorf("prune nodes: %w", err)
	}
	return int(result.DeletedCount), nil
}

func (r *MongoRegistry) Close(_ context.Context) error {
	return nil // user manages the mongo.Database lifecycle
}
