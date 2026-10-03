package git

import (
	"bytes"
	"context"
	"io"
)

const contentLimit = 2 << 20

// Drain even oversized results so Git can exit normally without retaining its
// entire output. The user can explicitly request an unlimited full view.
func runContent(ctx context.Context, dir string, full bool, args ...string) ([]byte, bool, error) {
	if full {
		out, err := run(ctx, dir, args...)
		return out, false, err
	}
	var out bytes.Buffer
	err := stream(ctx, dir, func(reader io.Reader) error {
		if _, err := io.Copy(&out, io.LimitReader(reader, contentLimit+1)); err != nil {
			return err
		}
		_, err := io.Copy(io.Discard, reader)
		return err
	}, args...)
	truncated := out.Len() > contentLimit
	data := out.Bytes()
	if truncated {
		data = data[:contentLimit]
	}
	return data, truncated, err
}
