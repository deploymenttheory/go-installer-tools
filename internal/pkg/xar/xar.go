package xar

//		Copyright 2023 SAS Software
//
//	 Licensed under the Apache License, Version 2.0 (the "License");
//	 you may not use this file except in compliance with the License.
//	 You may obtain a copy of the License at
//
//	     http://www.apache.org/licenses/LICENSE-2.0
//
//	 Unless required by applicable law or agreed to in writing, software
//	 distributed under the License is distributed on an "AS IS" BASIS,
//	 WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
//	 See the License for the specific language governing permissions and
//	 limitations under the License.
//
// xar contains utilities to parse xar files, most of the logic here is a
// simplified version extracted from the logic to sign xar files in
// https://github.com/sassoftware/relic

import (
	"bytes"
	"compress/bzip2"
	"compress/zlib"
	"crypto/sha256"
	"encoding/binary"
	"encoding/xml"
	"errors"
	"fmt"
	"io"
	"strings"

	hash "github.com/deploymenttheory/go-installer-tools/internal/crypto"
	"github.com/deploymenttheory/go-installer-tools/internal/logger"
	"github.com/deploymenttheory/go-installer-tools/internal/reader"
)

const (
	// xarMagic is the [file signature][1] (or magic bytes) for xar
	//
	// [1]: https://en.wikipedia.org/wiki/List_of_file_signatures
	xarMagic = 0x78617221

	xarHeaderSize = 28
)

const (
	hashNone uint32 = iota
	hashSHA1
	hashMD5
	hashSHA256
	hashSHA512
)

var (
	// ErrInvalidType is used to signal that the provided package can't be
	// parsed because is an invalid file type.
	ErrInvalidType = errors.New("invalid file type")
	// ErrNotSigned is used to signal that the provided package doesn't
	// contain a signature.
	ErrNotSigned = errors.New("file is not signed")
)

type xarHeader struct {
	Magic            uint32
	HeaderSize       uint16
	Version          uint16
	CompressedSize   int64
	UncompressedSize int64
	HashType         uint32
}

type tocXar struct {
	TOC toc `xml:"toc"`
}

type toc struct {
	Signature  *any `xml:"signature"`
	XSignature *any `xml:"x-signature"`
}

type xmlXar struct {
	XMLName xml.Name `xml:"xar"`
	TOC     xmlTOC
}

type xmlTOC struct {
	XMLName xml.Name   `xml:"toc"`
	Files   []*xmlFile `xml:"file"`
}

type xmlFileData struct {
	XMLName  xml.Name `xml:"data"`
	Length   int64    `xml:"length"`
	Offset   int64    `xml:"offset"`
	Size     int64    `xml:"size"`
	Encoding struct {
		Style string `xml:"style,attr"`
	} `xml:"encoding"`
}

type xmlFile struct {
	XMLName xml.Name `xml:"file"`
	Name    string   `xml:"name"`
	Type    string   `xml:"type"`
	Data    *xmlFileData
}

// ExtractXARMetadata extracts the name and version metadata from a .pkg file
// in the XAR format. This version skips processing embedded packages.
func ExtractXARMetadata(tfr *reader.TempFileReader) (*PKGInstallerMetadata, error) {
	logger.Debug("Starting XAR metadata extraction")

	var meta *PKGInstallerMetadata
	var isSignedStatus bool

	// Compute SHA256 hash and capture total file size
	sha256Hash := sha256.New()
	size, _ := io.Copy(sha256Hash, tfr)
	logger.Debug("Calculated initial hash and size", "size", size)
	pkgSizeMB := float64(size) / (1024 * 1024)

	// Check for package signature
	if err := tfr.Rewind(); err != nil {
		logger.Error("Failed to rewind reader for signature check", "error", err)
		return nil, fmt.Errorf("rewind reader for signature check: %w", err)
	}

	// Check signature status
	err := CheckPKGSignature(tfr)
	if err == nil {
		isSignedStatus = true
		logger.Debug("Package is signed")
	} else if err == ErrNotSigned {
		isSignedStatus = false
		logger.Debug("Package is not signed")
	} else if err == ErrInvalidType {
		logger.Error("Invalid XAR file type")
		return nil, err
	} else {
		logger.Error("Error checking package signature", "error", err)
		return nil, fmt.Errorf("check package signature: %w", err)
	}

	if err := tfr.Rewind(); err != nil {
		logger.Error("Failed to rewind reader", "error", err)
		return nil, fmt.Errorf("rewind reader: %w", err)
	}

	// Read the file header
	var hdr xarHeader
	if err := binary.Read(tfr, binary.BigEndian, &hdr); err != nil {
		logger.Error("Failed to decode XAR header", "error", err)
		return nil, fmt.Errorf("decode xar header: %w", err)
	}

	// Read TOC
	var root xmlXar
	tocReader, err := zlib.NewReader(io.LimitReader(tfr, hdr.CompressedSize))
	if err != nil {
		logger.Error("Failed to create TOC reader", "error", err)
		return nil, fmt.Errorf("create TOC reader: %w", err)
	}
	defer tocReader.Close()

	if err := xml.NewDecoder(tocReader).Decode(&root); err != nil {
		logger.Error("Failed to decode TOC XML", "error", err)
		return nil, fmt.Errorf("decode TOC XML: %w", err)
	}

	// Dump full TOC structure for analysis
	tocBytes, _ := xml.MarshalIndent(root, "", "  ")
	logger.Debug("Full TOC Structure", "toc", string(tocBytes))
	logger.Debug("Successfully decoded XAR XML TOC", "fileCount", len(root.TOC.Files))

	// Calculate base offset for all files
	heapOffset := xarHeaderSize + hdr.CompressedSize

	// Variables to hold raw metadata file contents
	var distributionContents []byte
	var packageInfoContents []byte

	// Loop through TOC entries and collect metadata file contents
	for _, f := range root.TOC.Files {
		if f == nil || f.Data == nil {
			continue
		}
		logger.Debug("Examining metadata file", "name", f.Name, "offset", f.Data.Offset, "length", f.Data.Length)
		contents, err := readCompressedFile(tfr, heapOffset, size, f)
		if err != nil {
			logger.Error("Failed to read file", "name", f.Name, "error", err)
			continue
		}
		switch f.Name {
		case "Distribution":
			logger.Debug("Found Distribution file", "size", len(contents))
			distributionContents = contents
		case "PackageInfo":
			logger.Debug("Found PackageInfo file", "size", len(contents))
			packageInfoContents = contents
		}
	}

	// Prefer the Distribution file if present
	if distributionContents != nil {
		logger.Debug("Processing Distribution file")
		if distMeta, err := parseDistributionFile(distributionContents); err != nil {
			logger.Error("Failed to parse Distribution", "error", err)
		} else {
			meta = distMeta
			meta.IsSigned = isSignedStatus // Set the signature status we detected earlier
		}
	}

	// Fallback: if Distribution wasn't found or parsed, try PackageInfo
	if meta == nil && packageInfoContents != nil {
		logger.Debug("Processing PackageInfo file as fallback")
		if pkgMeta, err := parsePackageInfoFile(packageInfoContents); err != nil {
			logger.Error("Failed to parse PackageInfo", "error", err)
		} else {
			meta = pkgMeta
			meta.IsSigned = isSignedStatus // Set the signature status we detected earlier
		}
	}

	// Finalize metadata
	if meta == nil {
		logger.Warn("No metadata found, returning minimal metadata")
		meta = &PKGInstallerMetadata{
			SHA256Sum: sha256Hash.Sum(nil),
			IsSigned:  false, // Set default value for new packages
		}
	} else {
		meta.SHA256Sum = sha256Hash.Sum(nil)
		meta.PkgSizeMB = pkgSizeMB
		// IsSigned is already set from earlier check
	}

	// Compute additional hashes
	if err := tfr.Rewind(); err != nil {
		logger.Error("Failed to rewind for SHA1", "error", err)
	} else {
		sha1Sum, err := hash.ComputeSHA1(tfr)
		if err != nil {
			logger.Error("Failed to compute SHA1", "error", err)
		} else {
			meta.SHA1Sum = sha1Sum
		}
	}
	if err := tfr.Rewind(); err != nil {
		logger.Error("Failed to rewind for MD5", "error", err)
	} else {
		md5Sum, err := hash.ComputeMD5(tfr)
		if err != nil {
			logger.Error("Failed to compute MD5", "error", err)
		} else {
			meta.MD5Sum = md5Sum
		}
	}
	if err := tfr.Rewind(); err != nil {
		logger.Error("Failed to rewind for SHA256", "error", err)
	} else {
		sha256Sum, err := hash.ComputeSHA256(tfr)
		if err != nil {
			logger.Error("Failed to compute SHA256", "error", err)
		} else {
			// Overwrite our previous SHA256 if needed
			meta.SHA256Sum = sha256Sum
		}
	}

	return meta, nil
}

func readCompressedFile(rat io.ReaderAt, heapOffset int64, sectionLength int64, f *xmlFile) ([]byte, error) {
	if f == nil {
		return nil, fmt.Errorf("nil file provided")
	}

	if f.Data == nil {
		return nil, fmt.Errorf("file has no data section")
	}

	var fileReader io.Reader
	heapReader := io.NewSectionReader(rat, heapOffset, sectionLength-heapOffset)
	fileReader = io.NewSectionReader(heapReader, f.Data.Offset, f.Data.Length)

	// the distribution file can be compressed differently than the TOC, the
	// actual compression is specified in the Encoding.Style field.
	if strings.Contains(f.Data.Encoding.Style, "x-gzip") {
		// despite the name, x-gzip fails to decode with the gzip package
		// (invalid header), but it works with zlib.
		logger.Debug("Using zlib decompression")
		zr, err := zlib.NewReader(fileReader)
		if err != nil {
			return nil, fmt.Errorf("create zlib reader: %w", err)
		}
		defer zr.Close()
		fileReader = zr
	} else if strings.Contains(f.Data.Encoding.Style, "x-bzip2") {
		logger.Debug("Using bzip2 decompression")
		fileReader = bzip2.NewReader(fileReader)
	}
	// TODO: what other compression methods are supported?

	contents, err := io.ReadAll(fileReader)
	if err != nil {
		return nil, fmt.Errorf("reading %s file: %w", f.Name, err)
	}

	return contents, nil
}

// CheckPKGSignature checks if the provided bytes correspond to a signed pkg
// (xar) file.
//
// - If the file is not xar, it returns a ErrInvalidType error
// - If the file is not signed, it returns a ErrNotSigned error
func CheckPKGSignature(pkg io.Reader) error {
	buff := bytes.NewBuffer(nil)
	if _, err := io.Copy(buff, pkg); err != nil {
		return err
	}
	r := bytes.NewReader(buff.Bytes())

	hdr, hashType, err := parseHeader(io.NewSectionReader(r, 0, 28))
	if err != nil {
		return err
	}

	base := int64(hdr.HeaderSize)
	toc, err := parseTOC(io.NewSectionReader(r, base, hdr.CompressedSize), hashType)
	if err != nil {
		return err
	}

	if toc.Signature == nil && toc.XSignature == nil {
		return ErrNotSigned
	}

	return nil
}

func decompress(r io.Reader) ([]byte, error) {
	zr, err := zlib.NewReader(r)
	if err != nil {
		return nil, err
	}
	defer zr.Close()
	return io.ReadAll(zr)
}
