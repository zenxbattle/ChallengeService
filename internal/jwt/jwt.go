package jwt

import (
	"errors"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

//CustomClaims represents the JWT claims with challengeId and userId
type CustomClaims struct {
	UserID      string `json:"userId"`
	ChallengeID string `json:"challengeId"`
	EntityType  string `json:"entityType"`
	jwt.RegisteredClaims
}

//JWTManager handles JWT operations
type JWTManager struct {
	secretKey []byte
}

//NewJWTManager creates a new JWT manager with the provided secret key
func NewJWTManager(secretKey string) *JWTManager {
	return &JWTManager{
		secretKey: []byte(secretKey),
	}
}

//generateToken generates a JWT token with userId and challengeId claims
func (j *JWTManager) GenerateToken(userID, challengeID string, expiration time.Duration) (string, error) {
	if userID == "" {
		return "", errors.New("userID cannot be empty")
	}
	if challengeID == "" {
		return "", errors.New("challengeID cannot be empty")
	}

	claims := CustomClaims{
		UserID:      userID,
		ChallengeID: challengeID,
		RegisteredClaims: jwt.RegisteredClaims{
			ExpiresAt: jwt.NewNumericDate(time.Now().Add(expiration)),
			IssuedAt:  jwt.NewNumericDate(time.Now()),
			NotBefore: jwt.NewNumericDate(time.Now()),
		},
	}

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	return token.SignedString(j.secretKey)
}

//validateToken validates a JWT token and returns the claims
func (j *JWTManager) ValidateToken(tokenString string) (*CustomClaims, error) {
	if tokenString == "" {
		return nil, errors.New("token cannot be empty")
	}

	token, err := jwt.ParseWithClaims(tokenString, &CustomClaims{}, func(token *jwt.Token) (interface{}, error) {
		// Validate the signing method
		if _, ok := token.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, errors.New("invalid signing method")
		}
		return j.secretKey, nil
	})

	if err != nil {
		return nil, err
	}

	if claims, ok := token.Claims.(*CustomClaims); ok && token.Valid {
		return claims, nil
	}

	return nil, errors.New("invalid token")
}

//extractClaims extracts claims from a token without full validation (for middleware use)
func (j *JWTManager) ExtractClaims(tokenString string) (*CustomClaims, error) {
	token, _, err := new(jwt.Parser).ParseUnverified(tokenString, &CustomClaims{})
	if err != nil {
		return nil, err
	}

	if claims, ok := token.Claims.(*CustomClaims); ok {
		return claims, nil
	}

	return nil, errors.New("invalid token claims")
}