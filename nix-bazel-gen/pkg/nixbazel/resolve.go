package nixbazel

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
)

func RunResolve(configFile, lockFile, channel string) error {
	// Read config
	data, err := os.ReadFile(configFile)
	if err != nil {
		return fmt.Errorf("failed to read config: %w", err)
	}
	var config Config
	if err := json.Unmarshal(data, &config); err != nil {
		return fmt.Errorf("failed to parse config: %w", err)
	}

	f := NewFetcher(defaultCacheURL, "")
	lock := Lockfile{
		Repositories: make(map[string]string),
		Packages:     make(map[string]ClosureNode),
	}

	for name, id := range config.Repositories {
		fmt.Printf("Resolving %s (%s)...\n", name, id)

		// 1. Resolve to store path (Hydra or direct hash)
		hash := extractHash(id)
		storePath := ""
		if hash == "" {
			path, err := f.resolveHydra(context.Background(), id, channel)
			if err != nil {
				return fmt.Errorf("failed to resolve %s: %w", id, err)
			}
			storePath = path
			hash = extractHash(storePath)
		}

		// 2. Build closure
		// We pass the global packages map to resolveClosure to populate it directly
		rootInfo, err := f.resolveClosure(context.Background(), hash, lock.Packages)
		if err != nil {
			return fmt.Errorf("failed to resolve closure for %s: %w", id, err)
		}

		if storePath == "" {
			storePath = rootInfo.StorePath
		}

		lock.Repositories[name] = storePath
	}

	// Write lockfile
	lockData, err := json.MarshalIndent(lock, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal lockfile: %w", err)
	}
	if err := os.WriteFile(lockFile, lockData, 0644); err != nil {
		return fmt.Errorf("failed to write lockfile: %w", err)
	}
	fmt.Printf("Generated %s\n", lockFile)
	return nil
}
