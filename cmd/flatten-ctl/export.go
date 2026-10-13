package main

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"syscall"

	"github.com/kuasar-sandbox/accelerator/pkg/flatten"
	"github.com/kuasar-sandbox/accelerator/pkg/manifest"
	"github.com/kuasar-sandbox/accelerator/pkg/remote"
)

func prepareDirExport(ctx context.Context, root, tmpDir string, skips []string, skipMounts bool, runtimeConfig string, noProgress bool, excluded []string) (*flatten.PreparedImage, error) {
	dopts := flatten.DirOptions{Skip: skips, SkipMounts: skipMounts, OutputPaths: excluded}
	if !noProgress {
		dopts.Warnf = func(format string, a ...any) { fmt.Fprintf(os.Stderr, format+"\n", a...) }
	}
	if runtimeConfig != "" {
		f, err := os.Open(runtimeConfig)
		if err != nil {
			return nil, fmt.Errorf("--runtime-config: %w", err)
		}
		data, err := io.ReadAll(io.LimitReader(f, 16<<20+1))
		err = errors.Join(err, f.Close())
		if err != nil {
			return nil, fmt.Errorf("--runtime-config: %w", err)
		}
		if len(data) > 16<<20 {
			return nil, fmt.Errorf("--runtime-config exceeds 16 MiB")
		}
		dopts.ConfigJSON = data
	}
	return flatten.PrepareDir(ctx, root, flatten.Options{TmpDir: tmpDir, Progress: flattenProgress(!noProgress)}, dopts)
}

func prepareRemoteExport(ctx context.Context, ref string, cfg *remote.Config, printDigest, noProgress bool) (*flatten.PreparedImage, error) {
	res, err := cfg.Resolve(ctx, ref)
	if err != nil {
		return nil, err
	}
	if !noProgress {
		fmt.Fprintf(os.Stderr, "resolved: %s\n", res.Digest)
	}
	if printDigest {
		if _, err := fmt.Println(res.Digest.String()); err != nil {
			return nil, fmt.Errorf("write source digest: %w", err)
		}
	}
	return cfg.Prepare(ctx, res, flatten.Options{TmpDir: cfg.TmpDir, Progress: flattenProgress(!noProgress)})
}

func writePreparedExport(ctx context.Context, output string, upload bool, tmpDir string, mcfg *manifest.Config, noProgress bool, prepare func([]string) (*flatten.PreparedImage, error), ops exportIO) (retErr error) {
	if output == "-" {
		if st, err := os.Stdout.Stat(); err == nil && st.Mode()&os.ModeCharDevice != 0 {
			return fmt.Errorf("export: refusing to write an image artifact to a terminal (use --output FILE or redirect stdout)")
		}
	}
	// Reserve a destination before planning so paths inside the source and
	// aliases are excluded by the same selection used at every validation.
	// Only upload/re-read needs a complete temporary P; stdout has none.
	named := output != "" && output != "-"
	if named {
		if _, _, err := outputPermissions(output); err != nil {
			return fmt.Errorf("create artifact: %w", err)
		}
	}
	var pending *pendingArtifact
	var excluded []string
	if named || upload {
		var err error
		pending, err = reserveArtifact(output, tmpDir)
		if err != nil {
			return fmt.Errorf("create artifact: %w", err)
		}
		defer func() { retErr = errors.Join(retErr, pending.Close()) }()
		// Exclude the entire private staging directory, including its file.
		excluded = append(excluded, pending.path)
	}
	if named {
		excluded = append(excluded, output)
	}
	p, err := prepare(excluded)
	if err != nil {
		return err
	}
	defer func() {
		if p != nil {
			retErr = errors.Join(retErr, p.Close())
		}
	}()
	w := io.Writer(os.Stdout)
	var owned io.WriteCloser
	if pending != nil {
		owned, err = ops.createArtifact(pending.file)
		if err != nil {
			return fmt.Errorf("create artifact: %w", err)
		}
		w = owned
	}
	_, _, writeErr := p.WriteArtifact(ctx, w, "image")
	if writeErr != nil {
		writeErr = fmt.Errorf("pack image artifact: %w", writeErr)
	}
	var closeErr error
	if owned != nil {
		if err := owned.Close(); err != nil {
			closeErr = fmt.Errorf("close artifact: %w", err)
		}
	}
	err = errors.Join(writeErr, closeErr, p.Close())
	p = nil
	if err != nil {
		return err
	}
	// All source descriptors and owned scratch are released before upload or
	// a second read of P. A failure never exposes a named partial artifact.
	if pending != nil {
		if err := pending.verify(); err != nil {
			return err
		}
	}
	if output == "-" && pending != nil {
		info, err := pending.file.Stat()
		if err != nil {
			return err
		}
		_, err = io.Copy(os.Stdout, exportContextReader{ctx, io.NewSectionReader(pending.file, 0, info.Size())})
		if err != nil {
			return fmt.Errorf("write to stdout: %w", err)
		}
	}
	var key string
	if upload {
		key, err = ops.ingestArtifact(ctx, pending.file, mcfg, noProgress)
		if err != nil {
			return err
		}
	}
	if err := ctx.Err(); err != nil {
		return err
	}
	if pending != nil {
		if err := pending.closeFile(); err != nil {
			return fmt.Errorf("close artifact: %w", err)
		}
		if err := pending.verify(); err != nil {
			return err
		}
	}
	if named {
		if err := pending.commit(); err != nil {
			return fmt.Errorf("commit artifact: %w", err)
		}
	}
	if upload {
		if _, err := fmt.Println(key); err != nil {
			return fmt.Errorf("write manifest key: %w", err)
		}
	}
	return nil
}

type exportContextReader struct {
	ctx context.Context
	r   io.Reader
}

func (r exportContextReader) Read(b []byte) (int, error) {
	if err := r.ctx.Err(); err != nil {
		return 0, err
	}
	return r.r.Read(b)
}

// outputPermissions accepts only ordinary destination files. Atomic replacement
// must not unlink a device, directory or symlink. Check again at commit time.
func outputPermissions(output string) (os.FileMode, *syscall.Stat_t, error) {
	st, err := os.Lstat(output)
	if errors.Is(err, os.ErrNotExist) {
		return 0666, nil, nil
	}
	if err != nil {
		return 0, nil, err
	}
	if !st.Mode().IsRegular() {
		return 0, nil, fmt.Errorf("destination is not a regular file: %s", output)
	}
	return st.Mode().Perm(), st.Sys().(*syscall.Stat_t), nil
}
