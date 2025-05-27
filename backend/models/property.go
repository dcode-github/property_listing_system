package models

import (
	"time"

	"go.mongodb.org/mongo-driver/bson/primitive"
)

type Property struct {
	ID            primitive.ObjectID `bson:"_id,omitempty" json:"_id"`
	PropId        string             `bson:"id" json:"id"`
	Title         string             `bson:"title" json:"title"`
	Type          string             `bson:"type" json:"type"`
	Price         int                `bson:"price" json:"price"`
	State         string             `bson:"state" json:"state"`
	City          string             `bson:"city" json:"city"`
	AreaSqFt      int                `bson:"areaSqFt" json:"areaSqFt"`
	Bedrooms      int                `bson:"bedrooms" json:"bedrooms"`
	Bathrooms     int                `bson:"bathrooms" json:"bathrooms"`
	Amenities     string             `bson:"amenities" json:"amenities"`
	Furnished     string             `bson:"furnished" json:"furnished"`
	AvailableFrom time.Time          `bson:"availableFrom" json:"availableFrom"`
	ListedBy      string             `bson:"listedBy" json:"listedBy"`
	Tags          string             `bson:"tags" json:"tags"`
	ColorTheme    string             `bson:"colorTheme" json:"colorTheme"`
	Rating        float64            `bson:"rating" json:"rating"`
	IsVerified    bool               `bson:"isVerified" json:"isVerified"`
	ListingType   string             `bson:"listingType" json:"listingType"`
	CreatedBy     string             `bson:"createdBy" json:"createdBy"`
}
