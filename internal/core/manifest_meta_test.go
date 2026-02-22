package core

import (
	"bytes"
	"encoding/json"
	"testing"
)

func TestManifestMetaRoundtrip(t *testing.T) {
	meta := &ManifestMeta{
		V:        ManifestMetaVersion,
		Rememory: "v0.0.16",
		Tlock: &TlockMeta{
			V:      TlockMetaVersion,
			Method: TlockMethodQuicknet,
			Round:  12345678,
			Unlock: "2027-06-15T00:00:00Z",
			Chain:  QuicknetChainHash,
		},
	}

	innerData := []byte("this is the inner ciphertext data")

	// Write envelope + inner data
	var buf bytes.Buffer
	if err := WriteManifestMeta(&buf, meta, bytes.NewReader(innerData)); err != nil {
		t.Fatalf("WriteManifestMeta: %v", err)
	}

	result := buf.Bytes()

	// Should start with '{'
	if !HasManifestMeta(result) {
		t.Fatal("HasManifestMeta returned false for enveloped data")
	}

	// Parse it back
	parsed, inner, err := ParseManifestMeta(result)
	if err != nil {
		t.Fatalf("ParseManifestMeta: %v", err)
	}

	// Verify envelope fields
	if parsed.V != ManifestMetaVersion {
		t.Errorf("V: got %d, want %d", parsed.V, ManifestMetaVersion)
	}
	if parsed.Rememory != "v0.0.16" {
		t.Errorf("Rememory: got %q, want %q", parsed.Rememory, "v0.0.16")
	}
	if parsed.Tlock == nil {
		t.Fatal("Tlock is nil")
	}
	if parsed.Tlock.V != TlockMetaVersion {
		t.Errorf("Tlock.V: got %d, want %d", parsed.Tlock.V, TlockMetaVersion)
	}
	if parsed.Tlock.Method != TlockMethodQuicknet {
		t.Errorf("Tlock.Method: got %q, want %q", parsed.Tlock.Method, TlockMethodQuicknet)
	}
	if parsed.Tlock.Round != 12345678 {
		t.Errorf("Tlock.Round: got %d, want %d", parsed.Tlock.Round, 12345678)
	}
	if parsed.Tlock.Unlock != "2027-06-15T00:00:00Z" {
		t.Errorf("Tlock.Unlock: got %q, want %q", parsed.Tlock.Unlock, "2027-06-15T00:00:00Z")
	}
	if parsed.Tlock.Chain != QuicknetChainHash {
		t.Errorf("Tlock.Chain: got %q, want %q", parsed.Tlock.Chain, QuicknetChainHash)
	}

	// Verify inner data
	if !bytes.Equal(inner, innerData) {
		t.Errorf("inner data mismatch: got %q, want %q", inner, innerData)
	}
}

func TestHasManifestMeta(t *testing.T) {
	tests := []struct {
		name string
		data []byte
		want bool
	}{
		{
			name: "JSON envelope",
			data: []byte(`{"v":1,"rememory":"v0.0.16"}` + "\ninner data"),
			want: true,
		},
		{
			name: "regular age file",
			data: []byte("age-encryption.org/v1\n-> scrypt ..."),
			want: false,
		},
		{
			name: "empty data",
			data: []byte{},
			want: false,
		},
		{
			name: "nil data",
			data: nil,
			want: false,
		},
		{
			name: "binary data",
			data: []byte{0x00, 0x01, 0x02},
			want: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := HasManifestMeta(tt.data)
			if got != tt.want {
				t.Errorf("HasManifestMeta: got %v, want %v", got, tt.want)
			}
		})
	}
}

func TestManifestMetaFutureFields(t *testing.T) {
	// Simulate an envelope with unknown future fields — they should not
	// cause parsing errors and should be silently ignored.
	raw := `{"v":1,"rememory":"v0.1.0","tlock":{"v":1,"method":"drand-quicknet","round":999,"unlock":"2030-01-01T00:00:00Z","chain":"abc123"},"expiry":"2099-01-01T00:00:00Z","signature":"deadbeef"}` + "\ninner"

	meta, inner, err := ParseManifestMeta([]byte(raw))
	if err != nil {
		t.Fatalf("ParseManifestMeta: %v", err)
	}

	if meta.V != 1 {
		t.Errorf("V: got %d, want 1", meta.V)
	}
	if meta.Rememory != "v0.1.0" {
		t.Errorf("Rememory: got %q, want %q", meta.Rememory, "v0.1.0")
	}
	if meta.Tlock == nil {
		t.Fatal("Tlock is nil")
	}
	if meta.Tlock.Round != 999 {
		t.Errorf("Tlock.Round: got %d, want 999", meta.Tlock.Round)
	}
	if string(inner) != "inner" {
		t.Errorf("inner: got %q, want %q", inner, "inner")
	}

	// Write it back and verify the round-tripped JSON still decodes
	var buf bytes.Buffer
	if err := WriteManifestMeta(&buf, meta, bytes.NewReader(inner)); err != nil {
		t.Fatalf("WriteManifestMeta: %v", err)
	}

	meta2, inner2, err := ParseManifestMeta(buf.Bytes())
	if err != nil {
		t.Fatalf("re-parse: %v", err)
	}
	if meta2.V != meta.V || meta2.Rememory != meta.Rememory {
		t.Error("round-trip lost envelope fields")
	}
	if !bytes.Equal(inner2, inner) {
		t.Error("round-trip lost inner data")
	}
}

func TestManifestMetaNoNewline(t *testing.T) {
	// A JSON header with no trailing newline should fail
	data := []byte(`{"v":1,"rememory":"v0.0.16"}`)
	_, _, err := ParseManifestMeta(data)
	if err == nil {
		t.Error("expected error for missing newline")
	}
}

func TestManifestMetaMissingVersion(t *testing.T) {
	data := []byte(`{"rememory":"v0.0.16"}` + "\ninner")
	_, _, err := ParseManifestMeta(data)
	if err == nil {
		t.Error("expected error for missing version")
	}
}

func TestManifestMetaNoTlock(t *testing.T) {
	// An envelope without tlock (future extensibility)
	meta := &ManifestMeta{
		V:        ManifestMetaVersion,
		Rememory: "v0.0.16",
	}

	var buf bytes.Buffer
	if err := WriteManifestMeta(&buf, meta, bytes.NewReader([]byte("data"))); err != nil {
		t.Fatalf("WriteManifestMeta: %v", err)
	}

	parsed, inner, err := ParseManifestMeta(buf.Bytes())
	if err != nil {
		t.Fatalf("ParseManifestMeta: %v", err)
	}

	if parsed.Tlock != nil {
		t.Error("Tlock should be nil when not set")
	}
	if string(inner) != "data" {
		t.Errorf("inner: got %q, want %q", inner, "data")
	}
}

func TestTlockMetaUnlockTime(t *testing.T) {
	meta := &TlockMeta{
		Unlock: "2027-06-15T00:00:00Z",
	}

	ut, err := meta.UnlockTime()
	if err != nil {
		t.Fatalf("UnlockTime: %v", err)
	}
	if ut.Year() != 2027 || ut.Month() != 6 || ut.Day() != 15 {
		t.Errorf("unexpected time: %v", ut)
	}
}

func TestManifestMetaJSONCompact(t *testing.T) {
	// Verify that the written JSON is compact (single line, no indentation)
	meta := &ManifestMeta{
		V:        1,
		Rememory: "v0.0.16",
		Tlock: &TlockMeta{
			V:      1,
			Method: TlockMethodQuicknet,
			Round:  100,
			Unlock: "2025-01-01T00:00:00Z",
			Chain:  "abc",
		},
	}

	var buf bytes.Buffer
	if err := WriteManifestMeta(&buf, meta, nil); err != nil {
		t.Fatalf("WriteManifestMeta: %v", err)
	}

	output := buf.String()
	lines := bytes.Count([]byte(output), []byte("\n"))
	if lines != 1 {
		t.Errorf("expected exactly 1 newline (compact JSON + trailing), got %d", lines)
	}

	// Verify it's valid JSON on the first line
	firstLine := output[:len(output)-1] // strip trailing newline
	if !json.Valid([]byte(firstLine)) {
		t.Error("header is not valid JSON")
	}
}
