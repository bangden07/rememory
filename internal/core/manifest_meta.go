package core

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"time"
)

// ManifestMetaVersion is the current envelope version.
const ManifestMetaVersion = 1

// TlockMetaVersion is the current tlock scheme version.
const TlockMetaVersion = 1

// ManifestMeta is a versioned metadata envelope prepended to MANIFEST.age
// when features like tlock require it. Regular MANIFEST.age files (starting
// with "age-encryption.org") have no envelope.
//
// Format: a single JSON line followed by a newline, then the inner ciphertext.
type ManifestMeta struct {
	V        int        `json:"v"`
	Rememory string     `json:"rememory"`
	Tlock    *TlockMeta `json:"tlock,omitempty"`
}

// TlockMeta holds the tlock-specific metadata within the envelope.
type TlockMeta struct {
	V      int    `json:"v"`
	Method string `json:"method"`
	Round  uint64 `json:"round"`
	Unlock string `json:"unlock"` // RFC 3339 timestamp
	Chain  string `json:"chain"`
}

// UnlockTime parses the Unlock field as a time.Time.
func (t *TlockMeta) UnlockTime() (time.Time, error) {
	return time.Parse(time.RFC3339, t.Unlock)
}

// HasManifestMeta returns true if data begins with a JSON envelope.
// Regular age files start with "age-encryption.org", so there is no ambiguity.
func HasManifestMeta(data []byte) bool {
	return len(data) > 0 && data[0] == '{'
}

// ParseManifestMeta splits a MANIFEST.age file with a metadata envelope into
// the parsed envelope and the remaining inner ciphertext. Returns an error if
// the data starts with '{' but the envelope is malformed.
func ParseManifestMeta(data []byte) (*ManifestMeta, []byte, error) {
	if !HasManifestMeta(data) {
		return nil, nil, errors.New("data does not contain a manifest metadata envelope")
	}

	// Find the newline that terminates the JSON header
	idx := bytes.IndexByte(data, '\n')
	if idx == -1 {
		return nil, nil, errors.New("manifest metadata envelope: missing newline after JSON header")
	}

	headerLine := data[:idx]
	inner := data[idx+1:]

	var meta ManifestMeta
	if err := json.Unmarshal(headerLine, &meta); err != nil {
		return nil, nil, fmt.Errorf("manifest metadata envelope: parsing JSON: %w", err)
	}

	if meta.V == 0 {
		return nil, nil, errors.New("manifest metadata envelope: missing version field")
	}

	return &meta, inner, nil
}

// WriteManifestMeta writes a JSON metadata envelope line followed by the inner
// ciphertext from src. The caller is responsible for writing the inner data
// after this call if src is nil.
func WriteManifestMeta(dst io.Writer, meta *ManifestMeta, src io.Reader) error {
	headerBytes, err := json.Marshal(meta)
	if err != nil {
		return fmt.Errorf("manifest metadata envelope: encoding JSON: %w", err)
	}

	if _, err := dst.Write(headerBytes); err != nil {
		return fmt.Errorf("manifest metadata envelope: writing header: %w", err)
	}
	if _, err := dst.Write([]byte("\n")); err != nil {
		return fmt.Errorf("manifest metadata envelope: writing newline: %w", err)
	}

	if src != nil {
		if _, err := io.Copy(dst, src); err != nil {
			return fmt.Errorf("manifest metadata envelope: writing inner data: %w", err)
		}
	}

	return nil
}
