package model

import (
	"time"

	"github.com/lijuuu/ChallengeWssManagerService/internal/constants"
)

const (
	SessionTimeout        = 30 * time.Minute
	CleanupInterval       = 5 * time.Minute
	MaxConcurrentMatches  = 100
	WebsocketReadTimeout  = 60 * time.Second
	EmptyChallengeTimeout = 10 * time.Minute
)

const (
	ChallengeOpen      = constants.CHALLENGE_OPEN
	ChallengeStarted   = constants.CHALLENGE_STARTED
	ChallengeForfieted = constants.CHALLENGE_FORFEITED
	ChallengeEnded     = constants.CHALLENGE_ENDED
	ChallengeAbandon   = constants.CHALLENGE_ABANDON
)

type QuestionDifficulty string

const (
	DifficultyEasy   QuestionDifficulty = "easy"
	DifficultyMedium QuestionDifficulty = "medium"
	DifficultyHard   QuestionDifficulty = "hard"
)

type Session struct {
	UserID      string `json:"userId"`
	ChallengeID string `json:"challengeId"`
	LastActive  int64  `json:"lastActive"`
	SessionHash string `json:"sessionHash"`
}

type ChallengeConfig struct {
	MaxUsers           int `json:"maxUsers"`
	MaxEasyQuestions   int `json:"maxEasyQuestions"`
	MaxMediumQuestions int `json:"maxMediumQuestions"`
	MaxHardQuestions   int `json:"maxHardQuestions"`
}

type ChallengeDocument struct {
	ChallengeID         string                           `bson:"challengeId" json:"challengeId"`
	CreatorID           string                           `bson:"creatorId" json:"creatorId"`
	CreatedAt           int64                            `bson:"createdAt" json:"createdAt"`
	Title               string                           `bson:"title" json:"title"`
	IsPrivate           bool                             `bson:"isPrivate" json:"isPrivate"`
	Password            string                           `bson:"password" json:"password"`
	Status              string                           `bson:"status" json:"status"`
	TimeLimit           int64                            `bson:"timeLimit" json:"timeLimit"`
	StartTime           int64                            `bson:"startTime" json:"startTime"`
	Participants        map[string]*ParticipantMetadata  `bson:"participants" json:"participants"`
	Submissions         map[string]map[string]Submission `bson:"submissions" json:"submissions"`
	Leaderboard         []*LeaderboardEntry              `bson:"leaderboard" json:"leaderboard"`
	Config              *ChallengeConfig                 `bson:"config" json:"config"`
	ProcessedProblemIds []string                         `bson:"processedProblemIds" json:"processedProblemIds"`
	ProblemCount        int64                            `bson:"problemCount" json:"problemCount"`
}

type Submission struct {
	SubmissionID string        `json:"submissionId"`
	TimeTaken    time.Duration `json:"timeTaken"` // ms
	Points       int           `json:"points"`
	UserCode     string        `json:"userCode"`
}

type ParticipantMetadata struct {
	ProblemsDone      map[string]ChallengeProblemMetadata `json:"problemsDone"`
	ProblemsAttempted int                                 `json:"problemsAttempted"`
	TotalScore        int                                 `json:"totalScore"`
	JoinTime          int64                               `json:"joinTime"`
	LastConnected     int64                               `json:"lastConnected"`
	InitialJoinIP     string                              `json:"initialJoinIp"`
	Status            string                              `json:"status"`
}

type ChallengeProblemMetadata struct {
	ProblemID   string `json:"problemId"`
	Score       int    `json:"score"`
	TimeTaken   int64  `json:"timeTaken"`
	CompletedAt int64  `json:"completedAt"`
}

type LeaderboardEntry struct {
	UserID            string `json:"userId"`
	ProblemsCompleted int    `json:"problemsCompleted"`
	TotalScore        int    `json:"totalScore"`
	Rank              int    `json:"rank"`
}
