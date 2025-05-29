package controllers

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"log"
	"net/http"
	"net/url"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/dcode-github/property_lisitng_system/backend/config"
	"github.com/dcode-github/property_lisitng_system/backend/models"
	"github.com/gorilla/mux"

	"github.com/redis/go-redis/v9"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo/options"
)

type ContextKey string

const UserIDKey = ContextKey("userID")

func CreateProperty(redisClient *redis.Client) http.HandlerFunc {
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

		_, err := config.PropertyCollection.InsertOne(r.Context(), property)
		if err != nil {
			log.Printf("Insert failed: %v", err)
			http.Error(w, "Failed to create property", http.StatusInternalServerError)
			return
		}

		go func() {
			deletePropertyCache(redisClient)
		}()

		w.WriteHeader(http.StatusCreated)
		json.NewEncoder(w).Encode(property)
	}
}

func GetAllProperties(redisClient *redis.Client) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		userID, ok := r.Context().Value(UserIDKey).(string)
		if !ok {
			log.Println("User ID missing in context for GetAllProperties")
			http.Error(w, "User ID missing in context", http.StatusUnauthorized)
			return
		}

		query := r.URL.Query()
		cacheKey := generateCacheKey(userID, query)

		cachedData, err := redisClient.Get(r.Context(), cacheKey).Result()
		if err == nil {
			log.Printf("Cache Hit for key: %s", cacheKey)
			w.Header().Set("Content-Type", "application/json")
			w.Write([]byte(cachedData))
			return
		}
		if err != redis.Nil {
			log.Printf("Redis GET error for key %s: %v", cacheKey, err)
		}

		log.Printf("Cache Miss for key: %s", cacheKey)

		var andConditions []bson.M
		fieldSpecificConditions := make(map[string]bson.M)

		operatorMap := map[string]string{
			"eq": "$eq", "ne": "$ne", "gt": "$gt", "gte": "$gte", "lt": "$lt", "lte": "$lte",
		}
		numericFields := map[string]bool{
			"price": true, "areaSqFt": true, "bedrooms": true, "bathrooms": true, "rating": true,
		}
		dateFields := map[string]bool{"availableFrom": true}
		boolFields := map[string]bool{"isVerified": true}
		stringFields := map[string]bool{
			"id": true, "propId": true, "title": true, "type": true, "state": true, "city": true,
			"furnished": true, "listedBy": true, "listingType": true, "createdBy": true,
		}

		for rawKey, queryValues := range query {
			if rawKey == "userID" || len(queryValues) == 0 || queryValues[0] == "" {
				continue
			}

			fieldKey := rawKey
			mongoOperator := "$eq"

			if strings.Contains(rawKey, "[") && strings.Contains(rawKey, "]") {
				parts := strings.SplitN(rawKey, "[", 2)
				fieldKey = parts[0]
				opKey := strings.TrimSuffix(parts[1], "]")
				if mappedOp, exists := operatorMap[opKey]; exists {
					mongoOperator = mappedOp
				} else {
					log.Printf("Unknown operator key: %s in query param %s", opKey, rawKey)
					continue
				}
			}
			queryValue := queryValues[0]
			if fieldKey == "tags" || fieldKey == "amenities" {
				terms := strings.Split(queryValue, ",")
				var orClausesForField bson.A
				for _, term := range terms {
					trimmedTerm := strings.TrimSpace(term)
					if trimmedTerm == "" {
						continue
					}
					orClausesForField = append(orClausesForField, bson.M{fieldKey: bson.M{"$regex": primitive.Regex{Pattern: trimmedTerm, Options: "i"}}})
				}
				if len(orClausesForField) > 0 {
					andConditions = append(andConditions, bson.M{"$or": orClausesForField})
				}
				continue
			}

			if stringFields[fieldKey] {
				values := strings.Split(queryValue, ",")
				var trimmedValues []string
				for _, v := range values {
					trimmedV := strings.TrimSpace(v)
					if trimmedV != "" {
						trimmedValues = append(trimmedValues, trimmedV)
					}
				}
				if len(trimmedValues) > 0 {
					if mongoOperator == "$eq" {
						andConditions = append(andConditions, bson.M{fieldKey: bson.M{"$in": trimmedValues}})
					} else if mongoOperator == "$ne" {
						andConditions = append(andConditions, bson.M{fieldKey: bson.M{"$nin": trimmedValues}})
					} else {
						log.Printf("Unsupported operator '%s' for string field '%s'. Defaulting to $eq/$in.", mongoOperator, fieldKey)
						andConditions = append(andConditions, bson.M{fieldKey: bson.M{"$in": trimmedValues}})
					}
				}
				continue
			}

			if boolFields[fieldKey] {
				boolVal, err := strconv.ParseBool(strings.ToLower(queryValue))
				if err == nil {
					andConditions = append(andConditions, bson.M{fieldKey: bson.M{mongoOperator: boolVal}})
				} else {
					log.Printf("Invalid boolean value for %s: %s", fieldKey, queryValue)
				}
				continue
			}

			if numericFields[fieldKey] || dateFields[fieldKey] {
				if _, ok := fieldSpecificConditions[fieldKey]; !ok {
					fieldSpecificConditions[fieldKey] = bson.M{}
				}

				if numericFields[fieldKey] {
					numVal, err := strconv.ParseFloat(queryValue, 64)
					if err == nil {
						fieldSpecificConditions[fieldKey][mongoOperator] = numVal
					} else {
						log.Printf("Invalid numeric value for %s operator %s: %s. Error: %v", fieldKey, mongoOperator, queryValue, err)
					}
				} else {
					t, err := time.Parse("2006-01-02", queryValue)
					if err == nil {
						fieldSpecificConditions[fieldKey][mongoOperator] = t
					} else {
						log.Printf("Invalid date value for %s operator %s: %s. Error: %v", fieldKey, mongoOperator, queryValue, err)
					}
				}
				continue
			}
			log.Printf("Unhandled query parameter: %s (parsed as field: %s)", rawKey, fieldKey)
		}

		for field, conditionsMap := range fieldSpecificConditions {
			if len(conditionsMap) > 0 {
				andConditions = append(andConditions, bson.M{field: conditionsMap})
			}
		}

		finalMongoQuery := bson.M{}
		if len(andConditions) > 0 {
			finalMongoQuery["$and"] = andConditions
		}
		findOptions := options.Find().SetLimit(10)

		cursor, err := config.PropertyCollection.Find(r.Context(), finalMongoQuery, findOptions)
		if err != nil {
			log.Printf("Error fetching properties with query %+v: %v", finalMongoQuery, err)
			http.Error(w, "Error fetching properties", http.StatusInternalServerError)
			return
		}
		defer cursor.Close(r.Context())

		var properties []models.Property
		if err := cursor.All(r.Context(), &properties); err != nil {
			log.Printf("Error decoding properties: %v", err)
			http.Error(w, "Error decoding properties", http.StatusInternalServerError)
			return
		}

		if len(properties) > 0 {
			propertyIDs := make([]primitive.ObjectID, 0, len(properties))
			for _, prop := range properties {
				propertyIDs = append(propertyIDs, prop.ID)
			}

			favFilter := bson.M{
				"userID":     userID,
				"propertyID": bson.M{"$in": propertyIDs},
			}

			favCursor, err := config.FavoriteCollection.Find(r.Context(), favFilter)
			if err != nil {
				log.Printf("Error fetching favorites for user %s: %v", userID, err)
			} else {
				defer favCursor.Close(r.Context())
				favMap := make(map[primitive.ObjectID]bool)
				for favCursor.Next(r.Context()) {
					var fav models.Favorite
					if err := favCursor.Decode(&fav); err != nil {
						log.Printf("Error decoding favorite: %v", err)
						continue
					}
					favMap[fav.PropertyID] = true
				}
				if favCursor.Err() != nil {
					log.Printf("Favorite cursor iteration error: %v", favCursor.Err())
				}

				for i := range properties {
					if _, isFav := favMap[properties[i].ID]; isFav {
						properties[i].IsFavorite = true
					}
				}
			}
		}

		resultBytes, err := json.Marshal(properties)
		if err != nil {
			log.Printf("Failed to serialize properties: %v", err)
			http.Error(w, "Failed to encode response", http.StatusInternalServerError)
			return
		}

		err = redisClient.Set(r.Context(), cacheKey, resultBytes, 10*time.Minute).Err()
		if err != nil {
			log.Printf("Failed to cache response for key %s: %v", cacheKey, err)
		}

		w.Header().Set("Content-Type", "application/json")
		w.Write(resultBytes)
	}
}

func UpdateProperty(redisClient *redis.Client) http.HandlerFunc {
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
			log.Printf("Invalid property ID %s: %v", propertyID, err)
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
		delete(updateData, "id")
		delete(updateData, "propId")
		delete(updateData, "createdBy")

		if af, ok := updateData["availableFrom"].(string); ok {
			t, err := time.Parse(time.RFC3339, af)
			if err == nil {
				updateData["availableFrom"] = t
			} else {
				log.Printf("Could not parse 'availableFrom' string '%s' as RFC3339 time: %v", af, err)
			}
		}

		filter := bson.M{"_id": objID, "createdBy": userID}
		update := bson.M{"$set": updateData}

		res, err := config.PropertyCollection.UpdateOne(r.Context(), filter, update)
		if err != nil {
			log.Printf("Update failed for property %s: %v", propertyID, err)
			http.Error(w, "Update failed", http.StatusInternalServerError)
			return
		}

		if res.MatchedCount == 0 {
			log.Printf("No property found with ID %s and createdBy %s, or unauthorized to update.", propertyID, userID)
			http.Error(w, "No property found or unauthorized", http.StatusForbidden)
			return
		}

		go func() {
			deletePropertyCache(redisClient)
		}()

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]string{"message": "Property updated successfully"})
	}
}

func DeleteProperty(redisClient *redis.Client) http.HandlerFunc {
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
			log.Printf("Invalid property ID %s: %v", propertyID, err)
			http.Error(w, "Invalid property ID", http.StatusBadRequest)
			return
		}

		filter := bson.M{"_id": objID, "createdBy": userID}

		res, err := config.PropertyCollection.DeleteOne(r.Context(), filter)
		if err != nil {
			log.Printf("Delete failed for property %s: %v", propertyID, err)
			http.Error(w, "Delete failed", http.StatusInternalServerError)
			return
		}

		if res.DeletedCount == 0 {
			log.Printf("No property found with ID %s and createdBy %s, or unauthorized to delete.", propertyID, userID)
			http.Error(w, "No property found or unauthorized", http.StatusForbidden)
			return
		}

		go func() {
			deletePropertyCache(redisClient)
		}()

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]string{"message": "Property deleted successfully"})
	}
}

func generateCacheKey(userID string, queryParams url.Values) string {
	keys := make([]string, 0, len(queryParams))
	for k := range queryParams {
		keys = append(keys, k)
	}
	sort.Strings(keys)

	var sb strings.Builder
	sb.WriteString(userID)
	sb.WriteString(":")

	for _, key := range keys {
		values := queryParams[key]
		sort.Strings(values)
		for _, val := range values {
			sb.WriteString(key)
			sb.WriteString("=")
			sb.WriteString(val)
			sb.WriteString("&")
		}
	}
	rawKey := strings.TrimSuffix(sb.String(), "&")

	sum := sha256.Sum256([]byte(rawKey))
	return "property:" + hex.EncodeToString(sum[:])
}

func deletePropertyCache(redisClient *redis.Client) {
	ctx := context.Background()
	const scanPattern = "property:*"
	const scanCount = 100

	var keysToDelete []string
	var cursor uint64
	var err error

	log.Println("Starting property cache invalidation...")

	for {
		var currentKeys []string
		currentKeys, cursor, err = redisClient.Scan(ctx, cursor, scanPattern, scanCount).Result()
		if err != nil {
			log.Printf("Error during Redis SCAN for pattern '%s': %v", scanPattern, err)
			return
		}
		keysToDelete = append(keysToDelete, currentKeys...)
		if cursor == 0 {
			break
		}
	}

	if len(keysToDelete) == 0 {
		log.Println("No property cache keys found matching pattern to delete.")
		return
	}

	pipe := redisClient.Pipeline()
	for _, key := range keysToDelete {
		pipe.Del(ctx, key)
	}
	_, execErr := pipe.Exec(ctx)

	if execErr != nil {
		log.Printf("Error executing pipeline for deleting %d property cache keys: %v", len(keysToDelete), execErr)
	} else {
		log.Printf("Property Cache Invalidated. Successfully deleted %d keys matching '%s'.", len(keysToDelete), scanPattern)
	}
}
