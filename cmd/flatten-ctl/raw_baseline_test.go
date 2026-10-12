package main

import (
	"context"
	"errors"
	"fmt"
	"github.com/kuasar-sandbox/accelerator/pkg/sparse"
	"github.com/kuasar-sandbox/accelerator/pkg/tailzip"
	"github.com/kuasar-sandbox/accelerator/pkg/tarstream"
	"io"
	"os"
)

// Legacy raw packing is retained only as an independent regression oracle.
// packImageArtifact wraps the raw erofs(+config ZIP) at rawPath as a
// tarstream artifact (payload "image" + digest marker) on w: the platform
// container for images. Holes come from the filesystem (the raw file
// is the live scratch source); the artifact itself is dense and
// self-describing.
func packImageArtifact(rawPath string, w io.Writer) (retErr error) {
	f, err := os.Open(rawPath)
	if err != nil {
		return fmt.Errorf("open erofs: %w", err)
	}
	defer func() {
		if err := f.Close(); retErr == nil && err != nil {
			retErr = fmt.Errorf("close erofs: %w", err)
		}
	}()
	st, err := f.Stat()
	if err != nil {
		return err
	}
	holes, err := sparse.ProbeHoles(f)
	if err != nil {
		return fmt.Errorf("probe holes: %w", err)
	}
	src, err := sparse.NewSource(f, uint64(st.Size()), holes)
	if err != nil {
		return err
	}
	tail, boundary, err := tailzip.Read(f, st.Size(), tailzip.Options{})
	if err != nil && !errors.Is(err, tailzip.ErrNotFound) {
		return fmt.Errorf("image tail: %w", err)
	}
	if err == nil {
		payload, err := tailzip.Prefix(src, uint64(boundary.Offset))
		if err != nil {
			return err
		}
		src, err = tailzip.Append(payload, tail, tailzip.Options{})
		if err != nil {
			return err
		}
	}
	if _, _, err := tarstream.WriteTo(context.Background(), w, "image", src); err != nil {
		return fmt.Errorf("pack image artifact: %w", err)
	}
	return nil
}
