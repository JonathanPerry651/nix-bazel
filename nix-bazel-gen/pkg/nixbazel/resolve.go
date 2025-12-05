package nixbazel

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
)

func RunResolve(configFile, lockFile, channel string, doFetch bool) error {
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

	// Try to read existing lockfile
	var existingLock Lockfile
	if lockData, err := os.ReadFile(lockFile); err == nil {
		json.Unmarshal(lockData, &existingLock)
	}

	lock := Lockfile{
		Repositories: make(map[string]RepositoryLock),
		Packages:     make(map[string]ClosureNode),
	}

	// Copy existing packages to new lock to avoid re-resolving if possible
	if existingLock.Packages != nil {
		lock.Packages = existingLock.Packages
	}

	for name, repoConfig := range config.Repositories {
		// Check if we can reuse existing resolution
		if existingRepo, ok := existingLock.Repositories[name]; ok {
			// If package name matches (simple check), reuse
			// In a real implementation we might want stricter checks
			fmt.Printf("Using cached resolution for %s\n", name)
			lock.Repositories[name] = existingRepo

			// Ensure closure is present in lock.Packages
			// If missing, we might need to re-resolve, but for now assume consistency
			continue
		}

		fmt.Printf("Resolving %s (%s)...\n", name, repoConfig.Package)

		// 1. Resolve to store path (Hydra or direct hash)
		hash := extractHash(repoConfig.Package)
		storePath := ""
		if hash == "" {
			path, err := f.resolveHydra(context.Background(), repoConfig.Package, channel)
			if err != nil {
				return fmt.Errorf("failed to resolve %s: %w", repoConfig.Package, err)
			}
			storePath = path
			hash = extractHash(storePath)
		}

		// 2. Build closure
		// We pass the global packages map to resolveClosure to populate it directly
		rootInfo, err := f.resolveClosure(context.Background(), hash, lock.Packages)
		if err != nil {
			return fmt.Errorf("failed to resolve closure for %s: %w", repoConfig.Package, err)
		}

		if storePath == "" {
			storePath = rootInfo.StorePath
		}

		lock.Repositories[name] = RepositoryLock{
			StorePath:  storePath,
			Entrypoint: repoConfig.Entrypoint,
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

	if doFetch {
		// Generate build files
		fmt.Println("Generating build files...")
		// Use current directory as outDir
		f.outDir = "."
		if err := f.FetchAllFromLock(&lock); err != nil {
			return fmt.Errorf("failed to generate build files: %w", err)
		}
	}

	return nil
}
