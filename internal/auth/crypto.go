package auth

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
)

// KeySize is the length of the secret key in bytes.
const KeySize = 32

// LoadOrCreateKey reads the 32-byte secret key at path, creating it (mode 0600, written
// atomically) with random bytes when it does not exist.
func LoadOrCreateKey(path string) ([]byte, error) {
	b, err := os.ReadFile(path)
	if err == nil {
		if len(b) != KeySize {
			return nil, fmt.Errorf("secret key %s: expected %d bytes, found %d", path, KeySize, len(b))
		}
		return b, nil
	}
	if !errors.Is(err, os.ErrNotExist) {
		return nil, fmt.Errorf("reading secret key: %w", err)
	}
	key := make([]byte, KeySize)
	if _, err := rand.Read(key); err != nil {
		return nil, err
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return nil, err
	}
	tmp, err := os.CreateTemp(filepath.Dir(path), ".secret-*.tmp")
	if err != nil {
		return nil, fmt.Errorf("creating secret key: %w", err)
	}
	defer func() { _ = os.Remove(tmp.Name()) }()
	_ = tmp.Chmod(0o600) // CreateTemp already uses 0600 on Unix; best effort elsewhere
	if _, err := tmp.Write(key); err != nil {
		_ = tmp.Close()
		return nil, err
	}
	if err := tmp.Sync(); err != nil {
		_ = tmp.Close()
		return nil, err
	}
	if err := tmp.Close(); err != nil {
		return nil, err
	}
	if err := os.Rename(tmp.Name(), path); err != nil {
		return nil, fmt.Errorf("writing secret key: %w", err)
	}
	return key, nil
}

// Crypto encrypts stored passwords with AES-256-GCM. Passwords must be reversible because
// Subsonic token authentication (t = md5(password + salt)) needs the plain password.
type Crypto struct {
	aead cipher.AEAD
}

// NewCrypto creates a Crypto from a key; keys that are not 32 bytes are stretched with
// SHA-256.
func NewCrypto(key []byte) *Crypto {
	if len(key) != KeySize {
		sum := sha256.Sum256(key)
		key = sum[:]
	}
	block, err := aes.NewCipher(key)
	if err != nil {
		panic("auth: aes.NewCipher: " + err.Error()) // impossible with a 32-byte key
	}
	aead, err := cipher.NewGCM(block)
	if err != nil {
		panic("auth: cipher.NewGCM: " + err.Error())
	}
	return &Crypto{aead: aead}
}

// ErrDecrypt is returned when a ciphertext cannot be decrypted (wrong key or corrupted).
var ErrDecrypt = errors.New("cannot decrypt value")

// Encrypt returns base64(nonce || ciphertext).
func (c *Crypto) Encrypt(plain string) (string, error) {
	nonce := make([]byte, c.aead.NonceSize())
	if _, err := io.ReadFull(rand.Reader, nonce); err != nil {
		return "", err
	}
	out := c.aead.Seal(nonce, nonce, []byte(plain), nil)
	return base64.StdEncoding.EncodeToString(out), nil
}

// Decrypt reverses Encrypt.
func (c *Crypto) Decrypt(enc string) (string, error) {
	raw, err := base64.StdEncoding.DecodeString(enc)
	if err != nil {
		return "", ErrDecrypt
	}
	ns := c.aead.NonceSize()
	if len(raw) < ns+c.aead.Overhead() {
		return "", ErrDecrypt
	}
	plain, err := c.aead.Open(nil, raw[:ns], raw[ns:], nil)
	if err != nil {
		return "", ErrDecrypt
	}
	return string(plain), nil
}

// NewToken returns 32 random bytes encoded as unpadded base64url (43 chars).
func NewToken() string {
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		panic("auth: crypto/rand failed: " + err.Error())
	}
	return base64.RawURLEncoding.EncodeToString(b)
}

// HashToken returns the lower-hex SHA-256 of a token (what the DB stores).
func HashToken(tok string) string {
	sum := sha256.Sum256([]byte(tok))
	return hex.EncodeToString(sum[:])
}
