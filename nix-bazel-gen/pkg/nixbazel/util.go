package nixbazel

import (
	"fmt"
	"path/filepath"
	"strings"

	"zombiezen.com/go/nix/nixbase32"
)

func extractHash(s string) string {
	base := filepath.Base(s)
	parts := strings.Split(base, "-")
	if len(parts) > 0 && len(parts[0]) == 32 {
		return parts[0]
	}
	return ""
}

func convertHashToHex(narHash string) string {
	if strings.HasPrefix(narHash, "sha256:") {
		hashPart := strings.TrimPrefix(narHash, "sha256:")
		// Try decoding as nixbase32
		decoded, err := nixbase32.DecodeString(hashPart)
		if err == nil {
			return fmt.Sprintf("%x", decoded)
		}
		// If not base32, maybe it's already hex? or base64?
		// For now assume nixbase32 as that's what Hydra returns.
	}
	return ""
}
