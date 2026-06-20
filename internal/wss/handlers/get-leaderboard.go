package wsshandler

import (
	"context"
	"encoding/json"
	"log"

	"github.com/google/uuid"
	"github.com/lijuuu/ChallengeWssManagerService/internal/constants"
	"github.com/lijuuu/ChallengeWssManagerService/internal/wss/broadcasts"
	wsstypes "github.com/lijuuu/ChallengeWssManagerService/internal/wss/types"
)

type GetLeaderboardPayload struct {
	UserId      string `json:"userId"`
	Type        string `json:"type"`
	ChallengeId string `json:"challengeId"`
	Limit       int    `json:"limit,omitempty"` // Optional limit, defaults to 100
}

func GetLeaderboardHandler(ctx *wsstypes.WsContext) error {
	requestID := uuid.New().String()

	var payload GetLeaderboardPayload
	raw, err := json.Marshal(ctx.Payload)
	if err != nil {
		log.Printf("[%s] [GetLeaderboard] Marshal error: %v", requestID, err)
		return broadcasts.SendErrorWithType(ctx.Conn, constants.CURRENT_LEADERBOARD, "Internal error", nil)
	}
	if err := json.Unmarshal(raw, &payload); err != nil {
		log.Printf("[%s] [GetLeaderboard] Unmarshal error: %v", requestID, err)
		return broadcasts.SendErrorWithType(ctx.Conn, constants.CURRENT_LEADERBOARD, "Invalid payload format", nil)
	}

	log.Printf("[%s] [GetLeaderboard] Request from userId %s for challenge %s", requestID, payload.UserId, payload.ChallengeId)

	// Validate required fields
	if payload.ChallengeId == "" {
		log.Printf("[%s] [GetLeaderboard] Missing challengeId", requestID)
		return broadcasts.SendErrorWithType(ctx.Conn, constants.CURRENT_LEADERBOARD, "Challenge ID is required", nil)
	}

	// Set default limit if not provided
	limit := payload.Limit
	if limit <= 0 {
		limit = 100
	}

	// Verify challenge exists in Redis
	challengeDoc, err := ctx.State.Redis.GetChallengeByID(context.Background(), payload.ChallengeId)
	if err != nil {
		log.Printf("[%s] [GetLeaderboard] Challenge not found in Redis: %v", requestID, err)
		return broadcasts.SendErrorWithType(ctx.Conn, constants.CURRENT_LEADERBOARD, "Challenge not found", nil)
	}

	// Check if user is participant (optional validation)
	if payload.UserId != "" {
		if _, exists := challengeDoc.Participants[payload.UserId]; !exists {
			log.Printf("[%s] [GetLeaderboard] User %s is not a participant in challenge %s", requestID, payload.UserId, payload.ChallengeId)
			return broadcasts.SendErrorWithType(ctx.Conn, constants.CURRENT_LEADERBOARD, "User is not a participant in this challenge", nil)
		}
	}

	// Get current leaderboard using the injected service
	leaderboard, err := ctx.State.LeaderboardManager.GetLeaderboard(payload.ChallengeId, limit, &challengeDoc)
	if err != nil {
		log.Printf("[%s] [GetLeaderboard] Failed to get leaderboard: %v", requestID, err)
		return broadcasts.SendErrorWithType(ctx.Conn, constants.CURRENT_LEADERBOARD, "Failed to retrieve leaderboard", nil)
	}

	// Create response payload
	response := map[string]interface{}{
		"type":        constants.CURRENT_LEADERBOARD,
		"challengeId": payload.ChallengeId,
		"leaderboard": leaderboard,
	}

	log.Printf("[%s] [GetLeaderboard] Sending leaderboard with %d entries", requestID, len(leaderboard))

	return broadcasts.SendJSON(ctx.Conn, response)
}
