package xar

import (
	"crypto"
	"encoding/binary"
	"encoding/xml"
	"fmt"
	"io"
)

func parseTOC(r io.Reader, hashType crypto.Hash) (*toc, error) {
	tocHash := hashType.New()
	r = io.TeeReader(r, tocHash)
	decomp, err := decompress(r)
	if err != nil {
		return nil, fmt.Errorf("decompressing TOC: %w", err)
	}
	var toc tocXar
	if err := xml.Unmarshal(decomp, &toc); err != nil {
		return nil, fmt.Errorf("decoding TOC: %w", err)
	}
	return &toc.TOC, nil
}

func parseHeader(r io.Reader) (xarHeader, crypto.Hash, error) {
	var hdr xarHeader
	if err := binary.Read(r, binary.BigEndian, &hdr); err != nil {
		return xarHeader{}, 0, err
	}

	if hdr.Magic != xarMagic {
		return hdr, 0, ErrInvalidType
	}

	var hashType crypto.Hash
	switch hdr.HashType {
	case hashSHA1:
		hashType = crypto.SHA1
	case hashSHA256:
		hashType = crypto.SHA256
	case hashSHA512:
		hashType = crypto.SHA512
	default:
		return xarHeader{}, 0, fmt.Errorf("unknown hash algorithm %d", hdr.HashType)
	}

	return hdr, hashType, nil
}
