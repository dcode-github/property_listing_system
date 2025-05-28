package models

import "go.mongodb.org/mongo-driver/bson/primitive"

type Favorite struct {
	ID         primitive.ObjectID `bson:"_id,omitempty" json:"id"`
	UserID     string             `bson:"userID" json:"userID"`
	PropertyID primitive.ObjectID `bson:"propertyID" json:"propertyID"`
}
