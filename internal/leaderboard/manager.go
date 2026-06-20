package leaderboard

import (
	"fmt"
	"sync"

	"github.com/lijuuu/ChallengeWssManagerService/internal/model"
	redisboard "github.com/lijuuu/RedisBoard"
)

// ParticipantLeaderboardData holds user rank information
type ParticipantLeaderboardData struct {
	UserID     string `json:"userId"`
	TotalScore int    `json:"totalScore"`
	GlobalRank int    `json:"globalRank"`
	Rank       int    `json:"rank"`
}

// LeaderboardManager manages multiple RedisBoard instances per challenge
type LeaderboardManager struct {
	Boards map[string]*redisboard.Leaderboard // challengeID -> RedisBoard instance
	Config *redisboard.Config
	MU     sync.RWMutex
}

// NewLeaderboardManager creates a new LeaderboardManager instance
func NewLeaderboardManager(redisAddr, redisPassword string) *LeaderboardManager {
	config := &redisboard.Config{
		K:           50,    // Track top 50 users
		MaxUsers:    10000, // Max users per challenge
		MaxEntities: 1,     // No entity grouping needed
		FloatScores: false, // Use integer scores
		RedisAddr:   redisAddr,
		RedisPass:   redisPassword,
	}

	return &LeaderboardManager{
		Boards: make(map[string]*redisboard.Leaderboard),
		Config: config,
	}
}

// InitializeLeaderboard creates a RedisBoard instance for a challenge
func (lm *LeaderboardManager) InitializeLeaderboard(challengeID string) error {
	lm.MU.Lock()
	defer lm.MU.Unlock()

	// Check if already initialized
	if _, exists := lm.Boards[challengeID]; exists {
		return nil
	}

	// Create challenge-specific config
	config := *lm.Config
	config.Namespace = fmt.Sprintf("challenge_%s", challengeID)

	// Create RedisBoard instance
	board, err := redisboard.New(config)
	if err != nil {
		return fmt.Errorf("failed to create leaderboard for challenge %s: %w", challengeID, err)
	}

	lm.Boards[challengeID] = board
	return nil
}

// CleanupLeaderboard properly closes RedisBoard instance for a challenge
func (lm *LeaderboardManager) CleanupLeaderboard(challengeID string) error {
	lm.MU.Lock()
	defer lm.MU.Unlock()

	board, exists := lm.Boards[challengeID]
	if !exists {
		return nil // Already cleaned up
	}

	// Close the RedisBoard instance
	err := board.Close()
	if err != nil {
		return fmt.Errorf("failed to close leaderboard for challenge %s: %w", challengeID, err)
	}

	// Remove from map
	delete(lm.Boards, challengeID)
	return nil
}

// getBoard safely retrieves a RedisBoard instance for a challenge
func (lm *LeaderboardManager) getBoard(challengeID string) (*redisboard.Leaderboard, error) {
	lm.MU.RLock()
	defer lm.MU.RUnlock()

	board, exists := lm.Boards[challengeID]
	if !exists {
		return nil, fmt.Errorf("leaderboard not initialized for challenge %s", challengeID)
	}

	return board, nil
}

// UpdateParticipantScore updates a participant's score using RedisBoard
func (lm *LeaderboardManager) UpdateParticipantScore(challengeID, userID string, points int) error {
	board, err := lm.getBoard(challengeID)
	if err != nil {
		return err
	}

	// Create user with score
	user := redisboard.User{
		ID:     userID,
		Entity: "", // No entity grouping for challenges
		Score:  float64(points),
	}

	// Add/update user score
	err = board.AddUser(user)
	if err != nil {
		return fmt.Errorf("failed to update score for user %s in challenge %s: %w", userID, challengeID, err)
	}

	return nil
}

// GetLeaderboard retrieves current leaderboard using RedisBoard's GetTopKGlobal
// and calculates problems completed for each participant using the challenge document.
// Ranking considers both total score and problems completed as specified in requirements.
func (lm *LeaderboardManager) GetLeaderboard(challengeID string, limit int, challengeDoc *model.ChallengeDocument) ([]*model.LeaderboardEntry, error) {
	board, err := lm.getBoard(challengeID)
	if err != nil {
		return nil, err
	}

	// Get top users from RedisBoard
	users, err := board.GetTopKGlobal()
	if err != nil {
		// Handle case where no users exist yet
		if err.Error() == "no users in global leaderboard" {
			return []*model.LeaderboardEntry{}, nil
		}
		return nil, fmt.Errorf("failed to get leaderboard for challenge %s: %w", challengeID, err)
	}

	// Convert RedisBoard users to LeaderboardEntry and calculate problems completed
	leaderboard := make([]*model.LeaderboardEntry, 0, len(users))
	for _, user := range users {
		// Calculate problems completed for this user using the challenge document
		problemsCompleted := lm.CalculateProblemsCompleted(challengeDoc, user.ID)

		entry := &model.LeaderboardEntry{
			UserID:            user.ID,
			ProblemsCompleted: problemsCompleted,
			TotalScore:        int(user.Score),
			Rank:              0, // Will be calculated after sorting
		}
		leaderboard = append(leaderboard, entry)
	}

	// Sort leaderboard considering both score and problems completed
	// Primary sort: Total Score (descending)
	// Secondary sort: Problems Completed (descending)
	// Tertiary sort: UserID (ascending) for consistent ordering
	for i := 0; i < len(leaderboard); i++ {
		for j := i + 1; j < len(leaderboard); j++ {
			shouldSwap := false

			// Primary comparison: Total Score (higher is better)
			if leaderboard[i].TotalScore < leaderboard[j].TotalScore {
				shouldSwap = true
			} else if leaderboard[i].TotalScore == leaderboard[j].TotalScore {
				// Secondary comparison: Problems Completed (higher is better)
				if leaderboard[i].ProblemsCompleted < leaderboard[j].ProblemsCompleted {
					shouldSwap = true
				} else if leaderboard[i].ProblemsCompleted == leaderboard[j].ProblemsCompleted {
					// Tertiary comparison: UserID (lexicographic for consistency)
					if leaderboard[i].UserID > leaderboard[j].UserID {
						shouldSwap = true
					}
				}
			}

			if shouldSwap {
				leaderboard[i], leaderboard[j] = leaderboard[j], leaderboard[i]
			}
		}
	}

	// Assign ranks after sorting
	for i := range leaderboard {
		leaderboard[i].Rank = i + 1 // 1-based ranking
	}

	// Apply limit if specified
	if limit > 0 && len(leaderboard) > limit {
		leaderboard = leaderboard[:limit]
	}

	return leaderboard, nil
}

// GetParticipantRank gets a participant's rank and data using RedisBoard
func (lm *LeaderboardManager) GetParticipantRank(challengeID, userID string) (*ParticipantLeaderboardData, error) {
	board, err := lm.getBoard(challengeID)
	if err != nil {
		return nil, err
	}

	// Get user's leaderboard data from RedisBoard
	data, err := board.GetUserLeaderboardData(userID)
	if err != nil {
		return nil, fmt.Errorf("failed to get rank for user %s in challenge %s: %w", userID, challengeID, err)
	}

	// Convert to our format
	participantData := &ParticipantLeaderboardData{
		UserID:     userID,
		TotalScore: int(data.Score),
		GlobalRank: data.GlobalRank + 1, // Convert to 1-based ranking
		Rank:       data.GlobalRank + 1, // Alias for compatibility
	}

	// Handle case where user is not found (rank -1)
	if data.GlobalRank == -1 {
		participantData.GlobalRank = -1
		participantData.Rank = -1
	}

	return participantData, nil
}

// CalculateProblemsCompleted calculates the number of unique problems solved by a participant
// from the challenge document participant data. Each problem is only counted once per participant.
func (lm *LeaderboardManager) CalculateProblemsCompleted(challengeDoc *model.ChallengeDocument, userID string) int {
	// Validate input parameters
	if challengeDoc == nil {
		return 0
	}

	if userID == "" {
		return 0
	}

	// Check if participants map exists
	if challengeDoc.Participants == nil {
		return 0
	}

	// Get participant data for the specified user
	participant, exists := challengeDoc.Participants[userID]
	if !exists {
		return 0
	}

	// Check if participant has problems done
	if participant.ProblemsDone == nil {
		return 0
	}

	// Count unique problems solved
	// Since ProblemsDone is a map[string]ChallengeProblemMetadata where the key is problemID,
	// each key represents a unique problem that has been solved by the participant.
	// We only count problems that have a score > 0 to ensure they were actually solved successfully.
	problemsCompleted := 0
	for _, problemMeta := range participant.ProblemsDone {
		// Only count problems that were solved successfully (score > 0)
		if problemMeta.Score > 0 {
			problemsCompleted++
		}
	}

	return problemsCompleted
}
