package routes

import (
	"github.com/dcode-github/property_lisitng_system/backend/controllers"
	"github.com/dcode-github/property_lisitng_system/backend/middleware"
	"github.com/gorilla/mux"
	"go.mongodb.org/mongo-driver/mongo"
)

func Routes(router *mux.Router, client *mongo.Client) {
	// Auth routes
	router.HandleFunc("/register", controllers.RegisterUser(client)).Methods("POST")
	router.HandleFunc("/login", controllers.LoginUser(client)).Methods("POST")

	// Routes that require authentication
	authenticated := router.PathPrefix("/api").Subrouter()
	authenticated.Use(middleware.AuthMiddleware)

	// Property routes
	authenticated.HandleFunc("/properties", controllers.CreateProperty(client)).Methods("POST")
	authenticated.HandleFunc("/properties", controllers.GetAllProperties(client)).Methods("GET")
	// authenticated.HandleFunc("/properties/{id}", controllers.GetPropertyByID(client)).Methods("GET")
	authenticated.HandleFunc("/properties/{id}", controllers.UpdateProperty(client)).Methods("PUT")
	authenticated.HandleFunc("/properties/{id}", controllers.DeleteProperty(client)).Methods("DELETE")

	// Favorites routes
	authenticated.HandleFunc("/favorites", controllers.AddFavorite(client)).Methods("POST")
	authenticated.HandleFunc("/favorites", controllers.GetFavorites(client)).Methods("GET")
	authenticated.HandleFunc("/favorites/{propertyID}", controllers.DeleteFavorite(client)).Methods("DELETE")

	// Recommendations routes
	// authenticated.HandleFunc("/recommend", controllers.RecommendProperty(client)).Methods("POST")
	// authenticated.HandleFunc("/recommendations", controllers.GetRecommendations(client)).Methods("GET")
}
