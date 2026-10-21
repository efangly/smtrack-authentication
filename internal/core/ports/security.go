package ports

import "github.com/efangly/smtrack-authentication/internal/core/domain"

// TokenPair is an access token and the refresh token that renews it.
type TokenPair struct {
	Access  string
	Refresh string
}

// TokenIssuer defines the driven port for issuing and verifying session tokens.
//
// Verification failures are returned as domain errors (not wrapping anything)
// so callers can report them to the client as-is.
type TokenIssuer interface {
	// IssuePair signs a fresh access and refresh token for actor.
	IssuePair(actor domain.Actor) (TokenPair, error)
	// VerifyAccess checks an access token and returns the actor it names.
	VerifyAccess(token string) (*domain.Actor, error)
	// VerifyRefresh checks a refresh token and returns the user id it names.
	// The caller must rebuild everything else from the current user record.
	VerifyRefresh(token string) (userID string, err error)
}

// PasswordHasher defines the driven port for one-way password hashing.
type PasswordHasher interface {
	Hash(plain string) (string, error)
	// Compare reports whether plain matches hash.
	Compare(hash, plain string) bool
	// CompareDummy costs the same as a failed Compare. Call it when there is
	// no user to compare against, so response time does not reveal which
	// usernames exist.
	CompareDummy(plain string)
}
