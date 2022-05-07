package main

import "time"

const (
	CHILD  = iota
	PARENT = iota
)

type User struct {
	ID             int64     `json:"id"`
	Nickname       string    `json:"nickname"`
	Type           int       `json:"type"`
	RegistrationTS time.Time `json:"registration_ts" bson:"time"`
}
