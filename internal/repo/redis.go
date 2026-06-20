package repo

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/lijuuu/ChallengeWssManagerService/internal/model"
	"github.com/redis/go-redis/v9"
)

type RedisRepository struct {
	client *redis.Client
}

func NewRedisRepository(client *redis.Client) *RedisRepository {
	return &RedisRepository{
		client: client,
	}
}

// CreateChallenge stores a new challenge in Redis
func (r *RedisRepository) CreateChallenge(ctx context.Context, challenge *model.ChallengeDocument) error {
	key := fmt.Sprintf("challenge:%s", challenge.ChallengeID)

	data, err := json.Marshal(challenge)
	if err != nil {
		return fmt.Errorf("failed to marshal challenge: %w", err)
	}

	// fmt.Println("create challenge ", string(data))

	return r.client.Set(ctx, key, data, 0).Err()
}

// GetChallenge retrieves a challenge from Redis
func (r *RedisRepository) GetChallenge(ctx context.Context, challengeID string) (*model.ChallengeDocument, error) {
	challengeDoc, err := r.GetChallengeByID(ctx, challengeID)
	if err != nil {
		return nil, err
	}
	return &challengeDoc, nil
}

// UpdateChallenge updates an existing challenge in Redis
func (r *RedisRepository) UpdateChallenge(ctx context.Context, challenge *model.ChallengeDocument) error {
	return r.CreateChallenge(ctx, challenge) // Same as create for Redis
}

// DeleteChallenge removes a challenge from Redis
func (r *RedisRepository) DeleteChallenge(ctx context.Context, challengeID string) error {
	key := fmt.Sprintf("challenge:%s", challengeID)
	return r.client.Del(ctx, key).Err()
}

// GetActiveChallenges returns all challenge IDs from Redis
func (r *RedisRepository) GetActiveChallenges(ctx context.Context) ([]string, error) {
	keys, err := r.client.Keys(ctx, "challenge:*").Result()
	if err != nil {
		return nil, fmt.Errorf("failed to get challenge keys: %w", err)
	}

	challengeIDs := make([]string, len(keys))
	for i, key := range keys {
		// Extract challenge ID from key (remove "challenge:" prefix)
		challengeIDs[i] = key[10:]
	}

	return challengeIDs, nil
}

// GetChallengesByStatus returns challenge IDs filtered by status
func (r *RedisRepository) GetChallengesByStatus(ctx context.Context, status string) ([]string, error) {
	challengeIDs, err := r.GetActiveChallenges(ctx)
	if err != nil {
		return nil, err
	}

	var filteredIDs []string
	for _, id := range challengeIDs {
		challenge, err := r.GetChallenge(ctx, id)
		if err != nil {
			continue // Skip challenges that can't be retrieved
		}

		if string(challenge.Status) == status {
			filteredIDs = append(filteredIDs, id)
		}
	}

	return filteredIDs, nil
}

// AddParticipant adds a participant to a challenge
func (r *RedisRepository) AddParticipant(ctx context.Context, challengeID, userID string, metadata *model.ParticipantMetadata) error {
	challenge, err := r.GetChallenge(ctx, challengeID)
	if err != nil {
		return err
	}

	if challenge.Participants == nil {
		challenge.Participants = make(map[string]*model.ParticipantMetadata)
	}

	challenge.Participants[userID] = metadata
	return r.UpdateChallenge(ctx, challenge)
}

// RemoveParticipant removes a participant from a challenge
func (r *RedisRepository) RemoveParticipant(ctx context.Context, challengeID, userID string) error {
	challenge, err := r.GetChallenge(ctx, challengeID)
	if err != nil {
		return err
	}

	if challenge.Participants != nil {
		delete(challenge.Participants, userID)
	}

	if challenge.Submissions != nil {
		delete(challenge.Submissions, userID)
	}

	return r.UpdateChallenge(ctx, challenge)
}

// UpdateParticipant updates participant metadata
func (r *RedisRepository) UpdateParticipant(ctx context.Context, challengeID, userID string, metadata *model.ParticipantMetadata) error {
	return r.AddParticipant(ctx, challengeID, userID, metadata)
}

// GetChallengeByID retrieves a challenge document from Redis
func (r *RedisRepository) GetChallengeByID(ctx context.Context, challengeID string) (model.ChallengeDocument, error) {
	key := fmt.Sprintf("challenge:%s", challengeID)

	data, err := r.client.Get(ctx, key).Result()
	if err != nil {
		if err == redis.Nil {
			return model.ChallengeDocument{}, fmt.Errorf("challenge not found")
		}
		return model.ChallengeDocument{}, fmt.Errorf("failed to get challenge: %w", err)
	}

	var challengeDoc model.ChallengeDocument
	if err := json.Unmarshal([]byte(data), &challengeDoc); err != nil {
		return model.ChallengeDocument{}, fmt.Errorf("failed to unmarshal challenge: %w", err)
	}

	return challengeDoc, nil
}

// AbandonChallenge updates challenge status to ABANDON in Redis
func (r *RedisRepository) AbandonChallenge(ctx context.Context, creatorID, challengeID string) error {
	challenge, err := r.GetChallenge(ctx, challengeID)
	if err != nil {
		return err
	}

	// Verify the creator
	if challenge.CreatorID != creatorID {
		return fmt.Errorf("only the creator can abandon the challenge")
	}

	// Update status to ABANDON
	challenge.Status = model.ChallengeAbandon
	return r.UpdateChallenge(ctx, challenge)
}

// RemoveParticipantInJoinPhase removes a participant during join phase
func (r *RedisRepository) RemoveParticipantInJoinPhase(ctx context.Context, challengeID, userID string) error {
	return r.RemoveParticipant(ctx, challengeID, userID)
}

// GetRedisAddr returns the Redis address from the client
func (r *RedisRepository) GetRedisAddr() string {
	return r.client.Options().Addr
}

// GetRedisPassword returns the Redis password from the client
func (r *RedisRepository) GetRedisPassword() string {
	return r.client.Options().Password
}

// CanJoin checks if a user can join a challenge
// Returns false if user is already the creator or already a participant
func (r *RedisRepository) CanJoin(ctx context.Context, challengeID, userID string) (bool, error) {
	if challengeID == "" || userID == "" {
		return false, fmt.Errorf("challengeID and userID cannot be empty")
	}

	challenge, err := r.GetChallenge(ctx, challengeID)
	if err != nil {
		return false, fmt.Errorf("failed to get challenge: %w", err)
	}

	// Check if user is the creator
	if challenge.CreatorID == userID {
		return false, fmt.Errorf("user is the creator of this challenge")
	}

	// Check if user is already a participant
	if challenge.Participants != nil {
		if _, exists := challenge.Participants[userID]; exists {
			return false, fmt.Errorf("user is already a participant in this challenge")
		}
	}

	// Check if challenge is in a joinable state
	if challenge.Status == model.ChallengeAbandon {
		return false, fmt.Errorf("challenge is abandoned")
	}

	if challenge.Status == model.ChallengeEnded {
		return false, fmt.Errorf("challenge has ended")
	}

	return true, nil
}

// CanCreate checks if a user can create a new challenge
// Returns false if user is already a creator or participant in any ongoing active challenge
func (r *RedisRepository) CanCreate(ctx context.Context, userID string) (bool, error) {
	if userID == "" {
		return false, fmt.Errorf("userID cannot be empty")
	}

	keys, err := r.client.Keys(ctx, "challenge:*").Result()
	if err != nil {
		return false, fmt.Errorf("failed to get challenge keys: %w", err)
	}

	if len(keys) == 0 {
		return true, nil
	}

	// Use goroutines for concurrent checking with early exit
	type result struct {
		canCreate   bool
		err         error
		challengeID string
	}

	resultChan := make(chan result, len(keys))
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()

	// Launch goroutines to check each challenge concurrently
	for _, key := range keys {
		go func(key string) {
			challengeID := key[10:] // Remove "challenge:" prefix

			data, err := r.client.Get(ctx, key).Result()
			if err != nil {
				if err == redis.Nil {
					resultChan <- result{canCreate: true, err: nil, challengeID: challengeID}
				} else {
					resultChan <- result{canCreate: true, err: nil, challengeID: challengeID} // Skip on error
				}
				return
			}

			var challenge model.ChallengeDocument
			if err := json.Unmarshal([]byte(data), &challenge); err != nil {
				resultChan <- result{canCreate: true, err: nil, challengeID: challengeID} // Skip malformed
				return
			}

			// Skip abandoned or ended challenges
			if challenge.Status == model.ChallengeAbandon || challenge.Status == model.ChallengeEnded {
				resultChan <- result{canCreate: true, err: nil, challengeID: challengeID}
				return
			}

			// Check if user is the creator of an ongoing challenge
			if challenge.CreatorID == userID {
				resultChan <- result{
					canCreate:   false,
					err:         fmt.Errorf("user is already the creator of an ongoing challenge: %s", challengeID),
					challengeID: challengeID,
				}
				return
			}

			// Check if user is a participant in an ongoing challenge
			if challenge.Participants != nil {
				if _, exists := challenge.Participants[userID]; exists {
					resultChan <- result{
						canCreate:   false,
						err:         fmt.Errorf("user is already a participant in an ongoing challenge: %s", challengeID),
						challengeID: challengeID,
					}
					return
				}
			}

			resultChan <- result{canCreate: true, err: nil, challengeID: challengeID}
		}(key)
	}

	// Collect results with early exit on first conflict
	for i := 0; i < len(keys); i++ {
		select {
		case res := <-resultChan:
			if !res.canCreate {
				cancel() // Cancel remaining goroutines
				return false, res.err
			}
		case <-ctx.Done():
			return false, ctx.Err()
		}
	}

	return true, nil
}
