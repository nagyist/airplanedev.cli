package middleware

import (
	"compress/gzip"
	"io"
	"net/http"

	"github.com/airplanedev/cli/pkg/cli/analytics"
	"github.com/airplanedev/cli/pkg/cli/server/handlers"
	"github.com/pkg/errors"
	"go.uber.org/multierr"
)

type wrappedGzipReader struct {
	rawBody    io.ReadCloser
	gzipReader *gzip.Reader
}

func newWrappedGzipReader(rawBody io.ReadCloser) (*wrappedGzipReader, error) {
	gzipReader, err := gzip.NewReader(rawBody)
	if err != nil {
		return nil, errors.Wrap(err, "creating gzip reader")
	}

	return &wrappedGzipReader{
		rawBody:    rawBody,
		gzipReader: gzipReader,
	}, nil
}

func (w *wrappedGzipReader) Read(p []byte) (n int, err error) {
	return w.gzipReader.Read(p)
}

func (w *wrappedGzipReader) Close() error {
	// Close both and then combine the errors as needed
	gzipErr := w.gzipReader.Close()
	rawErr := w.rawBody.Close()

	return multierr.Combine(gzipErr, rawErr)
}

// ReqBodyDecompression is a middleware layer that decompresses gzipped request
// bodies if needed.
func ReqBodyDecompression(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Content-Encoding") == "gzip" {
			gzipReader, err := newWrappedGzipReader(r.Body)
			if err != nil {
				handlers.WriteHTTPError(w, r, err, analytics.ReportError)
				return
			}
			r.Body = gzipReader
		}

		next.ServeHTTP(w, r)
	})
}
