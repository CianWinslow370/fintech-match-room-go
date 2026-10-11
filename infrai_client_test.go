package main

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestRealtimeRequestFields(t *testing.T) {
	requests := make(chan map[string]any, 2)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var body map[string]any
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			t.Fatal(err)
		}
		requests <- body
		_, _ = w.Write([]byte(`{"ok":true}`))
	}))
	defer server.Close()

	client := &realtimeClient{key: "test", base: server.URL, http: server.Client()}
	if err := client.createChannel(context.Background(), "room-1", "create-1"); err != nil {
		t.Fatal(err)
	}
	if err := client.publish(context.Background(), "room-1", "joined", map[string]any{"member": "alice"}, "account-1", "publish-1"); err != nil {
		t.Fatal(err)
	}

	create := <-requests
	if len(create) != 2 || create["channel"] != "room-1" || create["idempotency_key"] != "create-1" {
		t.Fatalf("create payload = %#v", create)
	}
	publish := <-requests
	if len(publish) != 5 || publish["channel"] != "room-1" || publish["idempotency_key"] != "publish-1" {
		t.Fatalf("publish payload = %#v", publish)
	}
}
