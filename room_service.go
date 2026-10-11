package main

import (
	"context"
	"fmt"
	"sync"
)

type publisher interface {
	createChannel(context.Context, string, string) error
	publish(context.Context, string, string, any, string, string) error
}

type domainError string

func (e domainError) Error() string { return string(e) }

type room struct {
	ID      string   `json:"room_id"`
	Members []string `json:"members"`
}

type roomService struct {
	mu        sync.Mutex
	rooms     []room
	byRequest map[string]room
	payments  map[string]paymentResult
	pub       publisher
}

type joinInput struct {
	RequestID string `json:"request_id"`
	AccountID string `json:"account_id"`
}

type paymentInput struct {
	EventID     string `json:"event_id"`
	RoomID      string `json:"room_id"`
	AccountID   string `json:"account_id"`
	AmountMinor int64  `json:"amount_minor"`
	RiskFlag    bool   `json:"risk_flag"`
}

type paymentResult struct {
	EventID  string `json:"event_id"`
	RoomID   string `json:"room_id"`
	Decision string `json:"decision"`
}

func newRoomService(pub publisher) *roomService {
	return &roomService{pub: pub, byRequest: make(map[string]room), payments: make(map[string]paymentResult)}
}

func (s *roomService) join(ctx context.Context, in joinInput) (room, error) {
	if in.RequestID == "" || in.AccountID == "" {
		return room{}, domainError("request_id and account_id are required")
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if previous, ok := s.byRequest[in.RequestID]; ok {
		return previous, nil
	}
	index := -1
	for i, r := range s.rooms {
		if len(r.Members) == 1 && r.Members[0] != in.AccountID {
			index = i
			break
		}
		for _, member := range r.Members {
			if member == in.AccountID {
				return room{}, domainError("account already in a room")
			}
		}
	}
	if index == -1 {
		id := "match-" + in.RequestID
		if err := s.pub.createChannel(ctx, id, "channel:"+in.RequestID); err != nil {
			return room{}, err
		}
		candidate := room{ID: id, Members: []string{in.AccountID}}
		if err := s.pub.publish(ctx, id, "room.joined", candidate, in.AccountID, "join:"+in.RequestID); err != nil {
			return room{}, err
		}
		s.rooms = append(s.rooms, candidate)
		s.byRequest[in.RequestID] = candidate
		return candidate, nil
	}
	candidate := room{ID: s.rooms[index].ID, Members: []string{s.rooms[index].Members[0], in.AccountID}}
	if err := s.pub.publish(ctx, candidate.ID, "room.joined", candidate, in.AccountID, "join:"+in.RequestID); err != nil {
		return room{}, err
	}
	s.rooms[index] = candidate
	s.byRequest[in.RequestID] = candidate
	return candidate, nil
}

func decidePayment(in paymentInput) string {
	if in.RiskFlag || in.AmountMinor > 100_000 {
		return "review"
	}
	return "recorded"
}

func (s *roomService) payment(ctx context.Context, in paymentInput) (paymentResult, error) {
	if in.EventID == "" || in.RoomID == "" || in.AccountID == "" || in.AmountMinor <= 0 {
		return paymentResult{}, domainError("event_id, room_id, account_id and positive amount_minor are required")
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if previous, ok := s.payments[in.EventID]; ok {
		return previous, nil
	}
	found := false
	for _, r := range s.rooms {
		if r.ID != in.RoomID {
			continue
		}
		for _, member := range r.Members {
			if member == in.AccountID {
				found = true
			}
		}
	}
	if !found {
		return paymentResult{}, domainError(fmt.Sprintf("account is not a member of room %s", in.RoomID))
	}
	result := paymentResult{EventID: in.EventID, RoomID: in.RoomID, Decision: decidePayment(in)}
	// Publish the immutable event before committing the local decision.
	if err := s.pub.publish(ctx, in.RoomID, "payment."+result.Decision, map[string]any{
		"event_id": in.EventID, "account_id": in.AccountID, "amount_minor": in.AmountMinor, "decision": result.Decision,
	}, in.AccountID, "payment:"+in.EventID); err != nil {
		return paymentResult{}, err
	}
	s.payments[in.EventID] = result
	return result, nil
}
