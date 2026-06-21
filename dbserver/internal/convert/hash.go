// Package convert maps the raw save structs (savefmt) into the relational
// domain model, hashing all secrets on import. The legacy plaintext passwords
// and PINs (data-formats.md §1.3) are NEVER carried into the new stack —
// migration-plan.md §5.
package convert

import (
	"crypto/rand"
	"encoding/base64"
	"fmt"

	"golang.org/x/crypto/argon2"
)

// argonParams are the argon2id cost parameters used for all imported secrets.
type argonParams struct {
	memory  uint32
	time    uint32
	threads uint8
	keyLen  uint32
	saltLen uint32
}

// defaultArgon follows OWASP's argon2id guidance (64 MiB, t=1, p=4).
var defaultArgon = argonParams{memory: 64 * 1024, time: 1, threads: 4, keyLen: 32, saltLen: 16}

// HashSecret returns an argon2id PHC-encoded hash of plain, or "" when plain is
// empty (an unset secret — e.g. no block password). A fresh random salt is used
// per call, so the same input yields different hashes.
func HashSecret(plain string) (string, error) {
	if plain == "" {
		return "", nil
	}
	salt := make([]byte, defaultArgon.saltLen)
	if _, err := rand.Read(salt); err != nil {
		return "", fmt.Errorf("convert: generate salt: %w", err)
	}
	h := argon2.IDKey([]byte(plain), salt, defaultArgon.time, defaultArgon.memory, defaultArgon.threads, defaultArgon.keyLen)
	b64 := base64.RawStdEncoding
	return fmt.Sprintf("$argon2id$v=%d$m=%d,t=%d,p=%d$%s$%s",
		argon2.Version, defaultArgon.memory, defaultArgon.time, defaultArgon.threads,
		b64.EncodeToString(salt), b64.EncodeToString(h)), nil
}
