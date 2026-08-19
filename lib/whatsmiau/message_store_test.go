package whatsmiau

import (
	"context"
	"database/sql"
	"encoding/json"
	"testing"

	_ "github.com/mattn/go-sqlite3"
)

func openTestStore(t *testing.T) *messageStore {
	t.Helper()

	db, err := sql.Open("sqlite3", ":memory:")
	if err != nil {
		t.Fatalf("failed to open in-memory sqlite: %v", err)
	}
	t.Cleanup(func() { _ = db.Close() })

	store, err := newMessageStore(db, "sqlite3")
	if err != nil {
		t.Fatalf("failed to create message store: %v", err)
	}
	return store
}

func TestMessageStoreUpsertAndFindChats(t *testing.T) {
	store := openTestStore(t)
	ctx := context.Background()

	msg := storedMessage{
		MessageID:        "MSG-1",
		RemoteJID:        "5511999998888@s.whatsapp.net",
		FromMe:           false,
		PushName:         "Test",
		MessageType:      "conversation",
		MessageTimestamp: 1000,
		MessageJSON:      json.RawMessage(`{"conversation":"oi"}`),
	}

	if err := store.upsert(ctx, "instance-a", msg); err != nil {
		t.Fatalf("upsert failed: %v", err)
	}

	// Mensagem de outra instância não deve aparecer.
	other := msg
	other.MessageID = "MSG-2"
	if err := store.upsert(ctx, "instance-b", other); err != nil {
		t.Fatalf("upsert (instance-b) failed: %v", err)
	}

	chats, err := store.findChats(ctx, "instance-a", 10)
	if err != nil {
		t.Fatalf("findChats failed: %v", err)
	}
	if len(chats) != 1 || chats[0] != msg.RemoteJID {
		t.Fatalf("expected [%s], got %v", msg.RemoteJID, chats)
	}
}

func TestMessageStoreUpsertIsIdempotent(t *testing.T) {
	store := openTestStore(t)
	ctx := context.Background()

	msg := storedMessage{
		MessageID:        "MSG-DUP",
		RemoteJID:        "5511999998888@s.whatsapp.net",
		MessageType:      "conversation",
		MessageTimestamp: 1000,
		MessageJSON:      json.RawMessage(`{"conversation":"oi"}`),
	}

	if err := store.upsert(ctx, "instance-a", msg); err != nil {
		t.Fatalf("first upsert failed: %v", err)
	}
	if err := store.upsert(ctx, "instance-a", msg); err != nil {
		t.Fatalf("second upsert (duplicate) failed: %v", err)
	}

	_, total, err := store.findMessages(ctx, "instance-a", msg.RemoteJID, 1, 50)
	if err != nil {
		t.Fatalf("findMessages failed: %v", err)
	}
	if total != 1 {
		t.Fatalf("expected 1 message after duplicate upsert, got %d", total)
	}
}

func TestMessageStoreFindMessagesPagination(t *testing.T) {
	store := openTestStore(t)
	ctx := context.Background()
	jid := "5511999998888@s.whatsapp.net"

	for i := 0; i < 5; i++ {
		msg := storedMessage{
			MessageID:        "MSG-" + string(rune('A'+i)),
			RemoteJID:        jid,
			MessageType:      "conversation",
			MessageTimestamp: int64(1000 + i),
			MessageJSON:      json.RawMessage(`{"conversation":"msg"}`),
		}
		if err := store.upsert(ctx, "instance-a", msg); err != nil {
			t.Fatalf("upsert failed: %v", err)
		}
	}

	page1, total, err := store.findMessages(ctx, "instance-a", jid, 1, 2)
	if err != nil {
		t.Fatalf("findMessages page 1 failed: %v", err)
	}
	if total != 5 {
		t.Fatalf("expected total=5, got %d", total)
	}
	if len(page1) != 2 {
		t.Fatalf("expected 2 records on page 1, got %d", len(page1))
	}
	// Mais recente primeiro — timestamp 1004 deve vir antes de 1003.
	if page1[0].MessageTimestamp != 1004 {
		t.Fatalf("expected most recent message first (1004), got %d", page1[0].MessageTimestamp)
	}

	page3, _, err := store.findMessages(ctx, "instance-a", jid, 3, 2)
	if err != nil {
		t.Fatalf("findMessages page 3 failed: %v", err)
	}
	if len(page3) != 1 {
		t.Fatalf("expected 1 record on page 3 (5 total, 2 per page), got %d", len(page3))
	}
}

func TestMessageStoreFindChatsEmptyWhenNoMessages(t *testing.T) {
	store := openTestStore(t)
	chats, err := store.findChats(context.Background(), "instance-none", 10)
	if err != nil {
		t.Fatalf("findChats failed: %v", err)
	}
	if len(chats) != 0 {
		t.Fatalf("expected no chats, got %v", chats)
	}
}
