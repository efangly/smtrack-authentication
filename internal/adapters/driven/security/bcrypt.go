package security

import (
	"fmt"

	"github.com/efangly/smtrack-authentication/internal/core/ports"
	"golang.org/x/crypto/bcrypt"
)

type bcryptHasher struct {
	cost  int
	dummy []byte
}

// NewBcryptHasher creates the password hasher. It precomputes a hash that
// matches nothing, so CompareDummy costs exactly what a real comparison does.
// cost is the work factor for new hashes; existing hashes keep verifying at whatever cost
// they were made with.
func NewBcryptHasher(cost int) ports.PasswordHasher {
	dummy, err := bcrypt.GenerateFromPassword([]byte("dummy-password"), cost)
	if err != nil {
		// Only fails on an invalid cost, which config.Validate rejects at startup.
		panic(fmt.Sprintf("bcrypt: %v", err))
	}
	return &bcryptHasher{cost: cost, dummy: dummy}
}

func (b *bcryptHasher) Hash(plain string) (string, error) {
	hashed, err := bcrypt.GenerateFromPassword([]byte(plain), b.cost)
	if err != nil {
		return "", fmt.Errorf("failed to hash password: %w", err)
	}
	return string(hashed), nil
}

func (b *bcryptHasher) Compare(hash, plain string) bool {
	return bcrypt.CompareHashAndPassword([]byte(hash), []byte(plain)) == nil
}

func (b *bcryptHasher) CompareDummy(plain string) {
	_ = bcrypt.CompareHashAndPassword(b.dummy, []byte(plain))
}
