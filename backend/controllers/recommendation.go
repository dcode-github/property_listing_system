package controllers

import (
	"context"
	"encoding/json"
	"log"
	"net/http"

	"github.com/dcode-github/property_lisitng_system/backend/config"
	"github.com/dcode-github/property_lisitng_system/backend/models"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
)

func RecommendProperty() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		fromUserID, ok := r.Context().Value(UserIDKey).(string)
		if !ok {
			http.Error(w, "User ID missing in context", http.StatusUnauthorized)
			return
		}

		var rec models.Recommendation
		if err := json.NewDecoder(r.Body).Decode(&rec); err != nil {
			http.Error(w, "Invalid input", http.StatusBadRequest)
			return
		}

		var user models.User

		err := config.UserCollection.FindOne(context.TODO(), bson.M{"email": rec.ToEmailID}).Decode(&user)
		if err != nil {
			if err == mongo.ErrNoDocuments {
				log.Println("No such user", rec.ToEmailID)
				http.Error(w, "No such user ", http.StatusBadRequest)
			} else {
				http.Error(w, "Error checking database", http.StatusInternalServerError)
			}
			return
		}

		rec.FromUserID = fromUserID
		rec.ToUserID = user.UserID
		_, err = config.RecommendationCollection.InsertOne(context.TODO(), rec)
		if err != nil {
			http.Error(w, "Insert failed", http.StatusInternalServerError)
			return
		}
		json.NewEncoder(w).Encode(map[string]string{"message": "Recommendation sent"})
	}
}

func GetRecommendations() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		toUserID, ok := r.Context().Value(UserIDKey).(string)
		if !ok {
			http.Error(w, "User ID missing in context", http.StatusUnauthorized)
			return
		}

		pipeline := mongo.Pipeline{
			{
				{Key: "$match", Value: bson.M{"toUserID": toUserID}},
			},
			{
				{Key: "$lookup", Value: bson.M{
					"from":         "properties",
					"localField":   "propertyId",
					"foreignField": "_id",
					"as":           "propertyDetails",
				}},
			},
			{
				{Key: "$unwind", Value: "$propertyDetails"},
			},
			{
				{Key: "$replaceWith", Value: bson.M{
					"$mergeObjects": bson.A{
						"$propertyDetails",
						bson.M{"recommendedBy": "$fromUserId"},
					},
				}},
			},
		}

		cursor, err := config.RecommendationCollection.Aggregate(context.TODO(), pipeline)
		if err != nil {
			log.Printf("Error aggregating recommendations for user %s: %v", toUserID, err)
			http.Error(w, "Failed to retrieve recommendations", http.StatusInternalServerError)
			return
		}
		defer cursor.Close(context.TODO())

		var recommendedProperties []models.Property
		if err := cursor.All(context.TODO(), &recommendedProperties); err != nil {
			log.Printf("Error decoding aggregated recommendations for user %s: %v", toUserID, err)
			http.Error(w, "Failed to decode recommendations", http.StatusInternalServerError)
			return
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(models.APIResponse{
			Success: true,
			Message: "Fetched recommended properties",
			Data:    recommendedProperties,
		})
	}
}
