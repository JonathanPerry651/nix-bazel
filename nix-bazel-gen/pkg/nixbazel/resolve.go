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
		Repositories: make(map[string]RepositoryLock),
		Packages:     make(map[string]ClosureNode),
	}

	for name, config := range config.Repositories {
		fmt.Printf("Resolving %s (%s)...\n", name, config.Package)

		// 1. Resolve to store path (Hydra or direct hash)
		hash := extractHash(config.Package)
		storePath := ""
		if hash == "" {
			path, err := f.resolveHydra(context.Background(), config.Package, channel)
			if err != nil {
				return fmt.Errorf("failed to resolve %s: %w", config.Package, err)
			}
			storePath = path
			hash = extractHash(storePath)
		}

		// 2. Build closure
		// We pass the global packages map to resolveClosure to populate it directly
		rootInfo, err := f.resolveClosure(context.Background(), hash, lock.Packages)
		if err != nil {
			return fmt.Errorf("failed to resolve closure for %s: %w", config.Package, err)
		}

		if storePath == "" {
			storePath = rootInfo.StorePath
		}

		lock.Repositories[name] = RepositoryLock{
			StorePath:  storePath,
			Entrypoint: config.Entrypoint,
		}
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
