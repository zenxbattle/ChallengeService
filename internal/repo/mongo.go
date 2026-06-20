package repo

import (
	"context"
	"errors"
	"fmt"

	// "github.com/google/uuid"
	"github.com/lijuuu/ChallengeWssManagerService/internal/model"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

type MongoRepository struct {
	challenges *mongo.Collection
}

func NewMongoRepository(client *mongo.Client, dbName string) *MongoRepository {
	return &MongoRepository{
		challenges: client.Database(dbName).Collection("challenges"),
	}
}

// PersistChallengeFromRedis persists challenge data from Redis to MongoDB for historical storage
func (r *MongoRepository) PersistChallengeFromRedis(ctx context.Context, challenge *model.ChallengeDocument) error {
	if challenge == nil {
		return errors.New("challenge cannot be nil")
	}

	// Check if challenge already exists in MongoDB
	filter := bson.M{"challengeId": challenge.ChallengeID}
	var existingChallenge model.ChallengeDocument
	err := r.challenges.FindOne(ctx, filter).Decode(&existingChallenge)

	if err == mongo.ErrNoDocuments {
		// Challenge doesn't exist, insert it
		_, err = r.challenges.InsertOne(ctx, challenge)
		return err
	} else if err != nil {
		// Some other error occurred
		return fmt.Errorf("failed to check existing challenge: %w", err)
	}

	// Challenge exists, update it
	update := bson.M{
		"$set": bson.M{
			"status":              challenge.Status,
			"participants":        challenge.Participants,
			"submissions":         challenge.Submissions,
			"leaderboard":         challenge.Leaderboard,
			"startTime":           challenge.StartTime,
			"processedProblemIds": challenge.ProcessedProblemIds,
			"problemCount":        challenge.ProblemCount,
		},
	}

	_, err = r.challenges.UpdateOne(ctx, filter, update)
	return err
}

// GetChallengeHistory returns challenge history; toggle it using isPrivate
func (r *MongoRepository) GetChallengeHistory(ctx context.Context, userID string, page, pageSize int, isPrivate bool) ([]model.ChallengeDocument, error) {
	if page < 1 || pageSize < 1 || userID == "" {
		return nil, errors.New("invalid pagination or userID")
	}

	filter := bson.M{
		"isPrivate": isPrivate,
		"$or": []bson.M{
			{"creatorId": userID},
			{"participants." + userID: bson.M{"$exists": true}},
		},
	}

	opts := options.Find().
		SetSort(bson.D{{Key: "startTime", Value: -1}}).
		SetSkip(int64((page - 1) * pageSize)).
		SetLimit(int64(pageSize))

	cursor, err := r.challenges.Find(ctx, filter, opts)
	if err != nil {
		return nil, err
	}
	defer cursor.Close(ctx)

	var results []model.ChallengeDocument
	if err := cursor.All(ctx, &results); err != nil {
		return nil, err
	}
	return results, nil
}

func (r *MongoRepository) AbandonChallenge(ctx context.Context, creatorId, challengeId string) error {
	filter := bson.M{
		"creatorId":   creatorId,
		"challengeId": challengeId,
	}

	update := bson.M{
		"$set": bson.M{
			"status": model.ChallengeAbandon,
		},
	}

	_, err := r.challenges.UpdateOne(ctx, filter, update)
	return err
}

func (r *MongoRepository) GetChallengeByID(ctx context.Context, challengeId string) (model.ChallengeDocument, error) {
	filter := bson.M{
		"challengeId": challengeId,
	}

	var result model.ChallengeDocument
	err := r.challenges.FindOne(ctx, filter).Decode(&result)
	return result, err
}

// GetActiveChallenges returns challenges not marked as finished
func (r *MongoRepository) GetActiveOpenChallenges(ctx context.Context, page, pageSize int) ([]model.ChallengeDocument, error) {
	if page < 1 || pageSize < 1 {
		return nil, errors.New("invalid pagination")
	}

	filter := bson.M{
		"status":    model.ChallengeOpen,
		"isPrivate": false,
	}
	opts := options.Find().
		SetSort(bson.D{{Key: "startTime", Value: 1}}).
		SetSkip(int64((page - 1) * pageSize)).
		SetLimit(int64(pageSize))

	cursor, err := r.challenges.Find(ctx, filter, opts)
	if err != nil {
		return nil, err
	}
	defer cursor.Close(ctx)

	var results []model.ChallengeDocument
	if err := cursor.All(ctx, &results); err != nil {
		return nil, err
	}
	return results, nil
}

// GetOwnersActiveChallenges returns challenges created by a specific user that are either open or started
func (r *MongoRepository) GetOwnersActiveChallenges(ctx context.Context, userID string, page, pageSize int) ([]model.ChallengeDocument, error) {
	if page < 1 || pageSize < 1 || userID == "" {
		return nil, errors.New("invalid pagination or userID")
	}

	filter := bson.M{
		"creatorId": userID,
		"status": bson.M{
			"$in": []string{model.ChallengeOpen, model.ChallengeStarted},
		},
	}
	opts := options.Find().
		SetSort(bson.D{{Key: "startTime", Value: -1}}).
		SetSkip(int64((page - 1) * pageSize)).
		SetLimit(int64(pageSize))

	cursor, err := r.challenges.Find(ctx, filter, opts)
	if err != nil {
		return nil, err
	}
	defer cursor.Close(ctx)

	var results []model.ChallengeDocument
	if err := cursor.All(ctx, &results); err != nil {
		return nil, err
	}
	return results, nil
}
