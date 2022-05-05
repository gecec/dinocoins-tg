package main

import "time"

const (
	OpenStatus      = "OPEN"
	CompletedStatus = "COMPLETED"
	CanceledStatus  = "CANCELED"
)

type Transaction struct {
	ID        string    `json:"id" bson:"_id"`
	Timestamp time.Time `json:"time" bson:"time"`
	Operation string    `json:"operation"`
	Cost      int       `json:"cost"`
	UserId    int64     `json:"user_id"`
	Status    string    `json: "status"`
}
