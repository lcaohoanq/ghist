package git

import (
	"context"
	"fmt"
	"strings"
)

// Files lists blobs from the captured HEAD across the entire repository.
// NUL records preserve unusual paths; submodule entries are not file history.
func (r *Repository) Files(ctx context.Context) ([]string, error) {
	out, err := run(ctx, r.Root, "ls-tree", "-r", "-z", "--full-tree", r.Head)
	if err != nil {
		return nil, err
	}
	var paths []string
	for _, entry := range strings.Split(string(out), "\x00") {
		if entry == "" {
			continue
		}
		metadata, path, ok := strings.Cut(entry, "\t")
		fields := strings.Fields(metadata)
		if !ok || len(fields) != 3 {
			return nil, fmt.Errorf("invalid Git tree response")
		}
		if fields[1] == "blob" {
			paths = append(paths, path)
		}
	}
	return paths, nil
}
