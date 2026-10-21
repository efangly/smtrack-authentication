package security

import (
	"fmt"

	"github.com/efangly/smtrack-authentication/internal/core/ports"
	"golang.org/x/crypto/bcrypt"
)

// bcryptCost is the work factor used for every password hash in this service.
const bcryptCost = 10

type bcryptHasher struct {
	dummy []byte
}

// NewBcryptHasher creates the password hasher. It precomputes a hash that
// matches nothing, so CompareDummy costs exactly what a real comparison does.
func NewBcryptHasher() ports.PasswordHasher {
	dummy, err := bcrypt.GenerateFromPassword([]byte("dummy-password"), bcryptCost)
	if err != nil {
		// Only fails on an invalid cost, which is a constant.
		panic(fmt.Sprintf("bcrypt: %v", err))
	}
	return &bcryptHasher{dummy: dummy}
}

func (b *bcryptHasher) Hash(plain string) (string, error) {
	hashed, err := bcrypt.GenerateFromPassword([]byte(plain), bcryptCost)
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
