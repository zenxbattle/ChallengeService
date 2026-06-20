package shared

import (
	"github.com/lijuuu/ChallengeWssManagerService/internal/model"
)

// WebSocket event payload structs
type LeaderboardUpdateEvent struct {
	Type        string                    `json:"type"`
	ChallengeID string                    `json:"challengeId"`
	Leaderboard []*model.LeaderboardEntry `json:"leaderboard"`
	UpdatedUser string                    `json:"updatedUser"`
}

type NewSubmissionEvent struct {
	Type        string `json:"type"`
	ChallengeID string `json:"challengeId"`
	UserID      string `json:"userId"`
	ProblemID   string `json:"problemId"`
	Points      int    `json:"points"`
	NewRank     int    `json:"newRank"`
}

type CurrentLeaderboardEvent struct {
	Type        string                    `json:"type"`
	ChallengeID string                    `json:"challengeId"`
	Leaderboard []*model.LeaderboardEntry `json:"leaderboard"`
}
