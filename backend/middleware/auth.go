package middleware

import (
	"context"
	"log"
	"net/http"
	"strings"

	"github.com/dcode-github/property_lisitng_system/backend/controllers"
	"github.com/dcode-github/property_lisitng_system/backend/utils"
)

func AuthMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {

		tokenHeader := r.Header.Get("Authorization")
		if tokenHeader == "" {
			log.Printf("Missing Authorization header from request %s %s", r.Method, r.URL)
			http.Error(w, "Missing Authorization header", http.StatusUnauthorized)
			return
		}

		tokenParts := strings.Split(tokenHeader, " ")
		if len(tokenParts) != 2 || tokenParts[0] != "Bearer" {
			log.Printf("Invalid Authorization header format from request %s %s", r.Method, r.URL)
			http.Error(w, "Invalid Authorization header format", http.StatusUnauthorized)
			return
		}

		token := tokenParts[1]

		claims, err := utils.ValidateJWT(token)
		if err != nil {
			log.Printf("Invalid or expired token: %v", err)
			http.Error(w, "Invalid or expired token", http.StatusUnauthorized)
			return
		}

		ctx := context.WithValue(r.Context(), controllers.UserIDKey, claims.UserID)

		next.ServeHTTP(w, r.WithContext(ctx))
	})
}
