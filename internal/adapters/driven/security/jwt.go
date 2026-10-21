package security

import (
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/efangly/smtrack-authentication/config"
	"github.com/efangly/smtrack-authentication/internal/core/domain"
	"github.com/efangly/smtrack-authentication/internal/core/ports"
	"github.com/golang-jwt/jwt/v5"
)

const (
	typAccess  = "access"
	typRefresh = "refresh"
)

// Verification errors are domain errors (they wrap nothing) so the HTTP layer
// may show them to the client.
var (
	errInvalidToken        = errors.New("invalid or expired token")
	errInvalidRefreshToken = errors.New("invalid or expired refresh token")
)

type jwtIssuer struct {
	accessSecret  []byte
	refreshSecret []byte
	accessTTL     time.Duration
	refreshTTL    time.Duration
	now           func() time.Time
}

// NewJWTIssuer creates an HS256 token issuer. Access and refresh tokens are
// signed with different secrets and carry a "typ" claim, so neither can stand
// in for the other.
func NewJWTIssuer(cfg *config.Config) ports.TokenIssuer {
	return &jwtIssuer{
		accessSecret:  []byte(cfg.JWTSecret),
		refreshSecret: []byte(cfg.JWTRefreshSecret),
		accessTTL:     parseDuration(cfg.ExpireTime, time.Hour),
		refreshTTL:    parseDuration(cfg.RefreshExpireTime, 7*24*time.Hour),
		now:           time.Now,
	}
}

func (j *jwtIssuer) IssuePair(actor domain.Actor) (ports.TokenPair, error) {
	access, err := j.sign(actor, typAccess, j.accessTTL, j.accessSecret)
	if err != nil {
		return ports.TokenPair{}, fmt.Errorf("failed to sign token: %w", err)
	}
	refresh, err := j.sign(actor, typRefresh, j.refreshTTL, j.refreshSecret)
	if err != nil {
		return ports.TokenPair{}, fmt.Errorf("failed to sign refresh token: %w", err)
	}
	return ports.TokenPair{Access: access, Refresh: refresh}, nil
}

func (j *jwtIssuer) sign(actor domain.Actor, typ string, ttl time.Duration, secret []byte) (string, error) {
	now := j.now()
	claims := jwt.MapClaims{
		"id":     actor.ID,
		"name":   actor.Name,
		"role":   string(actor.Role),
		"hosId":  actor.HosID,
		"wardId": actor.WardID,
		"typ":    typ,
		"exp":    now.Add(ttl).Unix(),
		"iat":    now.Unix(),
	}
	return jwt.NewWithClaims(jwt.SigningMethodHS256, claims).SignedString(secret)
}

func (j *jwtIssuer) VerifyAccess(token string) (*domain.Actor, error) {
	claims, ok := j.parse(token, typAccess, j.accessSecret)
	if !ok {
		return nil, errInvalidToken
	}
	return &domain.Actor{
		ID:     claimString(claims, "id"),
		Name:   claimString(claims, "name"),
		Role:   domain.Role(claimString(claims, "role")),
		HosID:  claimString(claims, "hosId"),
		WardID: claimString(claims, "wardId"),
	}, nil
}

func (j *jwtIssuer) VerifyRefresh(token string) (string, error) {
	claims, ok := j.parse(token, typRefresh, j.refreshSecret)
	if !ok {
		return "", errInvalidRefreshToken
	}
	userID := claimString(claims, "id")
	if userID == "" {
		return "", errInvalidRefreshToken
	}
	return userID, nil
}

// parse verifies signature, algorithm, expiry and token type.
func (j *jwtIssuer) parse(token, wantTyp string, secret []byte) (jwt.MapClaims, bool) {
	parsed, err := jwt.Parse(token, func(t *jwt.Token) (any, error) {
		if _, ok := t.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, fmt.Errorf("unexpected signing method: %v", t.Header["alg"])
		}
		return secret, nil
	}, jwt.WithTimeFunc(j.now))
	if err != nil || !parsed.Valid {
		return nil, false
	}
	claims, ok := parsed.Claims.(jwt.MapClaims)
	if !ok {
		return nil, false
	}
	// Tokens issued before "typ" existed carry no type. They are still
	// accepted (the separate secrets already keep the two kinds apart) so a
	// deploy does not sign everyone out.
	// TODO: reject a missing typ once REFRESH_EXPIRE_TIME has passed after
	// this shipped; every token in circulation will carry it by then.
	if typ, present := claims["typ"]; present && typ != wantTyp {
		return nil, false
	}
	return claims, true
}

func claimString(claims jwt.MapClaims, key string) string {
	if s, ok := claims[key].(string); ok {
		return s
	}
	return ""
}

// parseDuration accepts Go durations ("1h", "90m") and a day suffix ("7d"),
// falling back to def for anything empty or invalid.
func parseDuration(s string, def time.Duration) time.Duration {
	if s == "" {
		return def
	}
	if strings.HasSuffix(s, "d") {
		var days int
		// time.ParseDuration has no day unit, so "7d" is handled here.
		if n, err := fmt.Sscanf(s, "%dd", &days); err != nil || n != 1 || days <= 0 {
			return def
		}
		return time.Duration(days) * 24 * time.Hour
	}
	d, err := time.ParseDuration(s)
	if err != nil {
		return def
	}
	return d
}
