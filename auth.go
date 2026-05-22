package main

import (
	"errors"
	"context"
	"net/http"
	"log/slog"
	"golang.org/x/crypto/bcrypt"
	pkgerr "github.com/pkg/errors"
)

type contextKey string

const UserContextKey contextKey = "user"

var allowedUsers = map[string]string{
	"frodo":   "$2a$10$B6O/n6teuCzpuh66jrUAdeaJ3WvXcxRkzpN0x7H.di9G9e/NGb9Me",
	"samwise": "$2a$10$EWZpvYhUJtJcEMmm/IBOsOGIcpxUnGIVMRiDlN/nxl1RRwWGkJtty",
}

func (s *server) authMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		username, password, ok := r.BasicAuth()
		if !ok {
			httpError(r.Context(), w, http.StatusUnauthorized, errors.New("unauthorized"))
			return
		}
		stored, exists := allowedUsers[username]
		if !exists {
			httpError(r.Context(), w, http.StatusUnauthorized, errors.New("unauthorized"))
			return
		}
		ok, err := s.validatePassword(r.Context(), password, stored)
		if err != nil {
			s.logger.Info("error validating password", 
				slog.String("user", username),
				slog.Any("error", err),
			)
			httpError(r.Context(), w, http.StatusInternalServerError, err)
			return
		}
		if !ok {
			httpError(r.Context(), w, http.StatusUnauthorized, errors.New("unauthorized"))
			return
		}

		r = r.WithContext(context.WithValue(r.Context(), UserContextKey, username))
		val := r.Context().Value(LogContextKey)
		if logCtx, ok := val.(*LogContext); ok{
			logCtx.Username = username
		}
		next.ServeHTTP(w, r)
	})
}

func (s *server) validatePassword(ctx context.Context, password, stored string) (bool, error) {
	ctx, span := tracer.Start(ctx, "auth.validate_password")
	defer span.End()
	err := bcrypt.CompareHashAndPassword([]byte(stored), []byte(password))
	if err == bcrypt.ErrMismatchedHashAndPassword {
		return false, nil
	}
	if err != nil {

		return false, pkgerr.WithStack(err)
	}
	return true, nil
}
