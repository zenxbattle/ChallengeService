# Challenge Lifecycle Documentation

## Overview

The ChallengeWssManagerService manages the complete lifecycle of competitive programming challenges through a dual-storage architecture using Redis for active state and MongoDB for historical persistence. The system provides real-time updates via WebSockets and maintains leaderboards using RedisBoard.

## Architecture Components

### Storage Layer
- **Redis**: Active challenge state, real-time data, participant sessions
- **MongoDB**: Historical challenge data, completed challenges, persistence
- **RedisBoard**: Real-time leaderboard management with ranking algorithms

### Communication Layer
- **WebSocket**: Real-time bidirectional communication with clients
- **gRPC**: Service-to-service communication for challenge operations
- **HTTP**: WebSocket upgrade endpoint and health checks

### State Management
- **Global State**: Shared repositories and leaderboard manager
- **Local State**: WebSocket connections, sessions, event channels per challenge

## Challenge States

```
CHALLENGEOPEN      → Challenge created, accepting participants
CHALLENGESTARTED   → Challenge in progress, submissions being processed
CHALLENGEFORFIETED → Challenge forfeited by participants
CHALLENGEENDED     → Challenge completed normally
CHALLENGEABANDON   → Challenge abandoned by creator
```

## Complete Challenge Lifecycle

### 1. Challenge Creation Phase

**Trigger**: gRPC `CreateChallenge` request

**Process**:
1. **Validation**: Check for existing active challenges (only one active challenge allowed)
2. **Initialization**: 
   - Create `ChallengeDocument` with status `CHALLENGEOPEN`
   - Initialize empty participants, submissions, and leaderboard maps
   - Generate password for private challenges
   - Set challenge configuration (max users, problem limits)
3. **Storage**: Store challenge in Redis only (not MongoDB yet)
4. **Leaderboard Setup**: 
   - Initialize RedisBoard instance for the challenge
   - Add creator to leaderboard with score 0
5. **Response**: Return challenge details to creator

**Data Flow**:
```
gRPC Request → ChallengeService → RedisRepository → LeaderboardManager
```

### 2. Participant Join Phase

**Trigger**: WebSocket `JOIN_CHALLENGE` message

**Process**:
1. **Authentication**: Validate user token via API Gateway
2. **Challenge Validation**: 
   - Fetch challenge from Redis
   - Verify challenge is not abandoned
   - Check password for private challenges
3. **Participant Management**:
   - Create/update participant metadata in Redis
   - Track join time, IP address, connection status
4. **Local State**: Add WebSocket connection to LocalStateManager
5. **Broadcasting**: Notify all connected clients of new participant
6. **Response**: Send challenge data and participant status

**Data Flow**:
```
WebSocket → JoinChallengeHandler → RedisRepository → LocalStateManager → Broadcast
```

### 3. Active Challenge Phase

#### Real-time Leaderboard Management

**Leaderboard Data Source**:
- **Primary**: RedisBoard instances (one per challenge)
- **Namespace**: `challenge_{challengeID}`
- **Ranking**: Real-time score-based ranking with O(log n) updates
- **Capacity**: Top 50 users tracked, max 10,000 users per challenge

**Leaderboard Updates**:
1. **Score Calculation**: Based on successful problem submissions
2. **Rank Calculation**: Automatic via RedisBoard's ranking algorithms
3. **Broadcasting**: Real-time updates to all WebSocket clients
4. **Persistence**: Leaderboard state maintained in Redis memory

#### Submission Processing

**Trigger**: gRPC `PushSubmissionStatus` request

**Process**:
1. **Validation**: 
   - Verify challenge exists and is active
   - Confirm user is participant
   - Process only successful submissions
2. **Data Updates**:
   - Store submission in Redis challenge document
   - Update participant metadata (problems done, total score)
   - Calculate new total score for participant
3. **Leaderboard Updates**:
   - Update participant score in RedisBoard
   - Get new rank and leaderboard data
4. **Real-time Broadcasting**:
   - `NEW_SUBMISSION` event with score and rank
   - `LEADERBOARD_UPDATE` event with updated rankings
5. **Response**: Confirm submission processing

**Data Flow**:
```
gRPC Request → ChallengeService → RedisRepository → LeaderboardManager → WebSocket Broadcast
```

#### WebSocket Event Broadcasting

**Event Types**:
- `USER_JOINED` / `OWNER_JOINED`: New participant connections
- `USER_LEFT` / `OWNER_LEFT`: Participant disconnections  
- `NEW_SUBMISSION`: Successful problem submissions
- `LEADERBOARD_UPDATE`: Ranking changes
- `CURRENT_LEADERBOARD`: Leaderboard data requests
- `CREATOR_ABANDON`: Challenge abandonment

**Broadcasting Mechanism**:
1. Get all WebSocket clients for challenge from LocalStateManager
2. Send JSON message to each connected client
3. Handle connection failures gracefully
4. Include timestamp and challenge context

### 4. Challenge Termination Phase

#### Normal Completion (CHALLENGEENDED)

**Trigger**: `EndChallenge` service call

**Process**:
1. **Authorization**: Verify only creator can end challenge
2. **Status Update**: Change challenge status to `CHALLENGEENDED` in Redis
3. **Leaderboard Cleanup**: Close RedisBoard instance and free resources
4. **Persistence**: Transfer complete challenge data from Redis to MongoDB
5. **Cleanup**: Remove challenge data from Redis after successful MongoDB storage

#### Challenge Abandonment (CHALLENGEABANDON)

**Trigger**: gRPC `AbandonChallenge` request

**Process**:
1. **Authorization**: Verify only creator can abandon challenge
2. **Status Update**: Change challenge status to `CHALLENGEABANDON` in Redis
3. **Leaderboard Cleanup**: Close RedisBoard instance
4. **Persistence**: Transfer challenge data to MongoDB for historical record
5. **Broadcasting**: Notify all participants of abandonment via WebSocket
6. **Cleanup**: Remove Redis data and close WebSocket connections

### 5. Data Persistence Strategy

#### Active Challenge Data (Redis)
- **Challenge Documents**: Complete challenge state including participants and submissions
- **Session Data**: User sessions and connection status
- **Real-time State**: Current leaderboard positions and scores
- **Temporary Storage**: Data exists only during active challenge lifecycle

#### Historical Data (MongoDB)
- **Completed Challenges**: Full challenge records with final results
- **Participant History**: User participation records across challenges
- **Submission Archives**: Complete submission history and scores
- **Leaderboard Snapshots**: Final rankings and statistics

#### Data Migration Flow
```
Active Challenge (Redis) → Challenge Completion → Historical Storage (MongoDB) → Redis Cleanup
```

## Real-time Features

### WebSocket Connection Management
- **Connection Tracking**: LocalStateManager maintains user-to-connection mapping
- **Session Management**: Track user activity and connection status
- **Graceful Disconnection**: Handle network failures and reconnections
- **Broadcasting**: Efficient message distribution to all challenge participants

### Leaderboard Real-time Updates
- **Score Updates**: Immediate reflection of submission results
- **Rank Changes**: Real-time position updates as scores change
- **Top-K Tracking**: Efficient top 50 user tracking via RedisBoard
- **Participant Queries**: Individual rank and score lookups

### Event-Driven Architecture
- **Event Channels**: Per-challenge event channels for internal communication
- **Message Broadcasting**: Real-time event distribution to WebSocket clients
- **State Synchronization**: Consistent state across Redis, leaderboard, and WebSocket clients

## Performance Characteristics

### Scalability
- **Concurrent Challenges**: Support for multiple simultaneous challenges
- **Participant Limits**: Configurable max users per challenge (default: 10,000)
- **Connection Limits**: WebSocket connection pooling and management
- **Memory Management**: Automatic cleanup of completed challenges

### Efficiency
- **O(log n) Leaderboard Updates**: RedisBoard provides efficient ranking operations
- **Redis Performance**: In-memory operations for active challenge data
- **Minimal MongoDB Writes**: Only persist completed/abandoned challenges
- **WebSocket Broadcasting**: Efficient message distribution without polling

## Error Handling and Recovery

### Connection Failures
- **WebSocket Reconnection**: Clients can rejoin challenges after disconnection
- **Session Timeout**: 30-minute session timeout with cleanup
- **Graceful Degradation**: Continue operation with partial connectivity

### Data Consistency
- **Redis Persistence**: RDB snapshots for data recovery
- **MongoDB Backup**: Historical data preservation
- **State Recovery**: Rebuild local state from Redis on service restart

### Challenge Recovery
- **Active Challenge Detection**: Restore active challenges from Redis on startup
- **Leaderboard Reconstruction**: Reinitialize RedisBoard instances for active challenges
- **WebSocket Reconnection**: Allow participants to rejoin active challenges

## Monitoring and Observability

### Key Metrics
- **Active Challenges**: Number of concurrent challenges
- **Participant Count**: Total and per-challenge participant counts
- **Submission Rate**: Submissions processed per second
- **WebSocket Connections**: Active connection count and health
- **Leaderboard Performance**: Update latency and query response times

### Health Checks
- **Redis Connectivity**: Monitor Redis connection and performance
- **MongoDB Status**: Check MongoDB connection and write performance
- **WebSocket Health**: Monitor connection counts and message throughput
- **gRPC Service**: Monitor service availability and response times