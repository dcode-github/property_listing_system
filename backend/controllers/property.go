package controllers

import (
	"context"
	"encoding/json"
	"log"
	"net/http"
	"strings"
	"time"

	"github.com/dcode-github/property_lisitng_system/backend/config"
	"github.com/dcode-github/property_lisitng_system/backend/models"
	"github.com/gorilla/mux"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

type ContextKey string

const UserIDKey = ContextKey("userID")

func CreateProperty(_ *mongo.Client) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		userID, ok := r.Context().Value(UserIDKey).(string)
		if !ok {
			log.Println("User ID missing in context")
			http.Error(w, "User ID missing in context", http.StatusUnauthorized)
			return
		}

		var property models.Property
		if err := json.NewDecoder(r.Body).Decode(&property); err != nil {
			log.Printf("Invalid request body: %v", err)
			http.Error(w, "Invalid request body", http.StatusBadRequest)
			return
		}

		objectID := primitive.NewObjectID()
		property.ID = objectID
		property.PropId = objectID.Hex()
		property.CreatedBy = userID
		if property.AvailableFrom.IsZero() {
			property.AvailableFrom = time.Now()
		}

		_, err := config.PropertyCollection.InsertOne(context.TODO(), property)
		if err != nil {
			log.Printf("Insert failed: %v", err)
			http.Error(w, "Failed to create property", http.StatusInternalServerError)
			return
		}

		w.WriteHeader(http.StatusCreated)
		json.NewEncoder(w).Encode(property)
	}
}

func GetAllProperties(_ *mongo.Client) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		userID, ok := r.Context().Value(UserIDKey).(string)
		if !ok {
			log.Println("User ID missing in context")
			http.Error(w, "User ID missing in context", http.StatusUnauthorized)
			return
		}

		filters := bson.M{}
		for key, values := range r.URL.Query() {
			if key != "userID" && len(values) > 0 {
				filters[key] = strings.Join(values, ",")
			}
		}

		findOptions := options.Find().SetLimit(10)

		cursor, err := config.PropertyCollection.Find(context.TODO(), filters, findOptions)
		if err != nil {
			log.Printf("Error fetching properties: %v", err)
			http.Error(w, "Error fetching properties", http.StatusInternalServerError)
			return
		}
		defer cursor.Close(context.TODO())

		var properties []models.Property
		if err := cursor.All(context.TODO(), &properties); err != nil {
			log.Printf("Error decoding properties: %v", err)
			http.Error(w, "Error decoding properties", http.StatusInternalServerError)
			return
		}

		propertyIDs := make([]primitive.ObjectID, 0, len(properties))
		for _, prop := range properties {
			propertyIDs = append(propertyIDs, prop.ID)
		}

		favFilter := bson.M{
			"userID":     userID,
			"propertyID": bson.M{"$in": propertyIDs},
		}

		favCursor, err := config.FavoriteCollection.Find(context.TODO(), favFilter)
		if err != nil {
			log.Printf("Error fetching favorites: %v", err)
			http.Error(w, "Error fetching favorites", http.StatusInternalServerError)
			return
		}
		defer favCursor.Close(context.TODO())

		favMap := make(map[primitive.ObjectID]bool)
		for favCursor.Next(context.TODO()) {
			var fav models.Favorite
			if err := favCursor.Decode(&fav); err != nil {
				log.Printf("Error decoding favorite: %v", err)
				continue
			}
			favMap[fav.PropertyID] = true
		}

		for i, prop := range properties {
			if favMap[prop.ID] {
				properties[i].IsFavorite = true
			}
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(properties)
	}
}

func UpdateProperty(_ *mongo.Client) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		userID, ok := r.Context().Value(UserIDKey).(string)
		if !ok {
			log.Println("User ID missing in context")
			http.Error(w, "User ID missing in context", http.StatusUnauthorized)
			return
		}

		propertyID := mux.Vars(r)["id"]
		objID, err := primitive.ObjectIDFromHex(propertyID)
		if err != nil {
			log.Println("Invalid property ID ", err)
			http.Error(w, "Invalid property ID", http.StatusBadRequest)
			return
		}

		var updateData map[string]interface{}
		if err := json.NewDecoder(r.Body).Decode(&updateData); err != nil {
			log.Printf("Invalid update data: %v", err)
			http.Error(w, "Invalid update data", http.StatusBadRequest)
			return
		}

		delete(updateData, "_id")
		delete(updateData, "createdBy")
		delete(updateData, "id")

		if af, ok := updateData["availableFrom"].(string); ok {
			t, err := time.Parse(time.RFC3339, af)
			if err == nil {
				updateData["availableFrom"] = t
			}
		}

		filter := bson.M{"_id": objID, "createdBy": userID}
		update := bson.M{"$set": updateData}

		res, err := config.PropertyCollection.UpdateOne(context.TODO(), filter, update)
		if err != nil {
			log.Printf("Update failed: %v", err)
			http.Error(w, "Update failed", http.StatusInternalServerError)
			return
		}

		if res.MatchedCount == 0 {
			log.Println("No property found or unauthorized")
			http.Error(w, "No property found or unauthorized", http.StatusForbidden)
			return
		}

		json.NewEncoder(w).Encode(map[string]string{"message": "Property updated successfully"})
	}
}

func DeleteProperty(_ *mongo.Client) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		userID, ok := r.Context().Value(UserIDKey).(string)
		if !ok {
			log.Println("User ID missing in context")
			http.Error(w, "User ID missing in context", http.StatusUnauthorized)
			return
		}

		propertyID := mux.Vars(r)["id"]
		objID, err := primitive.ObjectIDFromHex(propertyID)
		if err != nil {
			log.Println("Invalid property ID ", err)
			http.Error(w, "Invalid property ID", http.StatusBadRequest)
			return
		}

		filter := bson.M{"_id": objID, "createdBy": userID}

		res, err := config.PropertyCollection.DeleteOne(context.TODO(), filter)
		if err != nil {
			log.Printf("Delete failed: %v", err)
			http.Error(w, "Delete failed", http.StatusInternalServerError)
			return
		}

		if res.DeletedCount == 0 {
			log.Println("No property found or unauthorized")
			http.Error(w, "No property found or unauthorized", http.StatusForbidden)
			return
		}

		json.NewEncoder(w).Encode(map[string]string{"message": "Property deleted successfully"})
	}
}
