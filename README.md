# ChallengeWssManagerService

A Go-based service for managing LeetCode-like tournament rooms with real-time updates via WebSockets.

## ⚙️ Setup

1. **Ensure Go 1.21+ is installed**
2. **Clone the repository**  
   `git clone https://github.com/lijuuu/ChallengeWssManagerService && cd ChallengeWssManagerService`
3. **Install dependencies**  
   `go mod tidy`
4. **Run the server**  
   `go run cmd/server/main.go`

## 🛠 Endpoints

- `POST /challenges`: Create a new challenge  
- `GET /challenges`: List open challenges  
- `GET /challenges/{challenge_id}`: Get challenge details  
- `GET /ws/{challenge_id}`: WebSocket endpoint for real-time updates

## 🔌 WebSocket Messages

- `join_challenge`: Join a challenge  
- `start_challenge`: Start a challenge (creator only)  
- `end_challenge`: End a challenge (creator only)  
- `delete_challenge`: Delete a challenge (creator only)  
- `submit_problem`: Submit a problem solution  
- `forfeit`: Forfeit the challenge  
- `ping`: Keep the session alive

## ⚙️ Configuration

- **Server Port**: `:8080`  
- **Max Concurrent Matches**: `100`  
- **Session Timeout**: `30 minutes`  
- **WebSocket Read Timeout**: `60 seconds`  
- **Empty Challenge Timeout**: `10 minutes`

---

🧪 Built for fast-paced, competitive environments where real-time code battles matter.
