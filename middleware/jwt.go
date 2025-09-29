package middleware

import (
	"crypto/rsa"
	"crypto/x509"
	"encoding/pem"
	"fmt"
	"io/ioutil"
	"net/http"
	"os"
	"strings"

	"github.com/golang-jwt/jwt/v5"
	"github.com/labstack/echo/v4"
)

// JWTMiddleware creates JWT authentication middleware based on configuration
func JWTMiddleware(config *JWTAuthConfig, celEngine *CELEngine) echo.MiddlewareFunc {
	if config == nil {
		panic("JWT configuration is required")
	}

	// Set defaults
	if config.SigningMethod == "" {
		config.SigningMethod = "HS256"
	}
	if config.TokenLookup == "" {
		config.TokenLookup = "header:Authorization"
	}
	if config.TokenPrefix == "" {
		config.TokenPrefix = "Bearer "
	}

	// Parse signing key based on method
	var signingKey interface{}
	var err error

	if isHMACMethod(config.SigningMethod) {
		// HMAC methods use string keys
		signingKey = []byte(config.SigningKey)
	} else {
		// RSA/ECDSA methods use key files
		if config.SigningKeyFile != "" {
			signingKey, err = loadSigningKeyFromFile(config.SigningKeyFile, config.SigningMethod)
			if err != nil {
				panic(fmt.Sprintf("Failed to load JWT signing key: %v", err))
			}
		} else {
			panic("SigningKeyFile is required for RSA/ECDSA signing methods")
		}
	}

	return func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c echo.Context) error {
			// Extract token from request
			tokenString, err := extractToken(c, config.TokenLookup, config.TokenPrefix)
			if err != nil {
				if config.ErrorHandler != nil {
					return config.ErrorHandler(err, c)
				}
				return echo.NewHTTPError(http.StatusUnauthorized, "Missing or invalid token")
			}

			// Parse and validate token
			token, err := jwt.Parse(tokenString, func(token *jwt.Token) (interface{}, error) {
				// Verify signing method
				if token.Method.Alg() != config.SigningMethod {
					return nil, fmt.Errorf("unexpected signing method: %v", token.Header["alg"])
				}
				return signingKey, nil
			})

			if err != nil {
				if config.ErrorHandler != nil {
					return config.ErrorHandler(err, c)
				}
				return echo.NewHTTPError(http.StatusUnauthorized, "Invalid token")
			}

			// Check if token is valid
			if !token.Valid {
				if config.ErrorHandler != nil {
					return config.ErrorHandler(fmt.Errorf("token is not valid"), c)
				}
				return echo.NewHTTPError(http.StatusUnauthorized, "Invalid token")
			}

			// Extract claims
			claims, ok := token.Claims.(jwt.MapClaims)
			if !ok {
				if config.ErrorHandler != nil {
					return config.ErrorHandler(fmt.Errorf("invalid token claims"), c)
				}
				return echo.NewHTTPError(http.StatusUnauthorized, "Invalid token claims")
			}

			// Apply CEL validation if configured
			if config.Validation != "" && celEngine != nil {
				variables := CreateJWTVariables(c, token, claims)
				valid, err := celEngine.EvaluateCondition(config.Validation, variables)
				if err != nil {
					if config.ErrorHandler != nil {
						return config.ErrorHandler(fmt.Errorf("CEL validation error: %w", err), c)
					}
					return echo.NewHTTPError(http.StatusInternalServerError, "Token validation error")
				}

				if !valid {
					if config.ErrorHandler != nil {
						return config.ErrorHandler(fmt.Errorf("CEL validation failed"), c)
					}
					return echo.NewHTTPError(http.StatusUnauthorized, "Token validation failed")
				}
			}

			// Store token and claims in context
			c.Set("jwt_token", token)
			c.Set("jwt_claims", claims)

			// Extract user information if available
			if userClaim, exists := claims["sub"]; exists {
				c.Set("user", userClaim)
			}
			if userClaim, exists := claims["username"]; exists {
				c.Set("user", userClaim)
			}

			// Call success handler if configured
			if config.SuccessHandler != nil {
				if err := config.SuccessHandler(c); err != nil {
					return err
				}
			}

			return next(c)
		}
	}
}

// extractToken extracts JWT token from various locations in the request
func extractToken(c echo.Context, tokenLookup, tokenPrefix string) (string, error) {
	parts := strings.Split(tokenLookup, ":")
	if len(parts) != 2 {
		return "", fmt.Errorf("invalid token lookup format: %s", tokenLookup)
	}

	method := parts[0]
	key := parts[1]
	var tokenString string

	switch method {
	case "header":
		tokenString = c.Request().Header.Get(key)
	case "query":
		tokenString = c.QueryParam(key)
	case "form":
		tokenString = c.FormValue(key)
	case "cookie":
		cookie, err := c.Cookie(key)
		if err != nil {
			return "", fmt.Errorf("token cookie not found: %s", key)
		}
		tokenString = cookie.Value
	default:
		return "", fmt.Errorf("unsupported token lookup method: %s", method)
	}

	if tokenString == "" {
		return "", fmt.Errorf("token not found in %s:%s", method, key)
	}

	// Remove token prefix if present
	if tokenPrefix != "" && strings.HasPrefix(tokenString, tokenPrefix) {
		tokenString = strings.TrimPrefix(tokenString, tokenPrefix)
	}

	if tokenString == "" {
		return "", fmt.Errorf("token is empty after removing prefix")
	}

	return tokenString, nil
}

// isHMACMethod checks if the signing method is HMAC-based
func isHMACMethod(method string) bool {
	return strings.HasPrefix(method, "HS")
}

// loadSigningKeyFromFile loads signing key from file for RSA/ECDSA methods
func loadSigningKeyFromFile(filename, signingMethod string) (interface{}, error) {
	keyData, err := ioutil.ReadFile(filename)
	if err != nil {
		return nil, fmt.Errorf("failed to read key file %s: %w", filename, err)
	}

	block, _ := pem.Decode(keyData)
	if block == nil {
		return nil, fmt.Errorf("failed to parse PEM block from key file %s", filename)
	}

	switch {
	case strings.HasPrefix(signingMethod, "RS"):
		// RSA public key for validation
		if block.Type == "RSA PUBLIC KEY" {
			return x509.ParsePKCS1PublicKey(block.Bytes)
		} else if block.Type == "PUBLIC KEY" {
			key, err := x509.ParsePKIXPublicKey(block.Bytes)
			if err != nil {
				return nil, fmt.Errorf("failed to parse RSA public key: %w", err)
			}
			if rsaKey, ok := key.(*rsa.PublicKey); ok {
				return rsaKey, nil
			}
			return nil, fmt.Errorf("key is not an RSA public key")
		} else {
			return nil, fmt.Errorf("expected RSA public key, got %s", block.Type)
		}

	case strings.HasPrefix(signingMethod, "ES"):
		// ECDSA public key for validation
		if block.Type == "PUBLIC KEY" {
			return x509.ParsePKIXPublicKey(block.Bytes)
		} else {
			return nil, fmt.Errorf("expected ECDSA public key, got %s", block.Type)
		}

	default:
		return nil, fmt.Errorf("unsupported signing method: %s", signingMethod)
	}
}

// GetJWTClaims is a helper function to extract JWT claims from Echo context
func GetJWTClaims(c echo.Context) (jwt.MapClaims, bool) {
	claims, ok := c.Get("jwt_claims").(jwt.MapClaims)
	return claims, ok
}

// GetJWTToken is a helper function to extract JWT token from Echo context
func GetJWTToken(c echo.Context) (*jwt.Token, bool) {
	token, ok := c.Get("jwt_token").(*jwt.Token)
	return token, ok
}

// GetJWTUser is a helper function to extract user from JWT claims
func GetJWTUser(c echo.Context) (string, bool) {
	claims, ok := GetJWTClaims(c)
	if !ok {
		return "", false
	}

	// Try different common user claim names
	if sub, exists := claims["sub"]; exists {
		if subStr, ok := sub.(string); ok {
			return subStr, true
		}
	}

	if username, exists := claims["username"]; exists {
		if usernameStr, ok := username.(string); ok {
			return usernameStr, true
		}
	}

	if user, exists := claims["user"]; exists {
		if userStr, ok := user.(string); ok {
			return userStr, true
		}
	}

	return "", false
}

// ValidateJWTConfig validates JWT configuration
func ValidateJWTConfig(config *JWTAuthConfig) error {
	if config == nil {
		return fmt.Errorf("JWT configuration is required")
	}

	// Check signing method
	supportedMethods := []string{
		"HS256", "HS384", "HS512",
		"RS256", "RS384", "RS512",
		"ES256", "ES384", "ES512",
	}

	validMethod := false
	for _, method := range supportedMethods {
		if config.SigningMethod == method {
			validMethod = true
			break
		}
	}

	if !validMethod {
		return fmt.Errorf("unsupported signing method: %s", config.SigningMethod)
	}

	// Check key configuration
	if isHMACMethod(config.SigningMethod) {
		if config.SigningKey == "" {
			return fmt.Errorf("SigningKey is required for HMAC signing methods")
		}
	} else {
		if config.SigningKeyFile == "" {
			return fmt.Errorf("SigningKeyFile is required for RSA/ECDSA signing methods")
		}

		// Check if key file exists
		if _, err := os.Stat(config.SigningKeyFile); os.IsNotExist(err) {
			return fmt.Errorf("signing key file does not exist: %s", config.SigningKeyFile)
		}
	}

	// Validate token lookup format
	if config.TokenLookup != "" {
		parts := strings.Split(config.TokenLookup, ":")
		if len(parts) != 2 {
			return fmt.Errorf("invalid TokenLookup format, expected 'method:key', got: %s", config.TokenLookup)
		}

		method := parts[0]
		validMethods := []string{"header", "query", "form", "cookie"}
		validLookupMethod := false
		for _, vm := range validMethods {
			if method == vm {
				validLookupMethod = true
				break
			}
		}

		if !validLookupMethod {
			return fmt.Errorf("invalid token lookup method: %s", method)
		}
	}

	return nil
}
