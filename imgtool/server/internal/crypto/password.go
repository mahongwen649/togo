package crypto

import (
	"crypto/rand"
	"crypto/subtle"
	"encoding/base64"
	"fmt"
	"strconv"
	"strings"

	"golang.org/x/crypto/argon2"
)

const (
	argonMemory      = 64 * 1024
	argonIterations  = 3
	argonParallelism = 1
	argonSaltLength  = 16
	argonKeyLength   = 32
)

func HashPassword(password string) (string, error) {
	salt := make([]byte, argonSaltLength)
	if _, err := rand.Read(salt); err != nil {
		return "", err
	}
	key := argon2.IDKey([]byte(password), salt, argonIterations, argonMemory, argonParallelism, argonKeyLength)
	return fmt.Sprintf(
		"$argon2id$v=19$m=%d,t=%d,p=%d$%s$%s",
		argonMemory,
		argonIterations,
		argonParallelism,
		base64.RawStdEncoding.EncodeToString(salt),
		base64.RawStdEncoding.EncodeToString(key),
	), nil
}

func VerifyPassword(encodedHash string, password string) bool {
	params, salt, expected, ok := parsePasswordHash(encodedHash)
	if !ok {
		return false
	}
	actual := argon2.IDKey([]byte(password), salt, params.iterations, params.memory, params.parallelism, uint32(len(expected)))
	return subtle.ConstantTimeCompare(actual, expected) == 1
}

type passwordParams struct {
	memory      uint32
	iterations  uint32
	parallelism uint8
}

func parsePasswordHash(encodedHash string) (passwordParams, []byte, []byte, bool) {
	parts := strings.Split(encodedHash, "$")
	if len(parts) != 6 || parts[1] != "argon2id" || parts[2] != "v=19" {
		return passwordParams{}, nil, nil, false
	}
	params, ok := parseParams(parts[3])
	if !ok {
		return passwordParams{}, nil, nil, false
	}
	salt, err := base64.RawStdEncoding.DecodeString(parts[4])
	if err != nil {
		return passwordParams{}, nil, nil, false
	}
	hash, err := base64.RawStdEncoding.DecodeString(parts[5])
	if err != nil {
		return passwordParams{}, nil, nil, false
	}
	return params, salt, hash, true
}

func parseParams(input string) (passwordParams, bool) {
	values := map[string]string{}
	for _, part := range strings.Split(input, ",") {
		key, value, ok := strings.Cut(part, "=")
		if !ok {
			return passwordParams{}, false
		}
		values[key] = value
	}
	memory, err := strconv.ParseUint(values["m"], 10, 32)
	if err != nil {
		return passwordParams{}, false
	}
	iterations, err := strconv.ParseUint(values["t"], 10, 32)
	if err != nil {
		return passwordParams{}, false
	}
	parallelism, err := strconv.ParseUint(values["p"], 10, 8)
	if err != nil {
		return passwordParams{}, false
	}
	if memory == 0 || iterations == 0 || parallelism == 0 {
		return passwordParams{}, false
	}
	return passwordParams{
		memory:      uint32(memory),
		iterations:  uint32(iterations),
		parallelism: uint8(parallelism),
	}, true
}
