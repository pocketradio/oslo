package queue

import "encoding/json"

type MessageType string

const (
	MessageTypeMatchRide    MessageType = "match_ride"
	MessageTypeOfferTimeout MessageType = "offer_timeout"
)

type Message struct {
	ID      string          `json:"id"`
	Type    MessageType     `json:"type"`
	Payload json.RawMessage `json:"payload"`
}

type MatchRideMessage struct {
	RideID string `json:"ride_id"`
}

type OfferTimeoutMessage struct {
	RideID  string `json:"ride_id"`
	OfferID string `json:"offer_id"`
}
