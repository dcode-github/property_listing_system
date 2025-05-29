package controllers

import (
	"context"
	"encoding/json"
	"log"
	"net/http"
	"time"

	"github.com/dcode-github/property_lisitng_system/backend/config"
	"github.com/dcode-github/property_lisitng_system/backend/models"
	"github.com/gorilla/mux"
	"github.com/redis/go-redis/v9"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
)

const (
	userFavoritesCachePrefix = "favorites:user:"
	favoriteCacheTTL         = 10 * time.Minute
)

func generateUserFavoritesCacheKey(userID string) string {
	return userFavoritesCachePrefix + userID
}

func deleteUserFavoritesCache(ctx context.Context, redisClient *redis.Client, userID string) {
	cacheKey := generateUserFavoritesCacheKey(userID)
	err := redisClient.Del(ctx, cacheKey).Err()
	if err != nil && err != redis.Nil {
		log.Printf("Error deleting user favorites cache key %s: %v", cacheKey, err)
	} else if err == nil {
		log.Printf("Successfully deleted user favorites cache key: %s", cacheKey)
	}
}

func AddFavorite(redisClient *redis.Client) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		requestCtx := r.Context()

		userID, ok := requestCtx.Value(UserIDKey).(string)
		if !ok {
			log.Println("User ID missing in context for AddFavorite")
			http.Error(w, "User ID missing in context", http.StatusUnauthorized)
			return
		}

		var favInput models.Favorite
		if err := json.NewDecoder(r.Body).Decode(&favInput); err != nil {
			log.Printf("Invalid request data for AddFavorite: %v", err)
			http.Error(w, "Invalid request data", http.StatusBadRequest)
			return
		}

		if favInput.PropertyID.IsZero() {
			log.Println("PropertyID is required for AddFavorite")
			http.Error(w, "PropertyID is required", http.StatusBadRequest)
			return
		}

		favToSave := models.Favorite{
			ID:         primitive.NewObjectID(),
			UserID:     userID,
			PropertyID: favInput.PropertyID,
		}

		err := config.FavoriteCollection.FindOne(requestCtx, bson.M{"userID": userID, "propertyID": favToSave.PropertyID}).Err()
		if err == nil {
			log.Printf("Property %s is already in favorites for user %s", favToSave.PropertyID.Hex(), userID)
			http.Error(w, "Property is already in favorites", http.StatusConflict)
			return
		}
		if err != mongo.ErrNoDocuments {
			log.Printf("Failed to check favorites for user %s, property %s: %v", userID, favToSave.PropertyID.Hex(), err)
			http.Error(w, "Failed to check favorites", http.StatusInternalServerError)
			return
		}

		_, err = config.FavoriteCollection.InsertOne(requestCtx, favToSave)
		if err != nil {
			log.Printf("Failed to add property %s to favorites for user %s: %v", favToSave.PropertyID.Hex(), userID, err)
			http.Error(w, "Failed to add property to favorites", http.StatusInternalServerError)
			return
		}

		go func() {
			deleteUserFavoritesCache(context.Background(), redisClient, userID)

			deletePropertyCache(redisClient)
			log.Printf("Caches invalidated after adding favorite for user %s, property %s", userID, favToSave.PropertyID.Hex())
		}()

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusCreated)
		json.NewEncoder(w).Encode(models.APIResponse{
			Success: true,
			Message: "Property added to favorites",
			Data:    favToSave,
		})
	}
}

func GetFavorites(redisClient *redis.Client) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		requestCtx := r.Context()

		userID, ok := requestCtx.Value(UserIDKey).(string)
		if !ok {
			log.Println("User ID missing in context for GetFavorites")
			http.Error(w, "User ID missing in context", http.StatusUnauthorized)
			return
		}

		cacheKey := generateUserFavoritesCacheKey(userID)
		cachedData, err := redisClient.Get(requestCtx, cacheKey).Result()

		if err == nil {
			log.Printf("Cache Hit for GetFavorites, user %s, key %s", userID, cacheKey)
			w.Header().Set("Content-Type", "application/json")
			w.Write([]byte(cachedData))
			return
		}
		if err != redis.Nil {
			log.Printf("Redis GET error for GetFavorites user %s, key %s: %v. Fetching from DB.", userID, cacheKey, err)
		}

		log.Printf("Cache Miss for GetFavorites, user %s, key %s", userID, cacheKey)

		pipeline := mongo.Pipeline{
			{{Key: "$match", Value: bson.M{"userID": userID}}},
			{{Key: "$lookup", Value: bson.M{
				"from":         config.PropertyCollection.Name(),
				"localField":   "propertyID",
				"foreignField": "_id",
				"as":           "propertyDetails",
			}}},
			{{Key: "$unwind", Value: "$propertyDetails"}},
			{{Key: "$replaceRoot", Value: bson.M{"newRoot": "$propertyDetails"}}},
			{{Key: "$addFields", Value: bson.M{"isFavorite": true}}},
		}

		cursor, err := config.FavoriteCollection.Aggregate(requestCtx, pipeline)
		if err != nil {
			log.Printf("Failed to fetch favorite properties for user %s via aggregation: %v", userID, err)
			http.Error(w, "Failed to fetch favorite properties", http.StatusInternalServerError)
			return
		}
		defer cursor.Close(requestCtx)

		var properties []models.Property
		if err := cursor.All(requestCtx, &properties); err != nil {
			log.Printf("Failed to decode favorite properties for user %s: %v", userID, err)
			http.Error(w, "Failed to decode favorite properties", http.StatusInternalServerError)
			return
		}

		response := models.APIResponse{
			Success: true,
			Message: "Fetched favorite properties",
			Data:    properties,
		}

		responseBytes, err := json.Marshal(response)
		if err != nil {
			log.Printf("Failed to marshal GetFavorites response for user %s: %v", userID, err)
			http.Error(w, "Error processing response data", http.StatusInternalServerError)
			return
		}

		err = redisClient.Set(requestCtx, cacheKey, responseBytes, favoriteCacheTTL).Err()
		if err != nil {
			log.Printf("Failed to cache GetFavorites response for user %s, key %s: %v", userID, cacheKey, err)
		}

		w.Header().Set("Content-Type", "application/json")
		w.Write(responseBytes)
	}
}

func DeleteFavorite(redisClient *redis.Client) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		requestCtx := r.Context()

		userID, ok := requestCtx.Value(UserIDKey).(string)
		if !ok {
			log.Println("User ID missing in context for DeleteFavorite")
			http.Error(w, "User ID missing in context", http.StatusUnauthorized)
			return
		}

		params := mux.Vars(r)
		propertyIDHex := params["id"]

		propertyObjID, err := primitive.ObjectIDFromHex(propertyIDHex)
		if err != nil {
			log.Printf("Invalid property ID format '%s' for DeleteFavorite: %v", propertyIDHex, err)
			http.Error(w, "Invalid property ID format", http.StatusBadRequest)
			return
		}

		deleteResult, err := config.FavoriteCollection.DeleteOne(requestCtx, bson.M{
			"userID":     userID,
			"propertyID": propertyObjID,
		})
		if err != nil {
			log.Printf("Failed to remove property %s from favorites for user %s: %v", propertyIDHex, userID, err)
			http.Error(w, "Failed to remove property from favorites", http.StatusInternalServerError)
			return
		}

		if deleteResult.DeletedCount == 0 {
			log.Printf("Favorite not found for property %s, user %s. Nothing to delete.", propertyIDHex, userID)
			http.Error(w, "Favorite not found", http.StatusNotFound)
			return
		}

		go func() {
			deleteUserFavoritesCache(context.Background(), redisClient, userID)

			deletePropertyCache(redisClient)
			log.Printf("Caches invalidated after deleting favorite for user %s, property %s", userID, propertyIDHex)
		}()

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(models.APIResponse{
			Success: true,
			Message: "Property removed from favorites",
			Data:    nil,
		})
	}
}
