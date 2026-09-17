package flags

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"path"
	"path/filepath"
	"strings"

	commonshttp "github.com/flanksource/commons/http"
)

// FileReferenceOptions controls binary @file/@url access.
type FileReferenceOptions struct {
	// BaseDir anchors relative paths; empty uses the working directory.
	BaseDir string
	// Root confines local files to this real directory. Remote callers must set it.
	Root string
	// Remote uses defensive HTTP and forbids unrooted local files.
	Remote bool
}

// FileReference retains the source name so callers can identify binary formats.
type FileReference struct {
	Name string
	Body io.ReadCloser
}

// OpenFileReference opens a binary @file or @http(s) reference without
// converting its bytes to a string. The caller owns the returned Body.
func OpenFileReference(ctx context.Context, value string, options FileReferenceOptions) (FileReference, error) {
	if !strings.HasPrefix(value, "@") || len(value) == 1 {
		return FileReference{}, fmt.Errorf("file reference must start with @ followed by a path or URL")
	}
	ref := strings.TrimPrefix(value, "@")
	if strings.Contains(ref, "://") && !isURL(ref) {
		return FileReference{}, fmt.Errorf("unsupported file reference URL scheme")
	}
	if isURL(ref) {
		return openURLFileReference(ctx, ref, options.Remote)
	}
	return openLocalFileReference(ref, options)
}

func openURLFileReference(ctx context.Context, ref string, remote bool) (FileReference, error) {
	parsed, err := url.Parse(ref)
	if err != nil || parsed.User != nil || parsed.Host == "" {
		return FileReference{}, fmt.Errorf("invalid file reference URL")
	}
	var response *http.Response
	if remote {
		result, fetchErr := commonshttp.NewDefensive().R(ctx).Get(ref)
		if fetchErr != nil {
			return FileReference{}, fileReferenceRequestError(fetchErr, ref, parsed)
		}
		response = result.Response
	} else {
		request, err := http.NewRequestWithContext(ctx, http.MethodGet, ref, nil)
		if err != nil {
			return FileReference{}, fileReferenceRequestError(err, ref, parsed)
		}
		response, err = http.DefaultClient.Do(request)
		if err != nil {
			return FileReference{}, fileReferenceRequestError(err, ref, parsed)
		}
	}
	if response.StatusCode != http.StatusOK {
		_ = response.Body.Close()
		return FileReference{}, fmt.Errorf("file reference URL returned HTTP %d", response.StatusCode)
	}
	return FileReference{Name: path.Base(parsed.Path), Body: response.Body}, nil
}

func openLocalFileReference(ref string, options FileReferenceOptions) (FileReference, error) {
	if options.Remote && options.Root == "" {
		return FileReference{}, fmt.Errorf("remote local file reference requires a root")
	}
	base := options.BaseDir
	if base == "" {
		var err error
		base, err = os.Getwd()
		if err != nil {
			return FileReference{}, fmt.Errorf("resolving file reference base: %w", err)
		}
	}
	local := ref
	if !filepath.IsAbs(local) {
		local = filepath.Join(base, local)
	}
	local, err := filepath.Abs(local)
	if err != nil {
		return FileReference{}, fmt.Errorf("resolving file reference: %w", err)
	}
	canonical, err := filepath.EvalSymlinks(local)
	if err != nil {
		return FileReference{}, fmt.Errorf("resolving file reference: %w", err)
	}
	if err := checkPath(canonical); err != nil {
		return FileReference{}, err
	}
	var root string
	if options.Root != "" {
		absoluteRoot, err := filepath.Abs(options.Root)
		if err != nil {
			return FileReference{}, fmt.Errorf("resolving file reference root: %w", err)
		}
		root, err = filepath.EvalSymlinks(absoluteRoot)
		if err != nil {
			return FileReference{}, fmt.Errorf("resolving file reference root: %w", err)
		}
		if !fileReferenceWithinRoot(root, canonical) {
			return FileReference{}, fmt.Errorf("file reference escapes root")
		}
	}
	file, err := openValidatedFile(canonical)
	if err != nil {
		return FileReference{}, err
	}
	if root != "" {
		openedPath, pathErr := filepath.EvalSymlinks(canonical)
		if pathErr != nil || !fileReferenceWithinRoot(root, openedPath) {
			_ = file.Close()
			return FileReference{}, fmt.Errorf("file reference escapes root after opening")
		}
	}
	return FileReference{Name: filepath.Base(canonical), Body: file}, nil
}

func fileReferenceWithinRoot(root, target string) bool {
	relative, err := filepath.Rel(root, target)
	return err == nil && relative != ".." && !strings.HasPrefix(relative, ".."+string(filepath.Separator)) && !filepath.IsAbs(relative)
}

func fileReferenceRequestError(err error, ref string, parsed *url.URL) error {
	message := strings.ReplaceAll(err.Error(), ref, parsed.Scheme+"://"+parsed.Host+parsed.Path)
	if parsed.RawQuery != "" {
		message = strings.ReplaceAll(message, parsed.RawQuery, "[redacted]")
	}
	return fmt.Errorf("fetching file reference URL: %s", message)
}
