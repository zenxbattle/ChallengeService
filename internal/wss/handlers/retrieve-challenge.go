package wsshandler

import (
	"context"
	"encoding/json"
	"log"

	"github.com/google/uuid"
	"github.com/lijuuu/ChallengeWssManagerService/internal/wss/broadcasts"
	wsstypes "github.com/lijuuu/ChallengeWssManagerService/internal/wss/types"
)

func RetreiveChallenge(ctx *wsstypes.WsContext) error {
	requestID := uuid.New().String()

	var payload wsstypes.RetreiveChallengePayload
	raw, err := json.Marshal(ctx.Payload)
	if err != nil {
		log.Printf("[%s] [RetreiveChallenge] Marshal error: %v", requestID, err)
		return broadcasts.SendJSON(ctx.Conn, map[string]interface{}{
			"type":   wsstypes.RETRIEVE_CHALLENGE,
			"status": "error",
			"error": map[string]interface{}{
				"code":    "MARSHAL_ERROR",
				"message": "Internal server error",
			},
		})
	}

	if err := json.Unmarshal(raw, &payload); err != nil {
		log.Printf("[%s] [RetreiveChallenge] Unmarshal error: %v", requestID, err)
		return broadcasts.SendJSON(ctx.Conn, map[string]interface{}{
			"type":   wsstypes.RETRIEVE_CHALLENGE,
			"status": "error",
			"error": map[string]interface{}{
				"code":    "INVALID_PAYLOAD",
				"message": "Payload format invalid",
			},
		})
	}

	// Load challenge from Redis
	challengeDoc, err := ctx.State.Redis.GetChallengeByID(context.Background(), payload.ChallengeId)
	if err != nil {
		log.Printf("[%s] [RetreiveChallenge] Challenge %s not found in Redis: %v", requestID, payload.ChallengeId, err)
		return broadcasts.SendJSON(ctx.Conn, map[string]interface{}{
			"type":   wsstypes.RETRIEVE_CHALLENGE,
			"status": "error",
			"error": map[string]interface{}{
				"code":    "CHALLENGE_NOT_FOUND",
				"message": "Challenge not found or not joined",
			},
		})
	}

	// Check if user has WebSocket connection (is joined)
	_, hasConnection := ctx.State.LocalState.GetWSClient(payload.ChallengeId, payload.UserId)
	if !hasConnection {
		log.Printf("[%s] [RetreiveChallenge] User %s not connected to challenge %s", requestID, payload.UserId, payload.ChallengeId)
		return broadcasts.SendJSON(ctx.Conn, map[string]interface{}{
			"type":   wsstypes.RETRIEVE_CHALLENGE,
			"status": "error",
			"error": map[string]interface{}{
				"code":    "NOT_JOINED",
				"message": "User not joined to this challenge",
			},
		})
	}

	log.Printf("[%s] [RetreiveChallenge] Sending latest challenge state to user %s", requestID, payload.UserId)

	if err := ctx.State.LeaderboardManager.InitializeLeaderboard(payload.ChallengeId); err != nil {
		log.Printf("[%s] [RetreiveChallenge] Failed to initialize leaderboard: %v", requestID, err)
	} else {
		leaderboard, err := ctx.State.LeaderboardManager.GetLeaderboard(payload.ChallengeId, 50, &challengeDoc)
		if err != nil {
			log.Printf("[%s] [RetreiveChallenge] Failed to fetch leaderboard snapshot: %v", requestID, err)
		} else {
			challengeDoc.Leaderboard = leaderboard
		}
	}

	return broadcasts.SendJSON(ctx.Conn, map[string]interface{}{
		"type":    wsstypes.RETRIEVE_CHALLENGE,
		"status":  "ok",
		"message": "Challenge state fetched successfully",
		"payload": map[string]interface{}{
			"userId":      payload.UserId,
			"challengeId": payload.ChallengeId,
			"challenge":   challengeDoc,
		},
	})
}
