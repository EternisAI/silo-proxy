package users

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestHashPassword(t *testing.T) {
	password := "testpassword123"

	hash, err := HashPassword(password)
	require.NoError(t, err)
	assert.NotEmpty(t, hash)
	assert.NotEqual(t, password, hash)

	// Verify the hash is valid bcrypt format (starts with $2a$)
	assert.True(t, len(hash) > 0)
	assert.Equal(t, "$2a$", hash[:4])
}

func TestCheckPassword(t *testing.T) {
	password := "correctpassword"

	hash, err := HashPassword(password)
	require.NoError(t, err)

	// Correct password should match
	assert.True(t, CheckPassword(password, hash))

	// Incorrect password should not match
	assert.False(t, CheckPassword("wrongpassword", hash))

	// Empty password should not match
	assert.False(t, CheckPassword("", hash))
}

func TestCheckPasswordWithMigrationHash(t *testing.T) {
	// This test verifies the hardcoded hash in the migration works
	// The hash was generated with password "changeme"
	migrationHash := "$2a$10$uejoNCSLZ9YkKOZriLlSGeg0pm/nuGVS3nRuSPyYuk/Z7HJHKBhGO"

	// Should match the expected password
	assert.True(t, CheckPassword("changeme", migrationHash))

	// Should not match other passwords
	assert.False(t, CheckPassword("root", migrationHash))
	assert.False(t, CheckPassword("admin", migrationHash))
}
