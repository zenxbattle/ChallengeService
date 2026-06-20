package utils

import (
	"crypto/rand"
)

func GenerateBigCapPassword(length int) string {
	const letters = "ABCDEFGHIJKLMNOPQRSTUVWXYZ1234567890"

	bytes := make([]byte, length)
	_, err := rand.Read(bytes)
	if err != nil {
		return ""
	}

	for i := 0; i < length; i++ {
		bytes[i] = letters[int(bytes[i])%len(letters)]
	}
	return string(bytes)
}
