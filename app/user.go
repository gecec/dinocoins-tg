package main

const (
	CHILD = iota
	ADULT = iota
)

type User struct {
	ID       int64
	Nickname string
	Type     int
}
