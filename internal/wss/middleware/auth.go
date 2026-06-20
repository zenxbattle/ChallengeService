package middleware

import (
	"context"
	"errors"
	"log"
	"strings"

	"github.com/lijuuu/ChallengeWssManagerService/internal/jwt"
	"github.com/lijuuu/ChallengeWssManagerService/internal/wss/broadcasts"
	wsstypes "github.com/lijuuu/ChallengeWssManagerService/internal/wss/types"
)

// AuthMiddleware handles JWT authentication for WebSocket connections
type AuthMiddleware struct {
	jwtManager *jwt.JWTManager
}

// NewAuthMiddleware creates a new authentication middleware
func NewAuthMiddleware(jwtManager *jwt.JWTManager) *AuthMiddleware {
	return &AuthMiddleware{
		jwtManager: jwtManager,
	}
}

// AuthenticateJWT validates JWT token and extracts claims
func (m *AuthMiddleware) AuthenticateJWT(ctx *wsstypes.WsContext, token string) (*jwt.CustomClaims, error) {
	if token == "" {
		return nil, errors.New("token is required")
	}

	// Remove "Bearer " prefix if present
	if after, ok := strings.CutPrefix(token, "Bearer "); ok {
		token = after
	}

	// Validate the token
	claims, err := m.jwtManager.ValidateToken(token)
	if err != nil {
		log.Printf("[Auth] JWT validation failed: %v", err)
		return nil, errors.New("invalid or expired token")
	}

	return claims, nil
}

// CheckChallengeAccess verifies if user can join the specified challenge
func (m *AuthMiddleware) CheckChallengeAccess(ctx *wsstypes.WsContext, userID, challengeID string) error {
	if userID == "" || challengeID == "" {
		return errors.New("userID and challengeID are required")
	}

	// Check if user can join the challenge
	canJoin, err := ctx.State.Redis.CanJoin(context.Background(), challengeID, userID)
	if err != nil {
		log.Printf("[Auth] CanJoin check failed for user %s, challenge %s: %v", userID, challengeID, err)
		return err
	}

	if !canJoin {
		return errors.New("user cannot join this challenge")
	}

	return nil
}

// AuthorizeJoinChallenge performs complete authorization for join challenge requests
func (m *AuthMiddleware) AuthorizeJoinChallenge(ctx *wsstypes.WsContext, token, challengeID string) (*jwt.CustomClaims, error) {
	// Authenticate JWT token
	claims, err := m.AuthenticateJWT(ctx, token)
	if err != nil {
		return nil, err
	}

	// Verify that the challenge ID in the token matches the request
	if claims.ChallengeID != challengeID {
		log.Printf("[Auth] Challenge ID mismatch: token has %s, request has %s", claims.ChallengeID, challengeID)
		return nil, errors.New("challenge ID mismatch")
	}

	// Check if user can join the challenge
	err = m.CheckChallengeAccess(ctx, claims.UserID, challengeID)
	if err != nil {
		return nil, err
	}

	return claims, nil
}

// SendAuthError sends a standardized authentication error response
func (m *AuthMiddleware) SendAuthError(ctx *wsstypes.WsContext, messageType, errorMsg string) error {
	return broadcasts.SendErrorWithType(ctx.Conn, messageType, errorMsg, map[string]any{
		"authError": true,
	})
}

// JWTMiddleware creates a middleware function for JWT verification
func (m *AuthMiddleware) JWTMiddleware() func(*wsstypes.WsContext) error {
	return func(ctx *wsstypes.WsContext) error {
		// Extract token from payload
		var token string
		if tokenVal, exists := ctx.Payload["token"]; exists {
			if tokenStr, ok := tokenVal.(string); ok {
				token = tokenStr
			}
		}

		if token == "" {
			return broadcasts.SendErrorWithType(ctx.Conn, "AUTH_ERROR", "Authentication token required", nil)
		}

		// Validate JWT token
		claims, err := m.jwtManager.ValidateToken(token)
		if err != nil {
			return broadcasts.SendErrorWithType(ctx.Conn, "AUTH_ERROR", "Invalid or expired token", nil)
		}

		// Store claims in context for handler use
		ctx.Claims = claims
		ctx.UserID = claims.UserID

		return nil
	}
}
