package crypto

import (
	"crypto/md5"
	"crypto/sha1"
	"crypto/sha256"
	"io"
)

// ComputeSHA1 calculates the SHA1 checksum for the data read from r.
func ComputeSHA1(r io.Reader) ([]byte, error) {
	h := sha1.New()
	if _, err := io.Copy(h, r); err != nil {
		return nil, err
	}
	return h.Sum(nil), nil
}

// ComputeMD5 calculates the MD5 checksum for the data read from r.
func ComputeMD5(r io.Reader) ([]byte, error) {
	h := md5.New()
	if _, err := io.Copy(h, r); err != nil {
		return nil, err
	}
	return h.Sum(nil), nil
}

// ComputeSHA256 calculates the SHA256 checksum for the data read from r.
func ComputeSHA256(r io.Reader) ([]byte, error) {
	h := sha256.New()
	if _, err := io.Copy(h, r); err != nil {
		return nil, err
	}
	return h.Sum(nil), nil
}
