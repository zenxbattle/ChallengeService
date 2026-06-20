package wsshandler

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"time"

	"github.com/google/uuid"
	"github.com/lijuuu/ChallengeWssManagerService/internal/config"
	"github.com/lijuuu/ChallengeWssManagerService/internal/constants"
	"github.com/lijuuu/ChallengeWssManagerService/internal/model"
	"github.com/lijuuu/ChallengeWssManagerService/internal/wss/broadcasts"
	wsstypes "github.com/lijuuu/ChallengeWssManagerService/internal/wss/types"
)

type AuthPayload struct {
	UserID string `json:"userId"`
}

func JoinChallengeHandler(ctx *wsstypes.WsContext) error {
	requestID := uuid.New().String()
	clientIP := ctx.Conn.RemoteAddr().String()

	var payload wsstypes.JoinChallengePayload
	raw, err := json.Marshal(ctx.Payload)
	if err != nil {
		log.Printf("[%s] [JoinChallenge] Marshal error: %v", requestID, err)
		return broadcasts.SendErrorWithType(ctx.Conn, wsstypes.JOIN_CHALLENGE, "Internal error", nil)
	}
	if err := json.Unmarshal(raw, &payload); err != nil {
		log.Printf("[%s] [JoinChallenge] Unmarshal error: %v", requestID, err)
		return broadcasts.SendErrorWithType(ctx.Conn, wsstypes.JOIN_CHALLENGE, "Invalid payload format", nil)
	}
	log.Printf("[%s] [JoinChallenge] Incoming request from userId %s IP: %s", requestID, payload.UserId, clientIP)

	fmt.Println("payload ", payload)

	// auth
	startAuth := time.Now()
	req, err := http.NewRequestWithContext(context.Background(), "GET", config.LoadConfig().APIGatewayTokenCheckURL, nil)
	if err != nil {
		log.Printf("[%s] [JoinChallenge] Auth request create fail: %v", requestID, err)
		return broadcasts.SendErrorWithType(ctx.Conn, wsstypes.JOIN_CHALLENGE, "Internal auth setup error", nil)
	}
	req.Header.Set("Authorization", payload.Token)

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		log.Printf("[%s] [JoinChallenge] Auth request failed: %v", requestID, err)
		return broadcasts.SendErrorWithType(ctx.Conn, wsstypes.JOIN_CHALLENGE, "Authentication service unreachable", nil)
	}
	defer resp.Body.Close()

	log.Printf("[%s] [JoinChallenge] Auth status: %d (took %v)", requestID, resp.StatusCode, time.Since(startAuth))

	var authResp model.GenericResponse
	if err := json.NewDecoder(resp.Body).Decode(&authResp); err != nil {
		log.Printf("[%s] [JoinChallenge] Decode auth response failed: %v", requestID, err)
		return broadcasts.SendErrorWithType(ctx.Conn, wsstypes.JOIN_CHALLENGE, "Failed to decode authentication", nil)
	}
	if !authResp.Success {
		log.Printf("[%s] [JoinChallenge] Auth failed: %v", requestID, authResp.Error)
		return broadcasts.SendErrorWithType(ctx.Conn, wsstypes.JOIN_CHALLENGE, "Authentication failed", map[string]interface{}{
			"error": authResp.Error,
		})
	}

	var userData AuthPayload
	authPayloadRaw, _ := json.Marshal(authResp.Payload)
	if err := json.Unmarshal(authPayloadRaw, &userData); err != nil {
		log.Printf("[%s] [JoinChallenge] Invalid auth payload structure: %v", requestID, err)
		return broadcasts.SendErrorWithType(ctx.Conn, wsstypes.JOIN_CHALLENGE, "Invalid auth data", nil)
	}

	log.Printf("[%s] [JoinChallenge] Authenticated user ID: %s", requestID, userData.UserID)

	// Load challenge from Redis
	startRepoCheck := time.Now()
	challengeDoc, err := ctx.State.Redis.GetChallengeByID(context.Background(), payload.ChallengeId)
	if err != nil {
		log.Printf("[%s] [JoinChallenge] Challenge not found in Redis: %v", requestID, err)
		return broadcasts.SendErrorWithType(ctx.Conn, wsstypes.JOIN_CHALLENGE, "Challenge not found", nil)
	}

	if challengeDoc.Status == model.ChallengeAbandon {
		return broadcasts.SendErrorWithType(ctx.Conn, wsstypes.JOIN_CHALLENGE, "Challenge is abandoned", nil)
	}

	// Check access (simplified - checking password if private)
	if challengeDoc.IsPrivate && challengeDoc.Password != payload.Password {
		log.Printf("[%s] [JoinChallenge] Access denied to challenge %s", requestID, payload.ChallengeId)
		return broadcasts.SendErrorWithType(ctx.Conn, wsstypes.JOIN_CHALLENGE, "Invalid challenge ID or password", nil)
	}

	log.Printf("[%s] [JoinChallenge] Access granted for challenge %s (took %v)", requestID, payload.ChallengeId, time.Since(startRepoCheck))

	// Add/update participant in Redis
	participant, exists := challengeDoc.Participants[userData.UserID]
	if !exists {
		participant = &model.ParticipantMetadata{
			ProblemsDone:  make(map[string]model.ChallengeProblemMetadata),
			JoinTime:      time.Now().Unix(),
			InitialJoinIP: clientIP,
		}
		err := ctx.State.Redis.AddParticipant(context.Background(), payload.ChallengeId, userData.UserID, participant)
		if err != nil {
			log.Printf("[%s] [JoinChallenge] Failed to persist participant: %v", requestID, err)
		}
		log.Printf("[%s] [JoinChallenge] New participant %s added", requestID, userData.UserID)
	} else {
		log.Printf("[%s] [JoinChallenge] Participant %s rejoined", requestID, userData.UserID)
	}
	participant.LastConnected = time.Now().Unix()
	challengeDoc.Participants[userData.UserID] = participant

	// Update participant in Redis
	err = ctx.State.Redis.UpdateParticipant(context.Background(), payload.ChallengeId, userData.UserID, participant)
	if err != nil {
		log.Printf("[%s] [JoinChallenge] Failed to update participant: %v", requestID, err)
	}

	if err := ctx.State.LeaderboardManager.InitializeLeaderboard(payload.ChallengeId); err != nil {
		log.Printf("[%s] [JoinChallenge] Failed to initialize leaderboard: %v", requestID, err)
	}

	if err := ctx.State.LeaderboardManager.UpdateParticipantScore(payload.ChallengeId, userData.UserID, participant.TotalScore); err != nil {
		log.Printf("[%s] [JoinChallenge] Failed to add participant to leaderboard: %v", requestID, err)
	}

	leaderboard, err := ctx.State.LeaderboardManager.GetLeaderboard(payload.ChallengeId, 50, &challengeDoc)
	if err != nil {
		log.Printf("[%s] [JoinChallenge] Failed to fetch leaderboard snapshot: %v", requestID, err)
	} else {
		challengeDoc.Leaderboard = leaderboard
		if err := ctx.State.Redis.UpdateChallenge(context.Background(), &challengeDoc); err != nil {
			log.Printf("[%s] [JoinChallenge] Failed to persist challenge snapshot: %v", requestID, err)
		}
	}

	// Add WebSocket connection to local state
	ctx.State.LocalState.AddWSClient(payload.ChallengeId, userData.UserID, ctx.Conn)

	// Get all WebSocket clients for broadcasting
	wsClients := ctx.State.LocalState.GetAllWSClients(payload.ChallengeId)
	broadcasts.BroadcastEntityJoinedWithClients(wsClients, userData.UserID, payload.ChallengeId, userData.UserID == challengeDoc.CreatorID)

	challengeToken, _ := ctx.State.JwtManager.GenerateToken(
		userData.UserID,
		payload.ChallengeId,
		time.Duration(challengeDoc.TimeLimit)*time.Millisecond+constants.BufferTime,
	)

	return broadcasts.SendJSON(ctx.Conn, map[string]interface{}{
		"type":    wsstypes.JOIN_CHALLENGE,
		"status":  "success",
		"message": "Joined challenge successfully",
		"payload": map[string]interface{}{
			"userId":         userData.UserID,
			"challengeId":    payload.ChallengeId,
			"challenge":      challengeDoc,
			"challengeToken": challengeToken,
		},
	})
}
