package models

import "go.mongodb.org/mongo-driver/bson/primitive"

type Recommendation struct {
	ID         primitive.ObjectID `bson:"_id,omitempty" json:"id"`
	FromUserID primitive.ObjectID `bson:"fromUserId" json:"fromUserId"`
	ToUserID   primitive.ObjectID `bson:"toUserId" json:"toUserId"`
	PropertyID primitive.ObjectID `bson:"propertyId" json:"propertyId"`
}
