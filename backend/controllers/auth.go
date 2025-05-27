package controllers

import (
	"context"
	"encoding/json"
	"log"
	"net/http"
	"time"

	"github.com/dcode-github/property_lisitng_system/backend/config"
	"github.com/dcode-github/property_lisitng_system/backend/models"
	"github.com/dcode-github/property_lisitng_system/backend/utils"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
)

type Response struct {
	Message string `json:"message"`
	Token   string `json:"token,omitempty"`
}

type ErrorResponse struct {
	Message string `json:"message"`
}

func RegisterUser(client *mongo.Client) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var user models.User
		if err := json.NewDecoder(r.Body).Decode(&user); err != nil {
			log.Printf("Error decoding user data: %v", err)
			http.Error(w, "Invalid request payload", http.StatusBadRequest)
			return
		}

		exists := config.UserCollection.FindOne(context.TODO(), bson.M{"userID": user.UserID})
		if exists.Err() == nil {
			log.Printf("UserID already exists: %s", user.UserID)
			http.Error(w, "UserID already exists", http.StatusConflict)
			return
		}

		exists = config.UserCollection.FindOne(context.TODO(), bson.M{"email": user.Email})
		if exists.Err() == nil {
			log.Printf("User email already exists: %s", user.Email)
			http.Error(w, "Email already exists", http.StatusConflict)
			return
		}

		hashedPwd, err := utils.HashPassword(user.Password)
		if err != nil {
			log.Printf("Error hashing password: %v", err)
			http.Error(w, "Failed to hash password", http.StatusInternalServerError)
			return
		}
		user.Password = hashedPwd
		user.CreatedAt = time.Now()

		_, err = config.UserCollection.InsertOne(context.TODO(), user)
		if err != nil {
			log.Printf("Error inserting user into the database: %v", err)
			http.Error(w, "Failed to create user", http.StatusInternalServerError)
			return
		}

		w.WriteHeader(http.StatusCreated)
		json.NewEncoder(w).Encode(Response{Message: "User registered successfully"})
	}
}

func LoginUser(client *mongo.Client) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var credentials models.User
		if err := json.NewDecoder(r.Body).Decode(&credentials); err != nil {
			log.Printf("Error decoding login credentials: %v", err)
			http.Error(w, "Invalid payload", http.StatusBadRequest)
			return
		}

		var dbUser models.User
		err := config.UserCollection.FindOne(context.TODO(), bson.M{"userID": credentials.UserID}).Decode(&dbUser)
		if err != nil {
			log.Printf("User not found: %s", credentials.UserID)
			http.Error(w, "User not found", http.StatusUnauthorized)
			return
		}

		if !utils.CheckPasswordHash(credentials.Password, dbUser.Password) {
			log.Printf("Invalid credentials for user: %s", credentials.UserID)
			http.Error(w, "Invalid credentials", http.StatusUnauthorized)
			return
		}

		token, err := utils.GenerateJWT(dbUser.UserID)
		if err != nil {
			log.Printf("Error generating JWT token: %v", err)
			http.Error(w, "Failed to generate token", http.StatusInternalServerError)
			return
		}

		json.NewEncoder(w).Encode(Response{Message: "Login successful", Token: token})
	}
}
