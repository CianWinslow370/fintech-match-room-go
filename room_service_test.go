package main

import (
	"context"
	"testing"
)

type recordingPublisher struct{ events []string }

func (p *recordingPublisher) createChannel(_ context.Context, _, _ string) error { return nil }
func (p *recordingPublisher) publish(_ context.Context, _, event string, _ any, _, _ string) error {
	p.events = append(p.events, event)
	return nil
}

func TestPaymentDecision(t *testing.T) {
	for _, tc := range []struct {
		name   string
		amount int64
		flag   bool
		want   string
	}{
		{"ordinary", 2500, false, "recorded"},
		{"threshold", 100001, false, "review"},
		{"risk signal", 2500, true, "review"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			pub := &recordingPublisher{}
			s := newRoomService(pub)
			r, err := s.join(context.Background(), joinInput{RequestID: "req-1", AccountID: "alice"})
			if err != nil {
				t.Fatal(err)
			}
			result, err := s.payment(context.Background(), paymentInput{EventID: "pay-1", RoomID: r.ID, AccountID: "alice", AmountMinor: tc.amount, RiskFlag: tc.flag})
			if err != nil {
				t.Fatal(err)
			}
			if result.Decision != tc.want || pub.events[1] != "payment."+tc.want {
				t.Fatalf("result=%+v events=%v", result, pub.events)
			}
			_, err = s.payment(context.Background(), paymentInput{EventID: "pay-1", RoomID: r.ID, AccountID: "alice", AmountMinor: tc.amount, RiskFlag: tc.flag})
			if err != nil || len(pub.events) != 2 {
				t.Fatalf("duplicate event published: %v, %v", pub.events, err)
			}
		})
	}
}

func TestMatchPairsDifferentAccounts(t *testing.T) {
	s := newRoomService(&recordingPublisher{})
	first, _ := s.join(context.Background(), joinInput{RequestID: "a", AccountID: "alice"})
	second, err := s.join(context.Background(), joinInput{RequestID: "b", AccountID: "bob"})
	if err != nil || second.ID != first.ID || len(second.Members) != 2 {
		t.Fatalf("pair=%+v error=%v", second, err)
	}
}
