package users

import (
	"fmt"

	"golang.org/x/crypto/bcrypt"
)

// DefaultCost is the bcrypt cost factor used for password hashing
const DefaultCost = bcrypt.DefaultCost

// HashPassword generates a bcrypt hash from a plaintext password
func HashPassword(password string) (string, error) {
	hash, err := bcrypt.GenerateFromPassword([]byte(password), DefaultCost)
	if err != nil {
		return "", fmt.Errorf("failed to hash password: %w", err)
	}
	return string(hash), nil
}

// CheckPassword compares a plaintext password with a bcrypt hash
func CheckPassword(password, hash string) bool {
	err := bcrypt.CompareHashAndPassword([]byte(hash), []byte(password))
	return err == nil
}
