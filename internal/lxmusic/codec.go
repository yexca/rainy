package lxmusic

import (
	"bytes"
	"compress/zlib"
	"crypto/aes"
	"crypto/cipher"
	"crypto/md5"
	"crypto/rsa"
	"crypto/x509"
	"encoding/base64"
	"encoding/binary"
	"encoding/hex"
	"encoding/pem"
	"errors"
	"fmt"
	"io"
	"math/big"
	"regexp"
	"strings"
	"unicode/utf16"

	"golang.org/x/text/encoding/htmlindex"
)

// Byte and string helpers behind lx.utils (Node's Buffer, crypto and zlib as lx-music
// exposes them) and the browser globals atob, btoa and TextDecoder.

// maxInflated bounds zlib output.
const maxInflated = 32 << 20

// nodeEncoding normalises a Node.js Buffer encoding name.
func nodeEncoding(enc string) (string, error) {
	switch strings.ToLower(strings.TrimSpace(enc)) {
	case "", "utf8", "utf-8":
		return "utf8", nil
	case "hex":
		return "hex", nil
	case "base64":
		return "base64", nil
	case "base64url":
		return "base64url", nil
	case "binary", "latin1":
		return "latin1", nil
	case "ascii":
		return "ascii", nil
	case "ucs2", "ucs-2", "utf16le", "utf-16le":
		return "utf16le", nil
	}
	return "", fmt.Errorf("unknown encoding: %s", enc)
}

// encodeString converts a string to bytes like Buffer.from(string, encoding).
func encodeString(s, enc string) ([]byte, error) {
	enc, err := nodeEncoding(enc)
	if err != nil {
		return nil, err
	}
	switch enc {
	case "hex":
		// Node decodes pairs until the first invalid one.
		n := len(s) / 2
		out := make([]byte, 0, n)
		for i := 0; i < n; i++ {
			b, err := hex.DecodeString(s[2*i : 2*i+2])
			if err != nil {
				break
			}
			out = append(out, b[0])
		}
		return out, nil
	case "base64", "base64url":
		return decodeBase64Lenient(s), nil
	case "latin1", "ascii":
		units := utf16.Encode([]rune(s))
		out := make([]byte, len(units))
		for i, u := range units {
			out[i] = byte(u)
		}
		return out, nil
	case "utf16le":
		units := utf16.Encode([]rune(s))
		out := make([]byte, 2*len(units))
		for i, u := range units {
			binary.LittleEndian.PutUint16(out[2*i:], u)
		}
		return out, nil
	}
	return []byte(s), nil
}

// decodeBytes converts bytes to a string like buffer.toString(encoding).
func decodeBytes(b []byte, enc string) (string, error) {
	enc, err := nodeEncoding(enc)
	if err != nil {
		return "", err
	}
	switch enc {
	case "hex":
		return hex.EncodeToString(b), nil
	case "base64":
		return base64.StdEncoding.EncodeToString(b), nil
	case "base64url":
		return base64.RawURLEncoding.EncodeToString(b), nil
	case "latin1":
		return latin1String(b), nil
	case "ascii":
		r := make([]rune, len(b))
		for i, c := range b {
			r[i] = rune(c & 0x7f)
		}
		return string(r), nil
	case "utf16le":
		units := make([]uint16, len(b)/2)
		for i := range units {
			units[i] = binary.LittleEndian.Uint16(b[2*i:])
		}
		return string(utf16.Decode(units)), nil
	}
	return strings.ToValidUTF8(string(b), replacementChar), nil
}

// Written as escapes so the source holds no invisible characters.
const (
	byteOrderMark   = "\xef\xbb\xbf"
	replacementChar = "\xef\xbf\xbd"
)

func latin1String(b []byte) string {
	r := make([]rune, len(b))
	for i, c := range b {
		r[i] = rune(c)
	}
	return string(r)
}

var base64Junk = regexp.MustCompile(`[^A-Za-z0-9+/\-_]`)

// decodeBase64Lenient decodes standard or URL-safe base64, ignoring padding, whitespace and
// other characters (like Node.js).
func decodeBase64Lenient(s string) []byte {
	s = base64Junk.ReplaceAllString(s, "")
	s = strings.NewReplacer("-", "+", "_", "/").Replace(s)
	if len(s)%4 == 1 {
		s = s[:len(s)-1]
	}
	out, err := base64.RawStdEncoding.DecodeString(s)
	if err != nil {
		return nil
	}
	return out
}

// btoa implements the browser function: Latin-1 text to base64.
func btoa(s string) (string, error) {
	b := make([]byte, 0, len(s))
	for _, r := range s {
		if r > 0xff {
			return "", errors.New("the string to be encoded contains characters outside of the Latin1 range")
		}
		b = append(b, byte(r))
	}
	return base64.StdEncoding.EncodeToString(b), nil
}

// atob implements the browser function: base64 to Latin-1 text.
func atob(s string) (string, error) {
	s = strings.Map(func(r rune) rune {
		if r == ' ' || r == '\t' || r == '\n' || r == '\f' || r == '\r' {
			return -1
		}
		return r
	}, s)
	if len(s)%4 == 0 {
		s = strings.TrimSuffix(strings.TrimSuffix(s, "="), "=")
	}
	invalid := errors.New("the string to be decoded is not correctly encoded")
	if len(s)%4 == 1 || strings.ContainsFunc(s, func(r rune) bool {
		return !isBase64Char(r)
	}) {
		return "", invalid
	}
	b, err := base64.RawStdEncoding.DecodeString(s)
	if err != nil {
		return "", invalid
	}
	return latin1String(b), nil
}

func isBase64Char(r rune) bool {
	return r >= 'A' && r <= 'Z' || r >= 'a' && r <= 'z' || r >= '0' && r <= '9' || r == '+' || r == '/'
}

// textDecode implements TextDecoder.decode for the WHATWG encoding label.
func textDecode(b []byte, label string) (string, error) {
	name, err := textEncoding(label)
	if err != nil {
		return "", err
	}
	if name == "utf-8" {
		return strings.ToValidUTF8(strings.TrimPrefix(string(b), byteOrderMark), replacementChar), nil
	}
	e, err := htmlindex.Get(name)
	if err != nil {
		return "", err
	}
	out, err := e.NewDecoder().Bytes(b)
	if err != nil {
		return "", err
	}
	return string(out), nil
}

// textEncoding returns the canonical name of a WHATWG encoding label.
func textEncoding(label string) (string, error) {
	label = strings.ToLower(strings.TrimSpace(label))
	if label == "" || label == "utf8" || label == "utf-8" || label == "unicode-1-1-utf-8" {
		return "utf-8", nil
	}
	e, err := htmlindex.Get(label)
	if err != nil {
		return "", fmt.Errorf("the encoding label provided ('%s') is invalid", label)
	}
	name, err := htmlindex.Name(e)
	if err != nil {
		return "", err
	}
	return name, nil
}

// aesEncrypt implements lx.utils.crypto.aesEncrypt(buffer, mode, key, iv) for the Node.js
// cipher names aes-128/192/256-ecb, -cbc and -ctr (PKCS#7 padding for ECB and CBC).
func aesEncrypt(data []byte, mode string, key, iv []byte) ([]byte, error) {
	name := strings.ToLower(strings.TrimSpace(mode))
	switch name {
	case "aes128":
		name = "aes-128-cbc"
	case "aes192":
		name = "aes-192-cbc"
	case "aes256":
		name = "aes-256-cbc"
	}
	parts := strings.Split(name, "-")
	if len(parts) != 3 || parts[0] != "aes" {
		return nil, fmt.Errorf("invalid cipher: %s", mode)
	}
	bits := map[string]int{"128": 16, "192": 24, "256": 32}[parts[1]]
	if bits == 0 {
		return nil, fmt.Errorf("invalid cipher: %s", mode)
	}
	if len(key) != bits {
		return nil, errors.New("invalid key length")
	}
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, err
	}
	switch parts[2] {
	case "ecb":
		in := pkcs7(data)
		out := make([]byte, len(in))
		for i := 0; i < len(in); i += aes.BlockSize {
			block.Encrypt(out[i:i+aes.BlockSize], in[i:i+aes.BlockSize])
		}
		return out, nil
	case "cbc":
		if len(iv) != aes.BlockSize {
			return nil, errors.New("invalid initialization vector")
		}
		in := pkcs7(data)
		out := make([]byte, len(in))
		cipher.NewCBCEncrypter(block, iv).CryptBlocks(out, in)
		return out, nil
	case "ctr":
		if len(iv) != aes.BlockSize {
			return nil, errors.New("invalid initialization vector")
		}
		out := make([]byte, len(data))
		cipher.NewCTR(block, iv).XORKeyStream(out, data)
		return out, nil
	}
	return nil, fmt.Errorf("unsupported cipher: %s", mode)
}

func pkcs7(data []byte) []byte {
	pad := aes.BlockSize - len(data)%aes.BlockSize
	return append(append([]byte{}, data...), bytes.Repeat([]byte{byte(pad)}, pad)...)
}

// rsaEncrypt implements lx.utils.crypto.rsaEncrypt: the data is left-padded with zeros to
// 128 bytes and encrypted without padding (RSA_NO_PADDING) with a PEM public key.
func rsaEncrypt(data []byte, key string) ([]byte, error) {
	if len(data) > 128 {
		return nil, errors.New("the data is longer than 128 bytes")
	}
	pub, err := parseRSAPublicKey(key)
	if err != nil {
		return nil, err
	}
	size := (pub.N.BitLen() + 7) / 8
	if size != 128 {
		return nil, errors.New("the key must be a 1024-bit RSA key")
	}
	padded := make([]byte, 128)
	copy(padded[128-len(data):], data)
	m := new(big.Int).SetBytes(padded)
	if m.Cmp(pub.N) >= 0 {
		return nil, errors.New("data too large for modulus")
	}
	c := new(big.Int).Exp(m, big.NewInt(int64(pub.E)), pub.N)
	return c.FillBytes(make([]byte, size)), nil
}

var pemArmor = regexp.MustCompile(`-----(BEGIN|END) [A-Z ]+-----`)

func parseRSAPublicKey(key string) (*rsa.PublicKey, error) {
	var der []byte
	if block, _ := pem.Decode([]byte(key)); block != nil {
		der = block.Bytes
	} else {
		// Keys pasted without line breaks.
		body := pemArmor.ReplaceAllString(key, "")
		der = decodeBase64Lenient(body)
	}
	if len(der) == 0 {
		return nil, errors.New("invalid public key")
	}
	if k, err := x509.ParsePKIXPublicKey(der); err == nil {
		if pub, ok := k.(*rsa.PublicKey); ok {
			return pub, nil
		}
		return nil, errors.New("not an RSA public key")
	}
	if pub, err := x509.ParsePKCS1PublicKey(der); err == nil {
		return pub, nil
	}
	return nil, errors.New("invalid public key")
}

func md5Hex(b []byte) string {
	sum := md5.Sum(b) // part of the script API, not a security measure
	return hex.EncodeToString(sum[:])
}

func inflate(b []byte) ([]byte, error) {
	zr, err := zlib.NewReader(bytes.NewReader(b))
	if err != nil {
		return nil, err
	}
	defer func() { _ = zr.Close() }()
	out, err := io.ReadAll(io.LimitReader(zr, maxInflated+1))
	if err != nil {
		return nil, err
	}
	if len(out) > maxInflated {
		return nil, errors.New("inflated data is too large")
	}
	return out, nil
}

func deflate(b []byte) ([]byte, error) {
	var buf bytes.Buffer
	zw := zlib.NewWriter(&buf)
	if _, err := zw.Write(b); err != nil {
		return nil, err
	}
	if err := zw.Close(); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}
