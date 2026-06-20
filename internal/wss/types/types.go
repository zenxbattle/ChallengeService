package wsstypes

import (
	"time"

	"github.com/gorilla/websocket"
	"github.com/lijuuu/ChallengeWssManagerService/internal/constants"
	"github.com/lijuuu/ChallengeWssManagerService/internal/global"
	"github.com/lijuuu/ChallengeWssManagerService/internal/jwt"
	"github.com/lijuuu/ChallengeWssManagerService/internal/model"
)

type WsContext struct {
	Conn      *websocket.Conn
	Payload   map[string]any
	UserID    string
	EventType string
	State     *global.State
	Claims    *jwt.CustomClaims
}

type WsMessageRequest struct {
	Type    string         `json:"type"`
	Payload map[string]any `json:"payload"`
}

// StandardBroadcastMessage provides a consistent format for all WebSocket broadcasts
type StandardBroadcastMessage struct {
	Type    string  `json:"type"`
	Payload any     `json:"payload"`
	Success bool    `json:"success"`
	Error   *string `json:"error"`
}

type JoinChallengePayload struct {
	UserId      string `json:"userId"`
	Type        string `json:"type"`
	ChallengeId string `json:"challengeId"`
	Password    string `json:"password"`
	Token       string `json:"token"`
}

type RetreiveChallengePayload struct {
	UserId      string `json:"userId"`
	Type        string `json:"type"`
	ChallengeId string `json:"challengeId"`
}

type GenericResponse struct {
	Success bool           `json:"success"`
	Status  int            `json:"status"`
	Payload map[string]any `json:"payload"`
	Error   *ErrorInfo     `json:"error"`
}

type ErrorInfo struct {
	ErrorType string `json:"errorType"`
	Code      int    `json:"code"`
	Message   string `json:"message"`
	Details   string `json:"details"`
}

// Enhanced payload types that include challenge document data directly

// LeaderboardUpdatePayload contains leaderboard data with full challenge context
type LeaderboardUpdatePayload struct {
	ChallengeID string                    `json:"challengeId"`
	Leaderboard []*model.LeaderboardEntry `json:"leaderboard"`
	UpdatedUser string                    `json:"updatedUser"`
	Time        time.Time                 `json:"time"`
}

// NewSubmissionPayload contains submission data with full challenge context
type NewSubmissionPayload struct {
	ChallengeID string    `json:"challengeId"`
	UserID      string    `json:"userId"`
	ProblemID   string    `json:"problemId"`
	Score       int       `json:"score"`
	NewRank     int       `json:"newRank"`
	Time        time.Time `json:"time"`
}

const (
	PING_SERVER = constants.PING_SERVER

	USER_JOINED       = constants.USER_JOINED
	USER_LEFT         = constants.USER_LEFT
	CREATOR_ABANDON   = constants.CREATOR_ABANDON
	CHALLENGE_STARTED = constants.CHALLENGE_STARTED
	OWNER_LEFT        = constants.OWNER_LEFT
	OWNER_JOINED      = constants.OWNER_JOINED

	NEW_OWNER_ASSIGNED = constants.NEW_OWNER_ASSIGNED

	JOIN_CHALLENGE      = constants.JOIN_CHALLENGE
	RETRIEVE_CHALLENGE  = constants.RETRIEVE_CHALLENGE
	CURRENT_LEADERBOARD = constants.CURRENT_LEADERBOARD
	LEADERBOARD_UPDATE  = constants.LEADERBOARD_UPDATE
	NEW_SUBMISSION      = constants.NEW_SUBMISSION
)
