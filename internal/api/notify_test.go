package api

import (
	"testing"
	"time"

	"shop/internal/model"
)

func TestHubBroadcastDeliversToSubscriber(t *testing.T) {
	hub := newNotificationHub()
	ch := hub.subscribe("alice")
	defer hub.unsubscribe("alice", ch)

	hub.broadcast("alice", model.Notification{Title: "hi"})

	select {
	case got := <-ch:
		if got.Title != "hi" {
			t.Fatalf("unexpected title %q", got.Title)
		}
	case <-time.After(time.Second):
		t.Fatal("timed out waiting for broadcast")
	}
}

func TestHubBroadcastDoesNotCrossUsers(t *testing.T) {
	hub := newNotificationHub()
	alice := hub.subscribe("alice")
	defer hub.unsubscribe("alice", alice)
	bob := hub.subscribe("bob")
	defer hub.unsubscribe("bob", bob)

	hub.broadcast("alice", model.Notification{Title: "a"})

	select {
	case got := <-alice:
		if got.Title != "a" {
			t.Fatalf("unexpected title %q", got.Title)
		}
	case <-time.After(time.Second):
		t.Fatal("alice should receive")
	}
	select {
	case <-bob:
		t.Fatal("bob should not receive alice's notification")
	case <-time.After(50 * time.Millisecond):
	}
}

func TestNotifyBroadcastsWithoutDB(t *testing.T) {
	hub := newNotificationHub()
	ch := hub.subscribe("alice")
	defer hub.unsubscribe("alice", ch)

	notify(nil, hub, "alice", model.NotificationTypeOrder, "订单已发货", "内容", "NO123")

	select {
	case got := <-ch:
		if got.Type != model.NotificationTypeOrder || got.OrderNo != "NO123" {
			t.Fatalf("unexpected notification %+v", got)
		}
	case <-time.After(time.Second):
		t.Fatal("timed out waiting for notify broadcast")
	}
}

func TestStatusTextCoversAllStatuses(t *testing.T) {
	for _, s := range []string{
		model.OrderStatusPending,
		model.OrderStatusShipped,
		model.OrderStatusCompleted,
		model.OrderStatusAftersale,
		model.OrderStatusFinished,
	} {
		if statusText[s] == "" {
			t.Fatalf("missing status text for %q", s)
		}
	}
}
