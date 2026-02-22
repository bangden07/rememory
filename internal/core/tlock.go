//go:build !js

package core

import (
	"fmt"
	"io"

	"github.com/drand/tlock"
	tlockhttp "github.com/drand/tlock/networks/http"
)

// TlockEncrypt encrypts src to a specific drand round number using tlock.
// The ciphertext can only be decrypted after the drand beacon for that round
// is emitted. A network connection is required to fetch the chain public key.
func TlockEncrypt(dst io.Writer, src io.Reader, roundNumber uint64) error {
	network, err := connectDrand()
	if err != nil {
		return fmt.Errorf("tlock encrypt: %w", err)
	}

	if err := tlock.New(network).Encrypt(dst, src, roundNumber); err != nil {
		return fmt.Errorf("tlock encrypt: %w", err)
	}

	return nil
}

// TlockDecrypt decrypts tlock-encrypted ciphertext by fetching the drand
// beacon signature for the round embedded in the ciphertext.
// Returns tlock.ErrTooEarly if the round has not been reached yet.
func TlockDecrypt(dst io.Writer, src io.Reader) error {
	network, err := connectDrand()
	if err != nil {
		return fmt.Errorf("tlock decrypt: %w", err)
	}

	if err := tlock.New(network).Decrypt(dst, src); err != nil {
		return fmt.Errorf("tlock decrypt: %w", err)
	}

	return nil
}

// connectDrand tries each drand endpoint until one connects.
func connectDrand() (tlock.Network, error) {
	var lastErr error
	for _, endpoint := range DrandEndpoints {
		network, err := tlockhttp.NewNetwork(endpoint, QuicknetChainHash)
		if err != nil {
			lastErr = err
			continue
		}
		return network, nil
	}
	return nil, fmt.Errorf("connecting to drand: all endpoints failed (last error: %w)", lastErr)
}
