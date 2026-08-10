package rpc

import (
	"compress/gzip"
	"io"
	"net/http"
	"strings"
	"sync"
)

// The rendered table format is deliberately redundant — every cell carries its
// own kind, and a column's styling repeats down the column — so the payload
// compresses by more than an order of magnitude. Serving it uncompressed makes
// the client wait on bytes that carry almost no information.
var gzipWriters = sync.Pool{
	New: func() any {
		writer, err := gzip.NewWriterLevel(io.Discard, gzip.BestSpeed)
		if err != nil {
			// The only error NewWriterLevel returns is an invalid level, and the
			// level here is a constant.
			panic(err)
		}
		return writer
	},
}

// compressible reports whether a body of this media type is worth gzipping.
//
// An empty type is refused rather than assumed: Go sniffs an unset Content-Type
// from the first bytes written, and those bytes would be the gzip header — so
// compressing here would relabel the response as gzip and lose its real type.
func compressible(contentType string) bool {
	media := strings.TrimSpace(strings.ToLower(strings.Split(contentType, ";")[0]))
	switch {
	case media == "":
		return false
	// Already compressed; re-compressing only adds a header and CPU.
	case media == "application/pdf",
		media == "application/zip",
		media == "application/gzip",
		media == "application/vnd.openxmlformats-officedocument.spreadsheetml.sheet":
		return false
	// An event stream is read a frame at a time by intermediaries that do not
	// all handle a compressed one, and its frames are small enough that the
	// saving would not pay for the risk.
	case media == "text/event-stream":
		return false
	case strings.HasPrefix(media, "text/"),
		strings.HasPrefix(media, "image/svg"):
		return true
	case strings.HasPrefix(media, "application/"):
		// application/json, application/json+clicky, application/x-ndjson,
		// application/yaml, application/javascript, ...
		return !strings.HasPrefix(media, "application/octet-stream")
	default:
		return false
	}
}

// compressingWriter gzips a response body when the caller asked for it.
//
// The decision is deferred to WriteHeader because that is the first moment the
// Content-Type is known, and it is the type — not the handler — that decides
// whether compressing is worth it.
type compressingWriter struct {
	http.ResponseWriter
	gzip        *gzip.Writer
	wroteHeader bool
}

func (c *compressingWriter) WriteHeader(status int) {
	if c.wroteHeader {
		return
	}
	c.wroteHeader = true
	header := c.Header()

	// 204 and 304 carry no body, and a handler that encoded its own body owns
	// that decision.
	noBody := status == http.StatusNoContent || status == http.StatusNotModified
	if noBody || header.Get("Content-Encoding") != "" || !compressible(header.Get("Content-Type")) {
		c.ResponseWriter.WriteHeader(status)
		return
	}

	header.Set("Content-Encoding", "gzip")
	// The compressed length is not known until the body has been written, and
	// the uncompressed one would be a lie the client reads as truth.
	header.Del("Content-Length")

	writer := gzipWriters.Get().(*gzip.Writer)
	writer.Reset(c.ResponseWriter)
	c.gzip = writer
	c.ResponseWriter.WriteHeader(status)
}

func (c *compressingWriter) Write(body []byte) (int, error) {
	if !c.wroteHeader {
		c.WriteHeader(http.StatusOK)
	}
	if c.gzip != nil {
		return c.gzip.Write(body)
	}
	return c.ResponseWriter.Write(body)
}

// Flush keeps a stream a stream. An export writes rows as it reads them, and a
// compressor that only drained at the end would turn that back into a wait.
func (c *compressingWriter) Flush() {
	if c.gzip != nil {
		_ = c.gzip.Flush()
	}
	if flusher, ok := c.ResponseWriter.(http.Flusher); ok {
		flusher.Flush()
	}
}

// Unwrap lets http.ResponseController reach the writer underneath.
func (c *compressingWriter) Unwrap() http.ResponseWriter { return c.ResponseWriter }

// close returns the pooled writer and reports whether the gzip member was
// finished. The error is returned rather than dropped because gzip.Close is
// what writes the final deflate block, the CRC and the length: without it the
// client holds a truncated member under a 200 it has every reason to trust.
func (c *compressingWriter) close() error {
	if c.gzip == nil {
		return nil
	}
	err := c.gzip.Close()
	// Reset clears the writer's sticky error along with the rest of its state,
	// so a failed member does not poison the next response out of the pool.
	c.gzip.Reset(io.Discard)
	gzipWriters.Put(c.gzip)
	c.gzip = nil
	return err
}

// Compress wraps next so responses are gzipped for callers that accept it.
//
// The wrapper is closed after next returns and before the server writes any
// declared trailer, so a trailing X-Truncated still lands after a complete
// compressed body rather than inside one.
func Compress(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Vary regardless of what this particular caller asked for: the same URL
		// answers with and without compression, and a cache that does not know
		// that will serve one to a client expecting the other. Only when it is
		// not already declared, though — a second copy of the token is not a
		// stronger instruction, it is a header caches and proxies have to
		// normalise, and it makes the response differ from itself run to run.
		if !variesOnAcceptEncoding(w.Header()) {
			w.Header().Add("Vary", "Accept-Encoding")
		}
		if !acceptsGzip(r.Header.Get("Accept-Encoding")) {
			next.ServeHTTP(w, r)
			return
		}
		writer := &compressingWriter{ResponseWriter: w}
		defer func() {
			// A panic already unwinding carries the better diagnosis, so it has
			// to reach net/http's logger intact. Raising ErrAbortHandler over
			// the top of it would substitute the one value net/http logs no
			// stack for, and a real bug would leave nothing behind but a
			// dropped connection. Re-panicking with the original value keeps
			// the stack and still aborts, because net/http drops the connection
			// on any unrecovered handler panic. Closing is best effort here,
			// and cannot happen twice: re-panicking leaves the closure at once.
			if panicked := recover(); panicked != nil {
				_ = writer.close()
				panic(panicked)
			}
			if writer.close() == nil {
				return
			}
			// The status line and headers went out at WriteHeader, so the
			// failure cannot be reported as a 5xx and the body already on the
			// wire cannot be recalled. Breaking the connection is the only
			// remaining way to say so: an HTTP/1.1 response ended without its
			// terminating chunk is a transfer error that curl -f and browsers
			// both surface, where a truncated gzip member under a 200 is read
			// as a complete answer. Recording the error on the writer instead
			// would be invisible: the handler has already returned and nothing
			// is left to read it.
			panic(http.ErrAbortHandler)
		}()
		next.ServeHTTP(writer, r)
	})
}

// variesOnAcceptEncoding reports whether the response already declares that it
// varies on Accept-Encoding. Field names are case-insensitive and a handler may
// have packed several into one comma-separated value, so neither a string
// compare nor a lookup of the whole value would find it.
func variesOnAcceptEncoding(header http.Header) bool {
	for _, value := range header.Values("Vary") {
		for _, field := range strings.Split(value, ",") {
			if strings.EqualFold(strings.TrimSpace(field), "Accept-Encoding") {
				return true
			}
		}
	}
	return false
}

// acceptsGzip reports whether the header offers gzip without refusing it — a
// "gzip;q=0" is an explicit no rather than a yes with a weight.
func acceptsGzip(accept string) bool {
	for _, part := range strings.Split(accept, ",") {
		fields := strings.Split(part, ";")
		if strings.TrimSpace(strings.ToLower(fields[0])) != "gzip" {
			continue
		}
		for _, parameter := range fields[1:] {
			parameter = strings.ReplaceAll(strings.TrimSpace(strings.ToLower(parameter)), " ", "")
			if parameter == "q=0" || strings.HasPrefix(parameter, "q=0.0") {
				return false
			}
		}
		return true
	}
	return false
}
