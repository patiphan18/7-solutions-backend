package security

import "golang.org/x/crypto/bcrypt"

type Bcrypt struct{}

// A missing account still performs a normal-cost comparison.
var missingAccountHash = func() []byte {
	hash, err := bcrypt.GenerateFromPassword([]byte("missing-account-dummy-password"), bcrypt.DefaultCost)
	if err != nil {
		panic(err)
	}

	return hash
}()

func (Bcrypt) Hash(password string) (string, error) {
	hash, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	return string(hash), err
}

func (Bcrypt) Compare(hash, password string) bool {
	if len(password) > 72 {
		return false
	}

	if hash == "" {
		_ = bcrypt.CompareHashAndPassword(missingAccountHash, []byte(password))
		return false
	}

	return bcrypt.CompareHashAndPassword([]byte(hash), []byte(password)) == nil
}
