package config

import (
	"log"
	"os"
	"strconv"

	"github.com/joho/godotenv"
)

type Config struct {
	ChallengeGRPCPort string
	ChallengeHTTPPort string
	PsqlURL           string
	MongoURL          string
	SessionSecretKey  string
	RedisURL          string
	RedisPassword     string
	RedisDB           int

	JWTSecret string

	APIGatewayTokenCheckURL string
}

func LoadConfig() Config {
	err := godotenv.Load()
	if err != nil {
		log.Fatal("Error loading .env file", err)
	}
	config := Config{
		ChallengeGRPCPort:       getEnv("CHALLENGEGRPCPORT", "50057"),
		PsqlURL:                 getEnv("PSQLURL", "host=localhost port=5432 user=admin password=password dbname=xcodedev sslmode=disable"),
		ChallengeHTTPPort:       getEnv("CHALLENGEHTTPPORT", "3333"),
		SessionSecretKey:        getEnv("SESSIONSECRETKEY", "something"),
		MongoURL:                getEnv("MONGOURL", ""),
		RedisURL:                getEnv("REDISURL", "localhost:6379"),
		RedisPassword:           getEnv("REDISPASSWORD", ""),
		RedisDB:                 getEnvInt("REDISDB", 0),
		APIGatewayTokenCheckURL: getEnv("APIGATEWAYTOKENCHECKURL", "http://localhost:7000/api/v1/users/check-token"),
		JWTSecret:               getEnv("JWTSECRET", "secrettt"),
	}

	return config
}

func getEnv(key, defaultValue string) string {
	if value, exists := os.LookupEnv(key); exists {
		return value
	}
	return defaultValue
}

func getEnvInt(key string, defaultValue int) int {
	if value, exists := os.LookupEnv(key); exists {
		if intValue, err := strconv.Atoi(value); err == nil {
			return intValue
		}
	}
	return defaultValue
}
