package model

import "encoding/json"

type EventType string

type Event struct {
	Type    EventType   `json:"type"`
	Payload interface{} `json:"payload"`
}

type ChallengeCreatedPayload struct {
	ChallengeID string `json:"challenge_id"`
	Title       string `json:"title"`
}

type ChallengeDeletedPayload struct {
	ChallengeID string `json:"challenge_id"`
}

type UserJoinedPayload struct {
	UserID string `json:"user_id"`
}

type UserLeftPayload struct {
	UserID string `json:"user_id"`
}

type UserForfeitedPayload struct {
	UserID string `json:"user_id"`
}

type ProblemSubmittedPayload struct {
	UserID    string `json:"user_id"`
	ProblemID string `json:"problem_id"`
	Score     int    `json:"score"`
}

type LeaderboardUpdatedPayload struct {
	Leaderboard []*LeaderboardEntry `json:"leaderboard"`
}

type ChallengeStatusChangedPayload struct {
	ChallengeID string `json:"challenge_id"`
	Status      string `json:"status"`
}

type TimeUpdatePayload struct {
	RemainingTime int64 `json:"remaining_time"` // In seconds
}

type ErrorPayload struct {
	Message string `json:"message"`
}

type WebSocketMessage struct {
	Type    string          `json:"type"`
	Payload json.RawMessage `json:"payload"`
}

type GenericResponse struct {
	Success bool                   `json:"success"`
	Status  int                    `json:"status"`
	Payload map[string]interface{} `json:"payload,omitempty"`
	Error   *ErrorInfo             `json:"error,omitempty"`
}

type ErrorInfo struct {
	ErrorType string `json:"type"`
	Code      int    `json:"code"`
	Message   string `json:"message"`
	Details   string `json:"details"`
}
