package nixbazel

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"io/fs"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"zombiezen.com/go/nix/nar"
)

type Fetcher struct {
	cacheURL string
	outDir   string
	client   *http.Client
	// Cache for resolved narinfos to avoid re-fetching during resolve
	narInfoCache map[string]*NarInfo
}

func NewFetcher(cacheURL, outDir string) *Fetcher {
	return &Fetcher{
		cacheURL:     strings.TrimRight(cacheURL, "/"),
		outDir:       outDir,
		client:       http.DefaultClient,
		narInfoCache: make(map[string]*NarInfo),
	}
}

// FetchAllFromLock downloads and unpacks all packages in the lockfile
func (f *Fetcher) FetchAllFromLock(lock *Lockfile) error {
	// Collect all unique store paths
	uniquePaths := make(map[string]*NarInfo)

	for storePath, node := range lock.Packages {
		uniquePaths[storePath] = &NarInfo{
			URL:         node.URL,
			StorePath:   storePath,
			References:  node.References,
			Compression: "xz",
		}
	}

	fmt.Printf("Fetching %d unique store paths...\n", len(uniquePaths))

	// Download and unpack
	for _, info := range uniquePaths {
		if err := f.downloadAndUnpack(context.Background(), info); err != nil {
			return fmt.Errorf("failed to fetch %s: %w", info.StorePath, err)
		}
		if err := f.patchBinaries(info); err != nil {
			fmt.Printf("Warning: patching failed for %s: %v\n", info.StorePath, err)
		}
	}

	return f.generateBuildFiles(*lock, uniquePaths, "")
}

// FetchFromLock downloads and unpacks a specific repository from the lockfile
func (f *Fetcher) FetchFromLock(lock *Lockfile, repoName string) error {
	storePath, ok := lock.Repositories[repoName]
	if !ok {
		return fmt.Errorf("repository %s not found in lockfile", repoName)
	}

	// Traverse to find the closure
	closure := make(map[string]ClosureNode)
	var traverse func(path string)
	traverse = func(path string) {
		if _, seen := closure[path]; seen {
			return
		}
		node, ok := lock.Packages[path]
		if !ok {
			fmt.Printf("Warning: package %s not found in lockfile packages\n", path)
			return
		}
		closure[path] = node
		for _, ref := range node.References {
			traverse(ref)
		}
	}
	traverse(storePath)

	for path, node := range closure {
		info := &NarInfo{
			URL:         node.URL,
			StorePath:   path,
			References:  node.References,
			Compression: "xz",
		}
		if err := f.downloadAndUnpack(context.Background(), info); err != nil {
			return err
		}
		if err := f.patchBinaries(info); err != nil {
			fmt.Printf("Warning: patching failed for %s: %v\n", path, err)
		}
	}

	// Generate BUILD file for the root package
	rootInfo := &NarInfo{
		StorePath: storePath,
	}
	return f.generateBuildFile(rootInfo)
}

func (f *Fetcher) Fetch(ctx context.Context, storePathOrHash string) error {
	hash := extractHash(storePathOrHash)
	if hash == "" {
		return fmt.Errorf("invalid store path or hash: %s", storePathOrHash)
	}

	// Check if already fetched
	// TODO: Better check

	fmt.Printf("Fetching info for %s...\n", hash)
	narInfo, err := f.getNarInfo(ctx, hash)
	if err != nil {
		return fmt.Errorf("failed to get narinfo for %s: %w", hash, err)
	}

	// Recursively fetch dependencies
	for _, ref := range narInfo.References {
		refHash := extractHash(ref)
		if refHash == hash {
			continue // Self-reference
		}
		if err := f.Fetch(ctx, refHash); err != nil {
			return err
		}
	}

	// Download and unpack NAR
	if err := f.downloadAndUnpack(ctx, narInfo); err != nil {
		return err
	}

	// Patch ELF binaries
	if err := f.patchBinaries(narInfo); err != nil {
		fmt.Printf("Warning: patching failed: %v\n", err)
	}

	// Generate BUILD file
	return f.generateBuildFile(narInfo)
}

func (f *Fetcher) getNarInfo(ctx context.Context, hash string) (*NarInfo, error) {
	url := fmt.Sprintf("%s/%s.narinfo", f.cacheURL, hash)
	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return nil, err
	}

	resp, err := f.client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("unexpected status code: %d", resp.StatusCode)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	info := &NarInfo{}
	lines := strings.Split(string(body), "\n")
	for _, line := range lines {
		parts := strings.SplitN(line, ": ", 2)
		if len(parts) != 2 {
			continue
		}
		key, val := parts[0], parts[1]
		switch key {
		case "URL":
			info.URL = val
		case "Compression":
			info.Compression = val
		case "References":
			if val != "" {
				info.References = strings.Split(val, " ")
			}
		case "StorePath":
			info.StorePath = val
		case "NarHash":
			info.NarHash = val
		case "NarSize":
			fmt.Sscanf(val, "%d", &info.NarSize)
		case "FileHash":
			info.FileHash = val
		case "FileSize":
			fmt.Sscanf(val, "%d", &info.FileSize)
		}
	}
	return info, nil
}

func (f *Fetcher) downloadAndUnpack(ctx context.Context, info *NarInfo) error {
	storeName := filepath.Base(info.StorePath)
	destDir := filepath.Join(f.outDir, storeName)

	if _, err := os.Stat(destDir); err == nil {
		fmt.Printf("Already exists: %s\n", destDir)
		return nil
	}

	fmt.Printf("Downloading %s...\n", info.URL)
	url := fmt.Sprintf("%s/%s", f.cacheURL, info.URL)

	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return err
	}

	resp, err := f.client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("download failed: %d", resp.StatusCode)
	}

	// Handle compression
	var r io.Reader = resp.Body
	if info.Compression == "xz" {
		// Use external xz command for now
		cmd := exec.Command("xz", "-d", "-c")
		cmd.Stdin = resp.Body
		pipe, err := cmd.StdoutPipe()
		if err != nil {
			return err
		}
		if err := cmd.Start(); err != nil {
			return err
		}
		r = pipe
	}

	// Unpack NAR
	fmt.Printf("Unpacking to %s...\n", destDir)
	return f.unpackNar(r, destDir)
}

func (f *Fetcher) unpackNar(r io.Reader, destDir string) error {
	narReader := nar.NewReader(r)

	for {
		hdr, err := narReader.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			return err
		}

		path := filepath.Join(destDir, hdr.Path)

		if hdr.Mode.IsDir() {
			if err := os.MkdirAll(path, 0755); err != nil {
				return err
			}
		} else if hdr.Mode.IsRegular() {
			file, err := os.Create(path)
			if err != nil {
				return err
			}
			if _, err := io.Copy(file, narReader); err != nil {
				file.Close()
				return err
			}
			file.Close()
			if hdr.Mode&0111 != 0 {
				os.Chmod(path, 0755)
			}
		} else if hdr.Mode&fs.ModeSymlink != 0 {
			if err := os.Symlink(hdr.LinkTarget, path); err != nil {
				return err
			}
		}
	}
	return nil
}

func (f *Fetcher) UnpackAndPatch(archivePath, storePath string, refs []string) error {
	// We unpack to f.outDir (repo root).
	// The NAR contains the directory structure (storePathBase/...).
	// So binaries will be in f.outDir/storePathBase/bin.

	storeBase := filepath.Base(storePath)
	actualStoreDir := filepath.Join(f.outDir, storeBase)

	if err := os.MkdirAll(actualStoreDir, 0755); err != nil {
		return err
	}

	fmt.Printf("Unpacking %s to %s...\n", archivePath, actualStoreDir)

	file, err := os.Open(archivePath)
	if err != nil {
		return err
	}
	defer file.Close()

	var r io.Reader = file
	// Assume xz
	cmd := exec.Command("xz", "-d", "-c")
	cmd.Stdin = file
	pipe, err := cmd.StdoutPipe()
	if err != nil {
		return err
	}
	if err := cmd.Start(); err != nil {
		return err
	}
	r = pipe

	if err := f.unpackNar(r, actualStoreDir); err != nil {
		return err
	}

	// Patch binaries
	info := &NarInfo{
		StorePath:  actualStoreDir,
		References: refs,
	}
	return f.patchBinaries(info)
}

func (f *Fetcher) patchBinaries(info *NarInfo) error {
	storeName := filepath.Base(info.StorePath)
	destDir := filepath.Join(f.outDir, storeName)

	// Construct RPATH
	var rpaths []string
	for _, ref := range info.References {
		refName := filepath.Base(ref)
		// $ORIGIN/../<ref>/lib
		rpaths = append(rpaths, fmt.Sprintf("$ORIGIN/../%s/lib", refName))
		rpaths = append(rpaths, fmt.Sprintf("$ORIGIN/../%s/lib64", refName))
	}
	rpathStr := strings.Join(rpaths, ":")

	return filepath.Walk(destDir, func(path string, info fs.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if !info.Mode().IsRegular() {
			return nil
		}
		// Check if executable or shared lib
		// Simple check: executable bit or .so extension
		isElf := (info.Mode()&0111 != 0) || strings.HasSuffix(path, ".so") || strings.Contains(path, ".so.")
		if !isElf {
			return nil
		}

		// Run patchelf
		fmt.Printf("Patching %s with RPATH %s\n", path, rpathStr)
		cmd := exec.Command("patchelf", "--set-rpath", rpathStr, path)
		if output, err := cmd.CombinedOutput(); err != nil {
			// Ignore errors for now as not all files are valid ELFs or writable
			fmt.Printf("patchelf failed for %s: %v %s\n", path, err, output)
		}
		return nil
	})
}

func (f *Fetcher) resolveHydra(ctx context.Context, packageId, channel string) (string, error) {
	// Try multiple jobsets
	jobsets := []string{
		"nixpkgs/trunk",        // Nixpkgs (Darwin/Linux) - Try this first!
		"nixos/trunk-combined", // NixOS (Linux)
	}

	if channel != "" {
		jobsets = []string{channel}
	}

	var lastErr error
	for _, jobset := range jobsets {
		// Adjust packageId based on jobset conventions
		jobName := packageId
		if strings.HasPrefix(jobset, "nixpkgs/") {
			jobName = strings.TrimPrefix(packageId, "nixpkgs.")
		}

		url := fmt.Sprintf("https://hydra.nixos.org/job/%s/%s/latest", jobset, jobName)
		fmt.Printf("Resolving %s via Hydra (%s)...\n", packageId, url)

		req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
		if err != nil {
			return "", err
		}
		req.Header.Set("Accept", "application/json")

		resp, err := f.client.Do(req)
		if err != nil {
			lastErr = err
			continue
		}
		defer resp.Body.Close()

		if resp.StatusCode != http.StatusOK {
			lastErr = fmt.Errorf("hydra request failed: %d", resp.StatusCode)
			continue
		}

		var result struct {
			BuildOutputs struct {
				Out struct {
					Path string `json:"path"`
				} `json:"out"`
			} `json:"buildoutputs"`
		}

		if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
			lastErr = fmt.Errorf("failed to decode hydra response: %w", err)
			continue
		}

		if result.BuildOutputs.Out.Path == "" {
			lastErr = fmt.Errorf("no output path found in hydra response")
			continue
		}

		fmt.Printf("Resolved to: %s\n", result.BuildOutputs.Out.Path)
		return result.BuildOutputs.Out.Path, nil
	}

	return "", fmt.Errorf("failed to resolve %s in any jobset: %v", packageId, lastErr)
}

func (f *Fetcher) resolveClosure(ctx context.Context, hash string, closure map[string]ClosureNode) (*NarInfo, error) {
	// Check cache
	if _, ok := closure[hash]; ok {
		return f.narInfoCache[hash], nil
	}

	info, err := f.getNarInfo(ctx, hash)
	if err != nil {
		return nil, err
	}
	f.narInfoCache[hash] = info

	node := ClosureNode{
		URL:        info.URL,
		Hash:       hash,
		References: info.References,
		NarHash:    convertHashToHex(info.NarHash),
		NarSize:    info.NarSize,
		FileHash:   convertHashToHex(info.FileHash),
		FileSize:   info.FileSize,
	}
	closure[info.StorePath] = node

	// Recurse
	for _, ref := range info.References {
		refHash := extractHash(ref)
		if refHash == hash {
			continue
		}
		// Check if we already visited this store path (to avoid infinite recursion/re-work)
		if _, ok := closure[ref]; ok {
			continue
		}

		if _, err := f.resolveClosure(ctx, refHash, closure); err != nil {
			return nil, err
		}
	}

	return info, nil
}
