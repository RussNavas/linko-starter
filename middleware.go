package main

import (
	"context"
	"crypto/rand"
	"fmt"
	"io"
	"log/slog"
	"net"
	"net/http"
	"time"
)


type spyReadCloser struct{
	io.ReadCloser
	bytesRead int
}


func (r *spyReadCloser) Read(p []byte) (int, error){
	n, err := r.ReadCloser.Read(p)
	r.bytesRead += n
	return n, err
}


type spyResponseWriter struct{
	http.ResponseWriter
	bytesWritten 		int
	statusCode 			int
}

func(w *spyResponseWriter) Write(p []byte) (int, error){
	if w.statusCode == 0 {
		w.statusCode = http.StatusOK
	}
	n, err := w.ResponseWriter.Write(p)
	w.bytesWritten += n
	return n, err
}

func (w *spyResponseWriter) WriteHeader(statusCode int){
	w.statusCode = statusCode
	w.ResponseWriter.WriteHeader(statusCode)
}

func httpError(ctx context.Context, w http.ResponseWriter, status int, err error) {
	if logCtx, ok := ctx.Value(LogContextKey).(*LogContext); ok {
		logCtx.Error = err
	}
	var msg string
	switch status{
		case 401:
			msg = http.StatusText(401)
		case 403:
			msg = http.StatusText(403)
		case 500:
			msg = http.StatusText(500)
		default:
			msg = err.Error()
	}
	http.Error(w, msg, status)
}


func requestLogger(logger *slog.Logger) func(http.Handler) http.Handler {

	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			start := time.Now()

			spyReader := &spyReadCloser{ReadCloser: r.Body}
			r.Body = spyReader
			spyWriter := &spyResponseWriter{ResponseWriter: w}
			logContext := &LogContext{}
			r = r.WithContext(context.WithValue(r.Context(), LogContextKey, logContext))
			next.ServeHTTP(spyWriter, r)

			attrs := []any{
				slog.String("method", r.Method),
				slog.String("path", r.URL.Path),
				slog.String("client_ip", redactIP(r.RemoteAddr)),
				slog.Duration("duration", time.Since(start)),
				slog.Int("request_body_bytes", spyReader.bytesRead),
				slog.Int("response_status", spyWriter.statusCode),
				slog.Int("response_body_bytes", spyWriter.bytesWritten),
				slog.String("request_id", r.Header.Get("X-Request-ID")),
			}

			if logContext.Username != ""{
				attrs = append(attrs, slog.String("user", logContext.Username))
			}

			if logContext.Error != nil{
				attrs = append(attrs,"error", logContext.Error)
			}


			logger.Info(
				"Served request",
				attrs...
			)
		})
	}
}


func requestID() func(http.Handler) http.Handler{
	return func(next http.Handler) http.Handler{
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request){
			reqID := r.Header.Get("X-Request-ID")
			if reqID == ""{
				reqID = rand.Text()
			}
			w.Header().Set("X-Request-ID",reqID)
			next.ServeHTTP(w, r)
		})
	}
}

func redactIP(ipStr string) string{
	host, _, err := net.SplitHostPort(ipStr)
	if err != nil{
		return fmt.Sprintf("Problem with split host port: %v", err)
	}
	ip := net.ParseIP(host)
	if ip == nil {
		return ipStr
	}
	
	v4 := ip.To4()
	if v4 == nil {
		return ipStr
	}

	return fmt.Sprintf("%d.%d.%d.x", v4[0], v4[1], v4[2])
}
