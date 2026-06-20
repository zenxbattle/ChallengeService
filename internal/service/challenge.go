package service

import (
	"context"
	"errors"
	"fmt"
	"log"
	"sync"
	"time"

	"github.com/lijuuu/ChallengeWssManagerService/internal/global"
	"github.com/lijuuu/ChallengeWssManagerService/internal/model"
	"github.com/lijuuu/ChallengeWssManagerService/internal/utils"
	"github.com/lijuuu/ChallengeWssManagerService/internal/wss/broadcasts"
	challengePb "github.com/lijuuu/GlobalProtoXcode/ChallengeService"
)

type ChallengeService struct {
	GlobalState *global.State
	challengePb.UnimplementedChallengeServiceServer
}

func NewChallengeService(GlobalState *global.State) *ChallengeService {
	return &ChallengeService{
		GlobalState: GlobalState,
	}
}

func (s *ChallengeService) fetchChallengesConcurrently(ctx context.Context, challengeIDs []string, isPrivate bool) []model.ChallengeDocument {
	var challenges []model.ChallengeDocument
	var mu sync.Mutex
	var wg sync.WaitGroup

	for _, challengeID := range challengeIDs {
		wg.Add(1)
		_ = s.GlobalState.AntsWorkerPool.Submit(func() {
			defer wg.Done()
			challenge, err := s.GlobalState.Redis.GetChallengeByID(ctx, challengeID)
			if err == nil {
				mu.Lock()
				if challenge.IsPrivate == isPrivate {
					challenges = append(challenges, challenge)
				}
				mu.Unlock()
			}
		})
	}

	wg.Wait()
	return challenges
}

func (s *ChallengeService) CreateChallenge(ctx context.Context, req *challengePb.ChallengeRecord) (*challengePb.ChallengeRecord, error) {

	// Check for active challenges using Redis
	canCreate, err := s.GlobalState.Redis.CanCreate(ctx, req.CreatorId)
	if err != nil {
		return nil, err
	}
	if !canCreate {
		return nil, errors.New("active challenge already found, can't create new challenge")
	}

	modelChallengeDoc := ChallengeDocumentFromProto(req, false)

	// Initialize challenge document for Redis storage
	modelChallengeDoc.Status = model.ChallengeOpen
	modelChallengeDoc.Participants = make(map[string]*model.ParticipantMetadata)
	modelChallengeDoc.Submissions = make(map[string]map[string]model.Submission)
	modelChallengeDoc.Leaderboard = make([]*model.LeaderboardEntry, 0)

	modelChallengeDoc.Participants[modelChallengeDoc.CreatorID] = &model.ParticipantMetadata{
		JoinTime: time.Now().Unix(),
	}

	if req.IsPrivate {
		modelChallengeDoc.Password = utils.GenerateBigCapPassword(7)
	}

	modelChallengeDoc.Leaderboard = append(modelChallengeDoc.Leaderboard, &model.LeaderboardEntry{
		UserID:            req.CreatorId,
		TotalScore:        0,
		Rank:              0,
		ProblemsCompleted: 0,
	})

	modelChallengeDoc.Config = &model.ChallengeConfig{
		MaxEasyQuestions:   int(req.GetConfig().GetMaxEasyQuestions()),
		MaxMediumQuestions: int(req.GetConfig().GetMaxMediumQuestions()),
		MaxHardQuestions:   int(req.GetConfig().GetMaxHardQuestions()),
		MaxUsers:           int(req.GetConfig().MaxUsers),
	}

	// Create challenge in Redis only
	if err := s.GlobalState.Redis.CreateChallenge(ctx, modelChallengeDoc); err != nil {
		return nil, err
	}

	// Initialize leaderboard for the new challenge
	if err := s.GlobalState.LeaderboardManager.InitializeLeaderboard(modelChallengeDoc.ChallengeID); err != nil {
		log.Printf("[CreateChallenge] Warning: Failed to initialize leaderboard for challenge %s: %v", modelChallengeDoc.ChallengeID, err)
		// Don't fail challenge creation if leaderboard initialization fails
	} else {
		// Add creator to leaderboard with initial score of 0
		if err := s.GlobalState.LeaderboardManager.UpdateParticipantScore(modelChallengeDoc.ChallengeID, modelChallengeDoc.CreatorID, 0); err != nil {
			log.Printf("[CreateChallenge] Warning: Failed to add creator to leaderboard for challenge %s: %v", modelChallengeDoc.ChallengeID, err)
		}
	}

	return ChallengesToProto([]*model.ChallengeDocument{modelChallengeDoc}, false)[0], nil
}

func (s *ChallengeService) LeaveChallenge(ctx context.Context, challengeId, userId string) bool {
	// Fetch the challenge to verify the creator using Redis repository
	challenge, err := s.GlobalState.Redis.GetChallengeByID(ctx, challengeId)
	if err != nil {
		return false
	}

	if challenge.CreatorID != userId {
		return false
	}

	if err := s.GlobalState.Redis.RemoveParticipantInJoinPhase(ctx, challengeId, userId); err != nil {
		return false
	}

	return true
}

func (s *ChallengeService) AbandonChallenge(ctx context.Context, req *challengePb.AbandonChallengeRequest) (*challengePb.AbandonChallengeResponse, error) {
	// Fetch the challenge to verify the creator using Redis repository
	challenge, err := s.GlobalState.Redis.GetChallengeByID(ctx, req.ChallengeId)
	if err != nil {
		return &challengePb.AbandonChallengeResponse{
			Success:   false,
			Message:   "Challenge not found",
			ErrorType: "CHALLENGENOTFOUND",
		}, err
	}

	if challenge.CreatorID != req.CreatorId {
		return &challengePb.AbandonChallengeResponse{
			Success:   false,
			Message:   "Only the creator can abandon the challenge",
			ErrorType: "NOTCREATOR",
		}, nil
	}

	if err := s.GlobalState.Redis.AbandonChallenge(ctx, req.CreatorId, req.ChallengeId); err != nil {
		return &challengePb.AbandonChallengeResponse{
			Success:   false,
			Message:   err.Error(),
			ErrorType: "CHALLENGEABANDONFAILED",
		}, err
	}

	// Clean up leaderboard for abandoned challenge
	if err := s.GlobalState.LeaderboardManager.CleanupLeaderboard(req.ChallengeId); err != nil {
		log.Printf("[AbandonChallenge] Warning: Failed to cleanup leaderboard for challenge %s: %v", req.ChallengeId, err)
	}

	// Trigger MongoDB persistence for ABANDONED challenge
	if err := s.persistChallengeToMongoDB(ctx, req.ChallengeId); err != nil {
		// Log the error but don't fail the abandon operation
		fmt.Printf("Warning: Failed to persist abandoned challenge %s to MongoDB: %v\n", req.ChallengeId, err)
	}

	// Check for nil websocketState or LocalState
	if s.GlobalState == nil || s.GlobalState.LocalState == nil {
		// Log the issue (consider adding a proper logger instead of fmt)
		fmt.Printf("Warning: websocketState or LocalState is nil for challenge ID %s\n", req.ChallengeId)
		return &challengePb.AbandonChallengeResponse{Success: true}, nil
	}

	// Get WebSocket clients for broadcasting
	wsClients := s.GlobalState.LocalState.GetAllWSClients(challenge.ChallengeID)
	if len(wsClients) == 0 {
		// Log the issue
		fmt.Printf("Warning: No WebSocket clients found for challenge ID %s\n", challenge.ChallengeID)
		return &challengePb.AbandonChallengeResponse{Success: true}, nil
	}

	// Broadcast the abandon event using the new method with clients
	broadcasts.BroadcastChallengeAbandonWithClients(wsClients, challenge.ChallengeID, challenge.CreatorID)

	return &challengePb.AbandonChallengeResponse{Success: true}, nil
}
func (s *ChallengeService) GetFullChallengeData(ctx context.Context, req *challengePb.GetFullChallengeDataRequest) (*challengePb.GetFullChallengeDataResponse, error) {
	challenge, err := s.GlobalState.Redis.GetChallengeByID(ctx, req.ChallengeId)
	if err != nil {
		challenge, err = s.GlobalState.Mongo.GetChallengeByID(ctx, req.ChallengeId)
		if err != nil {
			return nil, err
		}
	}

	return &challengePb.GetFullChallengeDataResponse{
		Challenge: ChallengesToProto([]*model.ChallengeDocument{&challenge}, false)[0],
	}, nil
}

func (s *ChallengeService) GetChallengeHistory(ctx context.Context, req *challengePb.GetChallengeHistoryRequest) (*challengePb.ChallengeListResponse, error) {
	// Historical data should come from MongoDB repository
	challenges, err := s.GlobalState.Mongo.GetChallengeHistory(ctx, req.UserId, int(req.GetPagination().GetPage()), int(req.GetPagination().GetPageSize()), req.GetIsPrivate())
	if err != nil {
		return nil, err
	}
	return &challengePb.ChallengeListResponse{
		Challenges: ChallengesToProto(toPtrSlice(challenges), false),
		TotalCount: int64(len(challenges)),
	}, nil
}

func (s *ChallengeService) GetActiveOpenChallenges(ctx context.Context, req *challengePb.PaginationRequest) (*challengePb.ChallengeListResponse, error) {
	// For active challenges, use Redis repository only
	challengeIDs, err := s.GlobalState.Redis.GetChallengesByStatus(ctx, string(model.ChallengeOpen))
	if err != nil {
		return nil, err
	}

	// Use ants workerpool for concurrent challenge fetching
	isPrivate := false
	challenges := s.fetchChallengesConcurrently(ctx, challengeIDs, isPrivate)

	return &challengePb.ChallengeListResponse{
		Challenges: ChallengesToProto(toPtrSlice(challenges), true),
		TotalCount: int64(len(challenges)),
	}, nil
}

func (s *ChallengeService) GetOwnersActiveChallenges(ctx context.Context, req *challengePb.GetOwnersActiveChallengesRequest) (*challengePb.ChallengeListResponse, error) {
	log.Printf("[GetOwnersActiveChallenges] Fetching active challenges for user %v", req)

	challengeIDs, err := s.GlobalState.Redis.GetActiveChallenges(ctx)
	if err != nil {
		log.Printf("[GetOwnersActiveChallenges] Error fetching active challenges: %v", err)
		return nil, err
	}
	log.Printf("[GetOwnersActiveChallenges] Retrieved %d active challenge IDs", len(challengeIDs))

	var challenges []model.ChallengeDocument
	for _, id := range challengeIDs {
		challenge, err := s.GlobalState.Redis.GetChallengeByID(ctx, id)
		if err != nil {
			log.Printf("[GetOwnersActiveChallenges] Skipping challenge %s due to fetch error: %v", id, err)
			continue
		}
		if challenge.CreatorID == req.UserId {
			log.Printf("[GetOwnersActiveChallenges] Challenge %s belongs to user %s", id, req.UserId)
			challenges = append(challenges, challenge)
		}
	}
	log.Printf("[GetOwnersActiveChallenges] Found %d challenges owned by user %s", len(challenges), req.UserId)

	log.Printf("[GetOwnersActiveChallenges] Returning %d challenges ", len(challenges))

	return &challengePb.ChallengeListResponse{
		Challenges: ChallengesToProto(toPtrSlice(challenges), false),
	}, nil
}

func (s *ChallengeService) PushSubmissionStatus(ctx context.Context, req *challengePb.PushSubmissionStatusRequest) (*challengePb.PushSubmissionStatusResponse, error) {
	// Extract submission data from request
	challengeID := req.GetChallengeId()
	userID := req.GetUserId()
	problemID := req.GetProblemId()
	score := int(req.GetScore())
	submissionID := req.GetSubmissionId()
	isSuccessful := req.GetIsSuccessful()
	timeTaken := time.Duration(req.GetTimeTakenMillis()) * time.Millisecond

	log.Printf("[PushSubmissionStatus] Processing submission: challenge=%s, user=%s, problem=%s, score=%d, successful=%v",
		challengeID, userID, problemID, score, isSuccessful)

	// Only process successful submissions for leaderboard updates
	if !isSuccessful {
		log.Printf("[PushSubmissionStatus] Submission unsuccessful, skipping leaderboard update")
		return &challengePb.PushSubmissionStatusResponse{Message: "received unsuccessful submission", Success: true}, nil
	}

	// Get challenge from Redis to verify it exists and is active
	challenge, err := s.GlobalState.Redis.GetChallengeByID(ctx, challengeID)
	if err != nil {
		log.Printf("[PushSubmissionStatus] Challenge not found: %v", err)
		return &challengePb.PushSubmissionStatusResponse{Message: "challenge not found", Success: false}, err
	}

	// Verify user is a participant
	participant, exists := challenge.Participants[userID]
	if !exists {
		log.Printf("[PushSubmissionStatus] User %s not a participant in challenge %s", userID, challengeID)
		return &challengePb.PushSubmissionStatusResponse{Message: "user not a participant", Success: false}, nil
	}

	// Update submission data in Redis
	submission := model.Submission{
		SubmissionID: submissionID,
		TimeTaken:    timeTaken,
		Points:       score,
	}

	// Initialize submissions map if needed
	if challenge.Submissions == nil {
		challenge.Submissions = make(map[string]map[string]model.Submission)
	}
	if challenge.Submissions[userID] == nil {
		challenge.Submissions[userID] = make(map[string]model.Submission)
	}

	// Store the submission
	challenge.Submissions[userID][problemID] = submission

	// Update participant metadata
	if participant.ProblemsDone == nil {
		participant.ProblemsDone = make(map[string]model.ChallengeProblemMetadata)
	}
	participant.ProblemsDone[problemID] = model.ChallengeProblemMetadata{
		Score:     score,
		TimeTaken: int64(timeTaken),
	}

	// Calculate new total score for the participant
	totalScore := 0
	for _, problemMeta := range participant.ProblemsDone {
		totalScore += problemMeta.Score
	}
	participant.TotalScore = totalScore
	participant.ProblemsAttempted = len(participant.ProblemsDone)

	// Update participant in Redis
	err = s.GlobalState.Redis.UpdateParticipant(ctx, challengeID, userID, participant)
	if err != nil {
		log.Printf("[PushSubmissionStatus] Failed to update participant: %v", err)
		return &challengePb.PushSubmissionStatusResponse{Message: "failed to update participant", Success: false}, err
	}

	// Update challenge in Redis
	err = s.GlobalState.Redis.UpdateChallenge(ctx, &challenge)
	if err != nil {
		log.Printf("[PushSubmissionStatus] Failed to update challenge: %v", err)
		return &challengePb.PushSubmissionStatusResponse{Message: "failed to update challenge", Success: false}, err
	}

	// Initialize leaderboard if not already done
	err = s.GlobalState.LeaderboardManager.InitializeLeaderboard(challengeID)
	if err != nil {
		log.Printf("[PushSubmissionStatus] Failed to initialize leaderboard: %v", err)
		// Continue processing even if leaderboard fails
	}

	// Update participant score in leaderboard
	err = s.GlobalState.LeaderboardManager.UpdateParticipantScore(challengeID, userID, totalScore)
	if err != nil {
		log.Printf("[PushSubmissionStatus] Failed to update leaderboard score: %v", err)
		// Continue processing even if leaderboard update fails
	}

	// Get updated leaderboard for broadcasting
	var leaderboard []*model.LeaderboardEntry
	var newRank int = -1

	// Get updated leaderboard data
	leaderboard, err = s.GlobalState.LeaderboardManager.GetLeaderboard(challengeID, 50, &challenge) // Get top 50
	if err != nil {
		log.Printf("[PushSubmissionStatus] Failed to get leaderboard: %v", err)
	} else {
		challenge.Leaderboard = leaderboard
		if err := s.GlobalState.Redis.UpdateChallenge(ctx, &challenge); err != nil {
			log.Printf("[PushSubmissionStatus] Failed to persist leaderboard snapshot: %v", err)
		}
	}

	// Get user's new rank
	participantData, err := s.GlobalState.LeaderboardManager.GetParticipantRank(challengeID, userID)
	if err != nil {
		log.Printf("[PushSubmissionStatus] Failed to get participant rank: %v", err)
	} else {
		newRank = participantData.Rank
	}

	// Broadcast events to WebSocket clients
	if s.GlobalState != nil && s.GlobalState.LocalState != nil {
		wsClients := s.GlobalState.LocalState.GetAllWSClients(challengeID)

		// Broadcast NEW_SUBMISSION event
		broadcasts.BroadcastNewSubmission(wsClients, challengeID, userID, problemID, score, newRank)

		// Broadcast LEADERBOARD_UPDATE event if we have leaderboard data
		if leaderboard != nil {
			broadcasts.BroadcastLeaderboardUpdate(wsClients, challengeID, leaderboard, userID)
		}
	}

	log.Printf("[PushSubmissionStatus] Successfully processed submission for user %s in challenge %s, new total score: %d",
		userID, challengeID, totalScore)

	return &challengePb.PushSubmissionStatusResponse{Message: "submission processed successfully", Success: true}, nil
}

// EndChallenge ends a challenge and triggers MongoDB persistence
func (s *ChallengeService) EndChallenge(ctx context.Context, challengeID, creatorID string) error {
	// Fetch the challenge to verify the creator using Redis repository
	challenge, err := s.GlobalState.Redis.GetChallengeByID(ctx, challengeID)
	if err != nil {
		return fmt.Errorf("challenge not found: %w", err)
	}

	if challenge.CreatorID != creatorID {
		return errors.New("only the creator can end the challenge")
	}

	// Clean up leaderboard for ended challenge
	if err := s.GlobalState.LeaderboardManager.CleanupLeaderboard(challengeID); err != nil {
		log.Printf("[EndChallenge] Warning: Failed to cleanup leaderboard for challenge %s: %v", challengeID, err)
	}

	// Update challenge status to ENDED using the helper method that triggers persistence
	if err := s.updateChallengeStatus(ctx, challengeID, model.ChallengeEnded); err != nil {
		return fmt.Errorf("failed to end challenge: %w", err)
	}

	return nil
}

func ChallengeDocumentFromProto(pb *challengePb.ChallengeRecord, hideProblems bool) *model.ChallengeDocument {
	participants := make(map[string]*model.ParticipantMetadata)
	for k, v := range pb.Participants {
		participants[k] = &model.ParticipantMetadata{
			ProblemsDone:      nil,
			LastConnected:     v.LastConnectedUnix,
			ProblemsAttempted: int(v.ProblemsAttempted),
			TotalScore:        int(v.TotalScore),
			JoinTime:          v.JoinTimeUnix,
		}
	}

	submissions := make(map[string]map[string]model.Submission)
	for _, userSub := range pb.Submissions {
		subMap := make(map[string]model.Submission)
		for _, entry := range userSub.Entries {
			subMap[entry.ProblemId] = model.Submission{
				SubmissionID: entry.Submission.SubmissionId,
				TimeTaken:    time.Duration(entry.Submission.TimeTakenMillis) * time.Millisecond,
				Points:       int(entry.Submission.Points),
			}
		}
		submissions[userSub.UserId] = subMap
	}

	var config *model.ChallengeConfig
	if pb.Config != nil {
		config = &model.ChallengeConfig{
			MaxEasyQuestions:   int(pb.GetConfig().GetMaxEasyQuestions()),
			MaxMediumQuestions: int(pb.GetConfig().GetMaxMediumQuestions()),
			MaxHardQuestions:   int(pb.GetConfig().GetMaxHardQuestions()),
			MaxUsers:           int(pb.GetConfig().GetMaxUsers()),
		}
	}

	var processedIds []string
	if !hideProblems {
		processedIds = pb.GetProcessedProblemIds()
	}

	return &model.ChallengeDocument{
		ChallengeID:         pb.GetChallengeId(),
		CreatorID:           pb.GetCreatorId(),
		CreatedAt:           pb.GetCreatedAt(),
		IsPrivate:           pb.GetIsPrivate(),
		Title:               pb.GetTitle(),
		Password:            pb.GetPassword(),
		ProcessedProblemIds: processedIds,
		ProblemCount:        int64(len(pb.GetProcessedProblemIds())),
		Status:              (pb.GetStatus()),
		TimeLimit:           pb.GetTimeLimitMillis(),
		StartTime:           pb.StartTimeUnix,
		Participants:        participants,
		Submissions:         submissions,
		Config:              config,
	}
}

func ChallengesToProto(challenges []*model.ChallengeDocument, hideProblems bool) []*challengePb.ChallengeRecord {
	protoChallenges := make([]*challengePb.ChallengeRecord, 0, len(challenges))
	for _, ch := range challenges {
		record := &challengePb.ChallengeRecord{
			ChallengeId:     ch.ChallengeID,
			CreatorId:       ch.CreatorID,
			CreatedAt:       ch.CreatedAt,
			Title:           ch.Title,
			IsPrivate:       ch.IsPrivate,
			Password:        ch.Password,
			Status:          string(ch.Status),
			ProblemCount:    ch.ProblemCount,
			TimeLimitMillis: ch.TimeLimit,
			StartTimeUnix:   ch.StartTime,
			Participants:    make(map[string]*challengePb.ParticipantMetadata),
			Submissions:     make([]*challengePb.UserSubmissions, 0),
		}

		if !hideProblems {
			record.ProcessedProblemIds = ch.ProcessedProblemIds
		}

		for k, v := range ch.Participants {
			record.Participants[k] = &challengePb.ParticipantMetadata{
				LastConnectedUnix: v.LastConnected,
				ProblemsAttempted: int32(v.ProblemsAttempted),
				TotalScore:        int32(v.TotalScore),
				ProblemsDone:      nil,
				JoinTimeUnix:      v.JoinTime,
			}
		}

		if ch.Config != nil {
			record.Config = &challengePb.ChallengeConfig{
				MaxEasyQuestions:   int32(ch.Config.MaxEasyQuestions),
				MaxMediumQuestions: int32(ch.Config.MaxMediumQuestions),
				MaxHardQuestions:   int32(ch.Config.MaxHardQuestions),
				MaxUsers:           int32(ch.Config.MaxUsers),
			}
		}

		for userId, subMap := range ch.Submissions {
			entries := make([]*challengePb.SubmissionEntry, 0, len(subMap))
			for problemId, sub := range subMap {
				entries = append(entries, &challengePb.SubmissionEntry{
					ProblemId: problemId,
					Submission: &challengePb.SubmissionMetadata{
						SubmissionId:    sub.SubmissionID,
						TimeTakenMillis: int64(sub.TimeTaken / time.Millisecond),
						Points:          int32(sub.Points),
					},
				})
			}
			record.Submissions = append(record.Submissions, &challengePb.UserSubmissions{
				UserId:  userId,
				Entries: entries,
			})
		}

		protoChallenges = append(protoChallenges, record)
	}
	return protoChallenges
}

func toPtrSlice(in []model.ChallengeDocument) []*model.ChallengeDocument {
	out := make([]*model.ChallengeDocument, len(in))
	for i := range in {
		out[i] = &in[i]
	}
	return out
}

// persistChallengeToMongoDB transfers challenge data from Redis to MongoDB and cleans up Redis
func (s *ChallengeService) persistChallengeToMongoDB(ctx context.Context, challengeID string) error {
	// Get challenge data from Redis
	challengeDoc, err := s.GlobalState.Redis.GetChallengeByID(ctx, challengeID)
	if err != nil {
		return fmt.Errorf("failed to get challenge from Redis: %w", err)
	}

	// Persist to MongoDB
	if err := s.GlobalState.Mongo.PersistChallengeFromRedis(ctx, &challengeDoc); err != nil {
		return fmt.Errorf("failed to persist challenge to MongoDB: %w", err)
	}

	// Clean up Redis data after successful MongoDB persistence
	if err := s.GlobalState.Redis.DeleteChallenge(ctx, challengeID); err != nil {
		// Log the error but don't fail the operation since MongoDB persistence succeeded
		fmt.Printf("Warning: Failed to clean up Redis data for challenge %s: %v\n", challengeID, err)
	}

	return nil
}

// updateChallengeStatus updates challenge status and triggers persistence if needed
func (s *ChallengeService) updateChallengeStatus(ctx context.Context, challengeID string, newStatus string) error {
	// Get current challenge from Redis
	challenge, err := s.GlobalState.Redis.GetChallenge(ctx, challengeID)
	if err != nil {
		return fmt.Errorf("failed to get challenge: %w", err)
	}

	// Update status
	challenge.Status = newStatus
	if err := s.GlobalState.Redis.UpdateChallenge(ctx, challenge); err != nil {
		return fmt.Errorf("failed to update challenge status: %w", err)
	}

	// Check if we need to trigger MongoDB persistence
	if newStatus == model.ChallengeAbandon || newStatus == model.ChallengeEnded {
		if err := s.persistChallengeToMongoDB(ctx, challengeID); err != nil {
			// Log the error but don't fail the status update
			fmt.Printf("Warning: Failed to persist challenge %s to MongoDB after status change to %s: %v\n", challengeID, newStatus, err)
		}
	}

	return nil
}
