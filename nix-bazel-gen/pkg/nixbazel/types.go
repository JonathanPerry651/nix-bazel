package nixbazel

const defaultCacheURL = "https://cache.nixos.org"

// Config represents nix_deps.yaml
type Config struct {
	Repositories map[string]string `json:"repositories"`
}

// Lockfile represents nix_deps.lock.json
type Lockfile struct {
	Repositories map[string]string      `json:"repositories"` // name -> store path
	Packages     map[string]ClosureNode `json:"packages"`     // store path -> node
}

type ClosureNode struct {
	URL        string   `json:"url"`
	Hash       string   `json:"hash"`
	Size       int64    `json:"size"`
	NarHash    string   `json:"narHash"` // Hex encoded SHA256 of uncompressed NAR
	NarSize    int64    `json:"narSize"`
	FileHash   string   `json:"fileHash"` // Hex encoded SHA256 of compressed file
	FileSize   int64    `json:"fileSize"`
	References []string `json:"references"`
}

type NarInfo struct {
	URL         string
	Compression string
	References  []string
	StorePath   string
	NarHash     string
	NarSize     int64
	FileHash    string
	FileSize    int64
}
