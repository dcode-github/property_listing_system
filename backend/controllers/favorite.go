package controllers

import (
	"context"
	"encoding/json"
	"log"
	"net/http"

	"github.com/dcode-github/property_lisitng_system/backend/config"
	"github.com/dcode-github/property_lisitng_system/backend/models"
	"github.com/gorilla/mux"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
)

func AddFavorite(client *mongo.Client) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		userID, ok := r.Context().Value(UserIDKey).(string)
		if !ok {
			log.Println("User ID missing in context")
			http.Error(w, "User ID missing in context", http.StatusUnauthorized)
			return
		}

		var fav models.Favorite
		if err := json.NewDecoder(r.Body).Decode(&fav); err != nil {
			log.Println("Invalid request data ", err)
			http.Error(w, "Invalid request data", http.StatusBadRequest)
			return
		}

		if fav.PropertyID.IsZero() {
			log.Println("PropertyID is required")
			http.Error(w, "PropertyID is required", http.StatusBadRequest)
			return
		}

		fav.UserID = userID
		fav.ID = primitive.NewObjectID()

		var existingFav models.Favorite
		err := config.FavoriteCollection.FindOne(context.TODO(), bson.M{"userID": userID, "propertyID": fav.PropertyID}).Decode(&existingFav)
		if err == nil {
			log.Println("Property is already in favorites")
			http.Error(w, "Property is already in favorites", http.StatusConflict)
			return
		}
		if err != mongo.ErrNoDocuments {
			log.Println("Failed to check favorites ", err)
			http.Error(w, "Failed to check favorites", http.StatusInternalServerError)
			return
		}

		_, err = config.FavoriteCollection.InsertOne(context.TODO(), fav)
		if err != nil {
			log.Println("Failed to add property to favorites ", err)
			http.Error(w, "Failed to add property to favorites", http.StatusInternalServerError)
			return
		}

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusCreated)
		json.NewEncoder(w).Encode(models.APIResponse{
			Success: true,
			Message: "Property added to favorites",
			Data:    fav,
		})
	}
}

func GetFavorites(client *mongo.Client) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		userID, ok := r.Context().Value(UserIDKey).(string)
		if !ok {
			log.Println("User ID missing in context")
			http.Error(w, "User ID missing in context", http.StatusUnauthorized)
			return
		}

		pipeline := mongo.Pipeline{
			{
				{Key: "$match", Value: bson.M{"userID": userID}},
			},
			{
				{Key: "$lookup", Value: bson.M{
					"from":         "demo_prop",
					"localField":   "propertyID",
					"foreignField": "_id",
					"as":           "propertyDetails",
				}},
			},
			{
				{Key: "$unwind", Value: "$propertyDetails"},
			},
			{
				{Key: "$replaceRoot", Value: bson.M{"newRoot": "$propertyDetails"}},
			},
		}

		cursor, err := config.FavoriteCollection.Aggregate(context.TODO(), pipeline)
		if err != nil {
			log.Println("Failed to fetch favorite properties ", err)
			http.Error(w, "Failed to fetch favorite properties", http.StatusInternalServerError)
			return
		}
		defer cursor.Close(context.TODO())

		var properties []models.Property
		if err := cursor.All(context.TODO(), &properties); err != nil {
			log.Println("Failed to decode favorite properties ", err)
			http.Error(w, "Failed to decode favorite properties", http.StatusInternalServerError)
			return
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(models.APIResponse{
			Success: true,
			Message: "Fetched favorite properties",
			Data:    properties,
		})
	}
}

func DeleteFavorite(client *mongo.Client) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		userID, ok := r.Context().Value(UserIDKey).(string)
		if !ok {
			log.Println("User ID missing in context")
			http.Error(w, "User ID missing in context", http.StatusUnauthorized)
			return
		}

		params := mux.Vars(r)
		propertyIDHex := params["propertyID"]

		propertyObjID, err := primitive.ObjectIDFromHex(propertyIDHex)
		if err != nil {
			log.Println("Invalid property ID format ", err)
			http.Error(w, "Invalid property ID format", http.StatusBadRequest)
			return
		}

		deleteResult, err := config.FavoriteCollection.DeleteOne(context.TODO(), bson.M{
			"userID":     userID,
			"propertyID": propertyObjID,
		})
		if err != nil {
			log.Println("Failed to remove property from favorites ", err)
			http.Error(w, "Failed to remove property from favorites", http.StatusInternalServerError)
			return
		}

		if deleteResult.DeletedCount == 0 {
			log.Println("Favorite not found")
			http.Error(w, "Favorite not found", http.StatusNotFound)
			return
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(models.APIResponse{
			Success: true,
			Message: "Property removed from favorites",
			Data:    nil,
		})
	}
}
