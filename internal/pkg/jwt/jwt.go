package jwt

import (
	"errors"
	"strconv"

	jwtlib "github.com/golang-jwt/jwt/v5"
)

var (
	ErrInvalidToken = errors.New("invalid or expired token")
)

type Claims struct {
	jwtlib.RegisteredClaims
	Email       string `json:"email"`
	CompanyID   uint   `json:"company_id"`
	IsSuperuser bool   `json:"is_superuser"`
}

type Manager struct {
	secret []byte
}

func NewManager(secret string) *Manager {
	return &Manager{
		secret: []byte(secret),
	}
}

// ValidateToken parses and validates a JWT string.
func (m *Manager) ValidateToken(tokenString string) (*Claims, error) {
	token, err := jwtlib.ParseWithClaims(tokenString, &Claims{}, func(t *jwtlib.Token) (interface{}, error) {
		if _, ok := t.Method.(*jwtlib.SigningMethodHMAC); !ok {
			return nil, ErrInvalidToken
		}
		return m.secret, nil
	})
	if err != nil {
		return nil, ErrInvalidToken
	}
	claims, ok := token.Claims.(*Claims)
	if !ok || !token.Valid {
		return nil, ErrInvalidToken
	}
	return claims, nil
}

func ParseUserID(subject string) uint {
	id, _ := strconv.ParseUint(subject, 10, 64)
	return uint(id)
}
