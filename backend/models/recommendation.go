package models

import "go.mongodb.org/mongo-driver/bson/primitive"

type Recommendation struct {
	ID         primitive.ObjectID `bson:"_id,omitempty" json:"id"`
	FromUserID string             `bson:"fromUserId" json:"fromUserId"`
	ToUserID   string             `bson:"toUserID" json:"toUserID"`
	ToEmailID  string             `bson:"toEmailID" json:"toEmailID"`
	PropertyID primitive.ObjectID `bson:"propertyID" json:"propertyID"`
}
