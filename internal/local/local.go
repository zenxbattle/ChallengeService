package state

import (
	"sync"

	"github.com/gorilla/websocket"
	"github.com/lijuuu/ChallengeWssManagerService/internal/model"
)

type LocalStateManager struct {
	challengeStates map[string]*ChallengeLocalState
	mu              sync.RWMutex
}

type ChallengeLocalState struct {
	Sessions  map[string]*model.Session
	WSClients map[string]*websocket.Conn
	MU        sync.RWMutex
	EventChan chan model.Event
}

func NewLocalStateManager() *LocalStateManager {
	return &LocalStateManager{
		challengeStates: make(map[string]*ChallengeLocalState),
	}
}

// GetChallengeState returns the local state for a challenge, creating it if it doesn't exist
func (lsm *LocalStateManager) GetChallengeState(challengeID string) *ChallengeLocalState {
	lsm.mu.Lock()
	defer lsm.mu.Unlock()

	state, exists := lsm.challengeStates[challengeID]
	if !exists {
		state = &ChallengeLocalState{
			Sessions:  make(map[string]*model.Session),
			WSClients: make(map[string]*websocket.Conn),
			EventChan: make(chan model.Event, 100),
		}
		lsm.challengeStates[challengeID] = state
	}

	return state
}

// AddSession adds a session to the challenge's local state
func (lsm *LocalStateManager) AddSession(challengeID string, session *model.Session) {
	state := lsm.GetChallengeState(challengeID)
	state.MU.Lock()
	defer state.MU.Unlock()

	state.Sessions[session.UserID] = session
}

// RemoveSession removes a session from the challenge's local state
func (lsm *LocalStateManager) RemoveSession(challengeID, userID string) {
	lsm.mu.RLock()
	state, exists := lsm.challengeStates[challengeID]
	lsm.mu.RUnlock()

	if !exists {
		return
	}

	state.MU.Lock()
	defer state.MU.Unlock()

	delete(state.Sessions, userID)
}

// GetSession retrieves a session from the challenge's local state
func (lsm *LocalStateManager) GetSession(challengeID, userID string) (*model.Session, bool) {
	lsm.mu.RLock()
	state, exists := lsm.challengeStates[challengeID]
	lsm.mu.RUnlock()

	if !exists {
		return nil, false
	}

	state.MU.RLock()
	defer state.MU.RUnlock()

	session, found := state.Sessions[userID]
	return session, found
}

// AddWSClient adds a WebSocket client to the challenge's local state
func (lsm *LocalStateManager) AddWSClient(challengeID, userID string, conn *websocket.Conn) {
	state := lsm.GetChallengeState(challengeID)
	state.MU.Lock()
	defer state.MU.Unlock()

	state.WSClients[userID] = conn
}

// RemoveWSClient removes a WebSocket client from the challenge's local state
func (lsm *LocalStateManager) RemoveWSClient(challengeID, userID string) {
	lsm.mu.RLock()
	state, exists := lsm.challengeStates[challengeID]
	lsm.mu.RUnlock()

	if !exists {
		return
	}

	state.MU.Lock()
	defer state.MU.Unlock()

	if conn, exists := state.WSClients[userID]; exists {
		conn.Close()
		delete(state.WSClients, userID)
	}
}

// GetWSClient retrieves a WebSocket client from the challenge's local state
func (lsm *LocalStateManager) GetWSClient(challengeID, userID string) (*websocket.Conn, bool) {
	lsm.mu.RLock()
	state, exists := lsm.challengeStates[challengeID]
	lsm.mu.RUnlock()

	if !exists {
		return nil, false
	}

	state.MU.RLock()
	defer state.MU.RUnlock()

	conn, found := state.WSClients[userID]
	return conn, found
}

// GetAllWSClients returns all WebSocket clients for a challenge
func (lsm *LocalStateManager) GetAllWSClients(challengeID string) map[string]*websocket.Conn {
	lsm.mu.RLock()
	state, exists := lsm.challengeStates[challengeID]
	lsm.mu.RUnlock()

	if !exists {
		return make(map[string]*websocket.Conn)
	}

	state.MU.RLock()
	defer state.MU.RUnlock()

	// Return a copy to avoid concurrent access issues
	clients := make(map[string]*websocket.Conn)
	for userID, conn := range state.WSClients {
		clients[userID] = conn
	}

	return clients
}

// SendEvent sends an event to the challenge's event channel
func (lsm *LocalStateManager) SendEvent(challengeID string, event model.Event) {
	state := lsm.GetChallengeState(challengeID)

	select {
	case state.EventChan <- event:
		// Event sent successfully
	default:
		// Channel is full, skip the event (or handle as needed)
	}
}

// GetEventChannel returns the event channel for a challenge
func (lsm *LocalStateManager) GetEventChannel(challengeID string) <-chan model.Event {
	state := lsm.GetChallengeState(challengeID)
	return state.EventChan
}

// CleanupChallenge removes all local state for a challenge
func (lsm *LocalStateManager) CleanupChallenge(challengeID string) {
	lsm.mu.Lock()
	defer lsm.mu.Unlock()

	state, exists := lsm.challengeStates[challengeID]
	if !exists {
		return
	}

	state.MU.Lock()
	defer state.MU.Unlock()

	// Close all WebSocket connections
	for _, conn := range state.WSClients {
		conn.Close()
	}

	// Close event channel
	close(state.EventChan)

	// Remove from map
	delete(lsm.challengeStates, challengeID)
}

// GetAllChallengeIDs returns all challenge IDs that have local state
func (lsm *LocalStateManager) GetAllChallengeIDs() []string {
	lsm.mu.RLock()
	defer lsm.mu.RUnlock()

	ids := make([]string, 0, len(lsm.challengeStates))
	for id := range lsm.challengeStates {
		ids = append(ids, id)
	}

	return ids
}
