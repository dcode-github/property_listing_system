package controllers

import (
	"context"
	"encoding/json"
	"log"
	"net/http"

	"github.com/dcode-github/property_lisitng_system/backend/config"
	"github.com/dcode-github/property_lisitng_system/backend/models"
	"github.com/redis/go-redis/v9"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
)

const (
	userRecommendationsCachePrefix = "recommendations:user:"
)

func generateUserRecommendationsCacheKey(userID string) string {
	return userRecommendationsCachePrefix + userID
}

func deleteUserRecommendationsCache(ctx context.Context, redisClient *redis.Client, userID string) {
	cacheKey := generateUserRecommendationsCacheKey(userID)
	err := redisClient.Del(ctx, cacheKey).Err()
	if err != nil && err != redis.Nil {
		log.Printf("Error deleting user recommendations cache key %s: %v", cacheKey, err)
	} else if err == nil {
		log.Printf("Successfully deleted user recommendations cache key: %s", cacheKey)
	}
}

func RecommendProperty(redisClient *redis.Client) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		requestCtx := r.Context()

		fromUserID, ok := requestCtx.Value(UserIDKey).(string)
		if !ok {
			log.Println("User ID (fromUserID) missing in context for RecommendProperty")
			http.Error(w, "User ID missing in context", http.StatusUnauthorized)
			return
		}

		var recInput models.Recommendation
		if err := json.NewDecoder(r.Body).Decode(&recInput); err != nil {
			log.Printf("Invalid input for RecommendProperty: %v", err)
			http.Error(w, "Invalid input", http.StatusBadRequest)
			return
		}

		if recInput.ToEmailID == "" {
			log.Println("ToEmailID is required for RecommendProperty")
			http.Error(w, "ToEmailID is required", http.StatusBadRequest)
			return
		}
		if recInput.PropertyID.IsZero() {
			log.Println("PropertyId is required for RecommendProperty")
			http.Error(w, "PropertyId is required", http.StatusBadRequest)
			return
		}

		var toUser models.User
		err := config.UserCollection.FindOne(requestCtx, bson.M{"email": recInput.ToEmailID}).Decode(&toUser)
		if err != nil {
			if err == mongo.ErrNoDocuments {
				log.Printf("No such user with email %s for recommendation", recInput.ToEmailID)
				http.Error(w, "User to recommend to not found", http.StatusBadRequest) // More specific error
			} else {
				log.Printf("Error checking database for user %s: %v", recInput.ToEmailID, err)
				http.Error(w, "Error checking database", http.StatusInternalServerError)
			}
			return
		}

		recommendationToSave := models.Recommendation{
			FromUserID: fromUserID,
			ToUserID:   toUser.UserID,
			ToEmailID:  recInput.ToEmailID,
			PropertyID: recInput.PropertyID,
		}

		_, err = config.RecommendationCollection.InsertOne(requestCtx, recommendationToSave)
		if err != nil {
			log.Printf("Insert failed for recommendation from %s to %s (email %s): %v", fromUserID, toUser.UserID, recInput.ToEmailID, err)
			http.Error(w, "Failed to send recommendation", http.StatusInternalServerError)
			return
		}

		go func() {
			deleteUserRecommendationsCache(context.Background(), redisClient, toUser.UserID)
			log.Printf("Recommendation cache invalidated for recipient user %s", toUser.UserID)
		}()

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]string{"message": "Recommendation sent successfully"})
	}
}

func GetRecommendations(redisClient *redis.Client) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		requestCtx := r.Context()

		toUserID, ok := requestCtx.Value(UserIDKey).(string)
		if !ok {
			log.Println("User ID (toUserID) missing in context for GetRecommendations")
			http.Error(w, "User ID missing in context", http.StatusUnauthorized)
			return
		}

		cacheKey := generateUserRecommendationsCacheKey(toUserID)
		cachedData, err := redisClient.Get(requestCtx, cacheKey).Result()

		if err == nil {
			log.Printf("Cache Hit for GetRecommendations, user %s, key %s", toUserID, cacheKey)
			w.Header().Set("Content-Type", "application/json")
			w.Write([]byte(cachedData))
			return
		}
		if err != redis.Nil {
			log.Printf("Redis GET error for GetRecommendations user %s, key %s: %v. Fetching from DB.", toUserID, cacheKey, err)
		}

		log.Printf("Cache Miss for GetRecommendations, user %s, key %s", toUserID, cacheKey)

		pipeline := mongo.Pipeline{
			{{Key: "$match", Value: bson.M{"toUserID": toUserID}}},
			{{Key: "$lookup", Value: bson.M{
				"from":         config.PropertyCollection.Name(),
				"localField":   "propertyId",
				"foreignField": "_id",
				"as":           "propertyDetails",
			}}},
			{{Key: "$unwind", Value: "$propertyDetails"}},
			{{Key: "$replaceWith", Value: bson.M{
				"$mergeObjects": bson.A{
					"$propertyDetails",
					bson.M{
						"recommendedBy": "$fromUserId",
					},
				},
			}}},
		}

		cursor, err := config.RecommendationCollection.Aggregate(requestCtx, pipeline)
		if err != nil {
			log.Printf("Error aggregating recommendations for user %s: %v", toUserID, err)
			http.Error(w, "Failed to retrieve recommendations", http.StatusInternalServerError)
			return
		}
		defer cursor.Close(requestCtx)

		var recommendedPropertiesWithMeta []map[string]interface{}

		if err := cursor.All(requestCtx, &recommendedPropertiesWithMeta); err != nil {
			log.Printf("Error decoding aggregated recommendations for user %s: %v", toUserID, err)
			http.Error(w, "Failed to decode recommendations", http.StatusInternalServerError)
			return
		}

		response := models.APIResponse{
			Success: true,
			Message: "Fetched recommended properties",
			Data:    recommendedPropertiesWithMeta,
		}

		responseBytes, err := json.Marshal(response)
		if err != nil {
			log.Printf("Failed to marshal GetRecommendations response for user %s: %v", toUserID, err)
			http.Error(w, "Error processing response data", http.StatusInternalServerError)
			return
		}

		err = redisClient.Set(requestCtx, cacheKey, responseBytes, defaultCacheTTL).Err()
		if err != nil {
			log.Printf("Failed to cache GetRecommendations response for user %s, key %s: %v", toUserID, cacheKey, err)
		}

		w.Header().Set("Content-Type", "application/json")
		w.Write(responseBytes)
	}
}
