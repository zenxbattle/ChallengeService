package global

import (
	"github.com/lijuuu/ChallengeWssManagerService/internal/jwt"
	"github.com/lijuuu/ChallengeWssManagerService/internal/leaderboard"
	localstate "github.com/lijuuu/ChallengeWssManagerService/internal/local"
	"github.com/lijuuu/ChallengeWssManagerService/internal/repo"
	"github.com/panjf2000/ants/v2"
)

// State holds the application state shared across WebSocket and service layers
type State struct {
	Redis              *repo.RedisRepository
	Mongo              *repo.MongoRepository
	LocalState         *localstate.LocalStateManager
	LeaderboardManager *leaderboard.LeaderboardManager
	JwtManager         *jwt.JWTManager
	AntsWorkerPool     *ants.Pool
}
