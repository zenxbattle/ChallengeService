package broadcasts

import (
	"time"

	"github.com/gorilla/websocket"
	"github.com/lijuuu/ChallengeWssManagerService/internal/constants"
	"github.com/lijuuu/ChallengeWssManagerService/internal/model"
	wsstypes "github.com/lijuuu/ChallengeWssManagerService/internal/wss/types"
)

// SendStandardMessage sends a standardized message to a single WebSocket connection.
// This is the core function for sending any standard broadcast message.
func SendStandardMessage(conn *websocket.Conn, msgType string, payload any, success bool, errorMsg *string) error {
	message := wsstypes.StandardBroadcastMessage{
		Type:    msgType,
		Payload: payload,
		Success: success,
		Error:   errorMsg,
	}
	return SendJSON(conn, message)
}

// SendJSON sends a JSON message over a single WebSocket connection.
func SendJSON(conn *websocket.Conn, data interface{}) error {
	return conn.WriteJSON(data)
}

// BroadcastStandardMessage broadcasts a standardized message to all provided WebSocket clients.
// This is the core function for broadcasting any standard broadcast message.
func BroadcastStandardMessage(wsClients map[string]*websocket.Conn, msgType string, payload any, success bool, errorMsg *string) {
	message := wsstypes.StandardBroadcastMessage{
		Type:    msgType,
		Payload: payload,
		Success: success,
		Error:   errorMsg,
	}

	// Send to each connection sequentially to avoid concurrent write issues
	for _, conn := range wsClients {
		if conn == nil {
			continue // Skip nil connections
		}
		// Send synchronously to avoid concurrent writes to the same connection
		if err := SendJSON(conn, message); err != nil {
			// Log the error, or handle it as needed.
			// For example, if a write fails, it might indicate a closed connection
			// and you might want to remove it from the wsClients map.
			// log.Printf("Error sending message to client: %v", err)
		}
	}
}

// SendStandardError sends a standardized error message to a single WebSocket connection.
func SendStandardError(conn *websocket.Conn, msgType string, errorMsg string) error {
	errorPtr := &errorMsg
	return SendStandardMessage(conn, msgType, nil, false, errorPtr)
}

// BroadcastStandardError broadcasts a standardized error message to all WebSocket clients.
func BroadcastStandardError(wsClients map[string]*websocket.Conn, msgType string, errorMsg string) {
	errorPtr := &errorMsg
	BroadcastStandardMessage(wsClients, msgType, nil, false, errorPtr)
}

// SendStandardSuccess sends a standardized success message to a single WebSocket connection.
func SendStandardSuccess(conn *websocket.Conn, msgType string, payload any) error {
	return SendStandardMessage(conn, msgType, payload, true, nil)
}

// BroadcastStandardSuccess broadcasts a standardized success message to all WebSocket clients.
func BroadcastStandardSuccess(wsClients map[string]*websocket.Conn, msgType string, payload any) {
	BroadcastStandardMessage(wsClients, msgType, payload, true, nil)
}

// SendErrorWithType sends a standardized error message with additional payload details to a single WebSocket connection.
// The `extra` map is passed as the `Payload`.
func SendErrorWithType(conn *websocket.Conn, eventType string, msg string, extra map[string]any) error {
	errorMsg := msg
	return SendStandardMessage(conn, eventType, extra, false, &errorMsg)
}

// BroadcastEntityJoinedWithClients broadcasts a user/owner joined event to WebSocket clients.
func BroadcastEntityJoinedWithClients(wsClients map[string]*websocket.Conn, userID, challengeID string, isOwner bool) {
	eventType := constants.USER_JOINED
	if isOwner {
		eventType = constants.OWNER_JOINED
	}

	payload := map[string]any{
		"userId":      userID,
		"challengeId": challengeID,
		"time":        time.Now(),
	}

	BroadcastStandardMessage(wsClients, eventType, payload, true, nil)
}

// BroadcastEntityLeftWithClients broadcasts a user/owner left event to WebSocket clients.
func BroadcastEntityLeftWithClients(wsClients map[string]*websocket.Conn, userID, challengeID string, isOwner bool) {
	eventType := constants.USER_LEFT
	if isOwner {
		eventType = constants.OWNER_LEFT
	}

	payload := map[string]any{
		"userId":      userID,
		"challengeId": challengeID,
		"time":        time.Now(),
	}

	BroadcastStandardMessage(wsClients, eventType, payload, true, nil)
}

// BroadcastChallengeAbandonWithClients broadcasts a challenge abandon event to WebSocket clients.
func BroadcastChallengeAbandonWithClients(wsClients map[string]*websocket.Conn, challengeID, creatorID string) {
	payload := map[string]any{
		"challengeId": challengeID,
		"userId":      creatorID,
		"time":        time.Now(),
	}

	BroadcastStandardMessage(wsClients, constants.CREATOR_ABANDON, payload, true, nil)
}

// BroadcastNewSubmission broadcasts NEW_SUBMISSION event to WebSocket clients.
func BroadcastNewSubmission(wsClients map[string]*websocket.Conn, challengeID, userID, problemID string, score, newRank int) {
	payload := map[string]any{
		"challengeId": challengeID,
		"userId":      userID,
		"problemId":   problemID,
		"score":       score,
		"newRank":     newRank,
		"time":        time.Now(),
	}

	BroadcastStandardMessage(wsClients, constants.NEW_SUBMISSION, payload, true, nil)
}

// BroadcastLeaderboardUpdate broadcasts LEADERBOARD_UPDATE event to WebSocket clients.
func BroadcastLeaderboardUpdate(wsClients map[string]*websocket.Conn, challengeID string, leaderboard []*model.LeaderboardEntry, updatedUser string) {
	payload := map[string]any{
		"challengeId": challengeID,
		"leaderboard": leaderboard,
		"updatedUser": updatedUser,
		"time":        time.Now(),
	}

	BroadcastStandardMessage(wsClients, constants.LEADERBOARD_UPDATE, payload, true, nil)
}
