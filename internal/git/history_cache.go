package git

import (
	"crypto/sha256"
	"encoding/gob"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/lcaohoanq/ghist/internal/history"
)

// diskCache persists fetched FileVersions so that an interrupted traversal can
// resume from the last seen commit rather than restarting from HEAD.
//
// Cache key = sha256(root + "\x00" + filePath + "\x00" + HEAD)[:8].
// Because HEAD is part of the key, a new commit on the branch starts a fresh
// entry. The old entry is left on disk and collected lazily by pruneCache.
//
// Complete=false means the traversal was interrupted; StreamHistory will
// continue from LastHash on the next run.
// Complete=true means the full history was fetched; no git I/O is needed.
type diskCacheEntry struct {
	Root, FilePath, Head string
	Versions             []history.FileVersion
	LastHash             string // hash of the oldest fetched commit
	Complete             bool   // true = reached the root commit
	SavedAt              time.Time
}

const cacheVersion = 1

type diskCacheFile struct {
	Version int
	Entry   diskCacheEntry
}

func cacheDir() (string, error) {
	base, err := os.UserCacheDir()
	if err != nil {
		return "", err
	}
	dir := filepath.Join(base, "ghist", "history")
	return dir, os.MkdirAll(dir, 0o700)
}

func cacheFilename(root, filePath, head string) string {
	h := sha256.Sum256([]byte(root + "\x00" + filePath + "\x00" + head))
	return fmt.Sprintf("%x.gob", h[:8])
}

// loadCache returns the cached entry for this (root, filePath, head) triple.
// Returns ok=false if no valid cache exists.
func loadCache(root, filePath, head string) (diskCacheEntry, bool) {
	dir, err := cacheDir()
	if err != nil {
		return diskCacheEntry{}, false
	}
	path := filepath.Join(dir, cacheFilename(root, filePath, head))
	f, err := os.Open(path)
	if err != nil {
		return diskCacheEntry{}, false
	}
	defer f.Close()

	var cf diskCacheFile
	if err := gob.NewDecoder(f).Decode(&cf); err != nil {
		return diskCacheEntry{}, false
	}
	if cf.Version != cacheVersion {
		return diskCacheEntry{}, false
	}
	e := cf.Entry
	// Sanity-check: reject if the stored identity doesn't match.
	if e.Root != root || e.FilePath != filePath || e.Head != head {
		return diskCacheEntry{}, false
	}
	return e, true
}

// saveCache atomically writes the entry to disk via a temp-file rename so a
// crash during the write never leaves a corrupt cache file.
func saveCache(e diskCacheEntry) {
	dir, err := cacheDir()
	if err != nil {
		return
	}
	tmp, err := os.CreateTemp(dir, "ghist-*.tmp")
	if err != nil {
		return
	}
	tmpName := tmp.Name()

	cf := diskCacheFile{Version: cacheVersion, Entry: e}
	if err := gob.NewEncoder(tmp).Encode(cf); err != nil {
		tmp.Close()
		os.Remove(tmpName)
		return
	}
	if err := tmp.Close(); err != nil {
		os.Remove(tmpName)
		return
	}
	dest := filepath.Join(dir, cacheFilename(e.Root, e.FilePath, e.Head))
	// os.Rename is atomic on POSIX; on Windows it will replace the dest file.
	if err := os.Rename(tmpName, dest); err != nil {
		os.Remove(tmpName)
	}
}
