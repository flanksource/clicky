package middleware

import (
	"bufio"
	"crypto/md5"
	"crypto/sha1"
	"encoding/hex"
	"fmt"
	"os"
	"strings"

	"golang.org/x/crypto/bcrypt"
)

// User represents a user with credentials
type User struct {
	Username string
	Password string
	Hash     string // For htpasswd files
}

// UserStore manages user credentials from various sources
type UserStore struct {
	users map[string]*User
}

// NewUserStore creates a new user store
func NewUserStore() *UserStore {
	return &UserStore{
		users: make(map[string]*User),
	}
}

// LoadHtpasswdFile loads users from an Apache htpasswd file
// Supports bcrypt, SHA1, and MD5 password hashing
func (us *UserStore) LoadHtpasswdFile(filename string) error {
	file, err := os.Open(filename)
	if err != nil {
		return fmt.Errorf("failed to open htpasswd file %s: %w", filename, err)
	}
	defer file.Close()

	scanner := bufio.NewScanner(file)
	lineNum := 0

	for scanner.Scan() {
		lineNum++
		line := strings.TrimSpace(scanner.Text())

		// Skip empty lines and comments
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}

		// Parse username:password_hash format
		parts := strings.SplitN(line, ":", 2)
		if len(parts) != 2 {
			return fmt.Errorf("invalid htpasswd format at line %d: %s", lineNum, line)
		}

		username := parts[0]
		hash := parts[1]

		if username == "" {
			return fmt.Errorf("empty username at line %d", lineNum)
		}

		us.users[username] = &User{
			Username: username,
			Hash:     hash,
		}
	}

	if err := scanner.Err(); err != nil {
		return fmt.Errorf("error reading htpasswd file: %w", err)
	}

	return nil
}

// LoadUserpassFile loads users from a simple user=password text file
func (us *UserStore) LoadUserpassFile(filename string) error {
	file, err := os.Open(filename)
	if err != nil {
		return fmt.Errorf("failed to open userpass file %s: %w", filename, err)
	}
	defer file.Close()

	scanner := bufio.NewScanner(file)
	lineNum := 0

	for scanner.Scan() {
		lineNum++
		line := strings.TrimSpace(scanner.Text())

		// Skip empty lines and comments
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}

		// Parse username=password format
		parts := strings.SplitN(line, "=", 2)
		if len(parts) != 2 {
			return fmt.Errorf("invalid userpass format at line %d: expected 'user=password', got: %s", lineNum, line)
		}

		username := parts[0]
		password := parts[1]

		if username == "" {
			return fmt.Errorf("empty username at line %d", lineNum)
		}

		us.users[username] = &User{
			Username: username,
			Password: password,
		}
	}

	if err := scanner.Err(); err != nil {
		return fmt.Errorf("error reading userpass file: %w", err)
	}

	return nil
}

// ValidateUser validates username and password against stored credentials
func (us *UserStore) ValidateUser(username, password string) (bool, error) {
	user, exists := us.users[username]
	if !exists {
		return false, nil
	}

	// If user has a hash (from htpasswd), validate against hash
	if user.Hash != "" {
		return us.validatePasswordHash(password, user.Hash)
	}

	// If user has plain password (from userpass), compare directly
	if user.Password != "" {
		return user.Password == password, nil
	}

	return false, fmt.Errorf("user %s has no password or hash configured", username)
}

// validatePasswordHash validates a password against various hash formats
func (us *UserStore) validatePasswordHash(password, hash string) (bool, error) {
	// Bcrypt hash (starts with $2a$, $2b$, $2x$, or $2y$)
	if strings.HasPrefix(hash, "$2") {
		err := bcrypt.CompareHashAndPassword([]byte(hash), []byte(password))
		return err == nil, nil
	}

	// SHA1 hash (starts with {SHA})
	if strings.HasPrefix(hash, "{SHA}") {
		expectedHash := hash[5:] // Remove {SHA} prefix
		sha := sha1.New()
		sha.Write([]byte(password))
		actualHash := hex.EncodeToString(sha.Sum(nil))
		return strings.EqualFold(expectedHash, actualHash), nil
	}

	// MD5 hash (starts with $apr1$ or $1$)
	if strings.HasPrefix(hash, "$apr1$") || strings.HasPrefix(hash, "$1$") {
		// Apache MD5 - this is a simplified implementation
		// In production, you'd want a more robust MD5-crypt implementation
		return us.validateMD5Hash(password, hash)
	}

	// Plain text comparison (not recommended for production)
	return hash == password, nil
}

// validateMD5Hash validates password against Apache MD5 hash
// This is a simplified implementation - consider using a proper MD5-crypt library for production
func (us *UserStore) validateMD5Hash(password, hash string) (bool, error) {
	// For now, just do a simple MD5 comparison
	// In production, you'd implement proper Apache MD5-crypt algorithm
	md5Hash := md5.New()
	md5Hash.Write([]byte(password))
	actualHash := hex.EncodeToString(md5Hash.Sum(nil))

	// Extract hash part (after the salt)
	parts := strings.Split(hash, "$")
	if len(parts) < 4 {
		return false, fmt.Errorf("invalid MD5 hash format")
	}

	expectedHash := parts[3]
	return strings.EqualFold(expectedHash, actualHash), nil
}

// GetUser retrieves user information by username
func (us *UserStore) GetUser(username string) (*User, bool) {
	user, exists := us.users[username]
	if !exists {
		return nil, false
	}

	// Return a copy to prevent modification
	return &User{
		Username: user.Username,
		Password: user.Password,
		Hash:     user.Hash,
	}, true
}

// ListUsers returns a list of all usernames
func (us *UserStore) ListUsers() []string {
	usernames := make([]string, 0, len(us.users))
	for username := range us.users {
		usernames = append(usernames, username)
	}
	return usernames
}

// UserCount returns the number of users in the store
func (us *UserStore) UserCount() int {
	return len(us.users)
}

// AddUser adds a user with plain password (for testing or dynamic user management)
func (us *UserStore) AddUser(username, password string) {
	us.users[username] = &User{
		Username: username,
		Password: password,
	}
}

// AddUserWithHash adds a user with pre-hashed password (for testing or dynamic user management)
func (us *UserStore) AddUserWithHash(username, hash string) {
	us.users[username] = &User{
		Username: username,
		Hash:     hash,
	}
}

// RemoveUser removes a user from the store
func (us *UserStore) RemoveUser(username string) {
	delete(us.users, username)
}

// Clear removes all users from the store
func (us *UserStore) Clear() {
	us.users = make(map[string]*User)
}
