package routes

import (
	"github.com/dcode-github/property_lisitng_system/backend/controllers"
	"github.com/dcode-github/property_lisitng_system/backend/middleware"
	"github.com/gorilla/mux"
	"github.com/redis/go-redis/v9"
	"go.mongodb.org/mongo-driver/mongo"
)

func Routes(router *mux.Router, client *mongo.Client, redisClient *redis.Client) {
	// Auth routes
	router.HandleFunc("/register", controllers.RegisterUser()).Methods("POST")
	router.HandleFunc("/login", controllers.LoginUser()).Methods("POST")

	// Routes that require authentication
	authenticated := router.PathPrefix("/api").Subrouter()
	authenticated.Use(middleware.AuthMiddleware)

	// Property routes
	authenticated.HandleFunc("/properties", controllers.CreateProperty(redisClient)).Methods("POST")
	authenticated.HandleFunc("/properties", controllers.GetAllProperties(redisClient)).Methods("GET")
	// authenticated.HandleFunc("/properties/{id}", controllers.GetPropertyByID()).Methods("GET")
	authenticated.HandleFunc("/properties/{id}", controllers.UpdateProperty(redisClient)).Methods("PUT")
	authenticated.HandleFunc("/properties/{id}", controllers.DeleteProperty(redisClient)).Methods("DELETE")

	// Favorites routes
	authenticated.HandleFunc("/favorites", controllers.AddFavorite(redisClient)).Methods("POST")
	authenticated.HandleFunc("/favorites", controllers.GetFavorites(redisClient)).Methods("GET")
	authenticated.HandleFunc("/favorites/{id}", controllers.DeleteFavorite(redisClient)).Methods("DELETE")

	// Recommendations routes
	authenticated.HandleFunc("/recommend", controllers.RecommendProperty(redisClient)).Methods("POST")
	authenticated.HandleFunc("/recommendations", controllers.GetRecommendations(redisClient)).Methods("GET")
}
