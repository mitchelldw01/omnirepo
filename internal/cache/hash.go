package cache

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"os"
)

// Stateful hasher that will only hash the same file path once.
// When it encounters a file path it's already hashed, it will use the previous result.
type sha256Hasher struct {
	hashes *concurrentMap[string]
}

func newSha256Hasher() *sha256Hasher {
	return &sha256Hasher{
		hashes: newConcurrentMap[string](),
	}
}

func (h *sha256Hasher) hash(paths ...string) ([]string, error) {
	hashes := []string{}

	for _, path := range paths {
		if hash, ok := h.hashes.get(path); ok {
			hashes = append(hashes, hash)
			continue
		}

		hash, err := h.computeHash(path)
		if err != nil {
			return nil, fmt.Errorf("failed to hash file %q: %v", path, err)
		}
		h.hashes.put(path, hash)
		hashes = append(hashes, hash)
	}

	return hashes, nil
}

func (h *sha256Hasher) computeHash(path string) (string, error) {
	file, err := os.Open(path)
	if err != nil {
		return "", err
	}
	defer file.Close()

	hash := sha256.New()
	if _, err := hash.Write([]byte(path)); err != nil {
		return "", err
	}

	if _, err := io.Copy(hash, file); err != nil {
		return "", err
	}

	return hex.EncodeToString(hash.Sum(nil)), nil
}
