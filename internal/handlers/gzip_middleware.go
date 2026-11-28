package handlers

import (
	"compress/gzip"
	"io"
	"net/http"
	"strings"
	"sync"
)

var gzipWriterPool = sync.Pool{
	New: func() interface{} {
		return gzip.NewWriter(io.Discard)
	},
}

type gzipResponseWriter struct {
	http.ResponseWriter
	writer      *gzip.Writer
	statusCode  int
	wroteHeader bool
	wroteBody   bool
	skipGzip    bool
}

func (w *gzipResponseWriter) WriteHeader(statusCode int) {
	if w.wroteHeader {
		return
	}
	w.wroteHeader = true
	w.statusCode = statusCode
	if statusCode == http.StatusNoContent || statusCode == http.StatusNotModified || (statusCode >= 100 && statusCode < 200) {
		w.skipGzip = true
		w.Header().Del("Content-Encoding")
		w.ResponseWriter.WriteHeader(statusCode)
		return
	}
	w.Header().Del("Content-Length")
	w.Header().Set("Content-Encoding", "gzip")
	w.Header().Add("Vary", "Accept-Encoding")
	w.ResponseWriter.WriteHeader(statusCode)
}

func (w *gzipResponseWriter) Write(b []byte) (int, error) {
	if !w.wroteHeader {
		if w.Header().Get("Content-Type") == "" {
			w.Header().Set("Content-Type", http.DetectContentType(b))
		}
		w.WriteHeader(http.StatusOK)
	}
	if w.skipGzip {
		return w.ResponseWriter.Write(b)
	}
	if len(b) == 0 {
		return 0, nil
	}
	w.wroteBody = true
	return w.writer.Write(b)
}

func (w *gzipResponseWriter) Flush() {
	if !w.skipGzip && w.wroteBody {
		_ = w.writer.Flush()
	}
	if fl, ok := w.ResponseWriter.(http.Flusher); ok {
		fl.Flush()
	}
}

func (w *gzipResponseWriter) Unwrap() http.ResponseWriter {
	return w.ResponseWriter
}

// GzipMiddleware compresses HTTP responses using gzip if requested by client.
func GzipMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Only compress if the client supports gzip, method is not HEAD, and not a WebSocket upgrade
		if r.Method == http.MethodHead ||
			!strings.Contains(r.Header.Get("Accept-Encoding"), "gzip") ||
			strings.Contains(r.Header.Get("Accept-Encoding"), "gzip;q=0") ||
			strings.EqualFold(r.Header.Get("Upgrade"), "websocket") {
			next.ServeHTTP(w, r)
			return
		}

		gz := gzipWriterPool.Get().(*gzip.Writer)
		gz.Reset(w)

		gzw := &gzipResponseWriter{ResponseWriter: w, writer: gz, statusCode: http.StatusOK}
		defer func() {
			if gzw.wroteBody && !gzw.skipGzip {
				_ = gz.Close()
			} else {
				gz.Reset(io.Discard)
			}
			gzipWriterPool.Put(gz)
		}()

		next.ServeHTTP(gzw, r)
	})
}
