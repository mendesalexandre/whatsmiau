package whatsmiau

import (
	"strings"
	"testing"

	"go.mau.fi/whatsmeow/proto/waE2E"
	"google.golang.org/protobuf/proto"
)

func TestParseWAMessageImageDirectViewOnceFlag(t *testing.T) {
	s := &Whatsmiau{}
	m := &waE2E.Message{
		ImageMessage: &waE2E.ImageMessage{
			Caption:  proto.String("doc"),
			ViewOnce: proto.Bool(true),
		},
	}

	messageType, raw, _ := s.parseWAMessage(m)

	if messageType != "imageMessage" {
		t.Fatalf("expected imageMessage, got %q", messageType)
	}
	if raw.ImageMessage == nil || !raw.ImageMessage.ViewOnce {
		t.Fatalf("expected ImageMessage.ViewOnce=true, got %+v", raw.ImageMessage)
	}
}

func TestParseWAMessageUnwrapsViewOnceMessageWrapper(t *testing.T) {
	s := &Whatsmiau{}
	m := &waE2E.Message{
		ViewOnceMessage: &waE2E.FutureProofMessage{
			Message: &waE2E.Message{
				ImageMessage: &waE2E.ImageMessage{
					Caption: proto.String("doc"),
				},
			},
		},
	}

	messageType, raw, _ := s.parseWAMessage(m)

	if messageType != "imageMessage" {
		t.Fatalf("expected imageMessage (unwrapped), got %q", messageType)
	}
	if raw.ImageMessage == nil || !raw.ImageMessage.ViewOnce {
		t.Fatalf("expected ImageMessage.ViewOnce forced true via wrapper, got %+v", raw.ImageMessage)
	}
}

func TestParseWAMessageUnwrapsViewOnceMessageV2WrapperVideo(t *testing.T) {
	s := &Whatsmiau{}
	m := &waE2E.Message{
		ViewOnceMessageV2: &waE2E.FutureProofMessage{
			Message: &waE2E.Message{
				VideoMessage: &waE2E.VideoMessage{
					Caption: proto.String("clip"),
				},
			},
		},
	}

	messageType, raw, _ := s.parseWAMessage(m)

	if messageType != "videoMessage" {
		t.Fatalf("expected videoMessage (unwrapped), got %q", messageType)
	}
	if raw.VideoMessage == nil || !raw.VideoMessage.ViewOnce {
		t.Fatalf("expected VideoMessage.ViewOnce forced true via wrapper, got %+v", raw.VideoMessage)
	}
}

func TestParseWAMessageUnwrapsViewOnceMessageV2ExtensionWrapper(t *testing.T) {
	s := &Whatsmiau{}
	m := &waE2E.Message{
		ViewOnceMessageV2Extension: &waE2E.FutureProofMessage{
			Message: &waE2E.Message{
				AudioMessage: &waE2E.AudioMessage{
					PTT: proto.Bool(true),
				},
			},
		},
	}

	messageType, raw, _ := s.parseWAMessage(m)

	if messageType != "audioMessage" {
		t.Fatalf("expected audioMessage (unwrapped), got %q", messageType)
	}
	if raw.AudioMessage == nil || !raw.AudioMessage.ViewOnce {
		t.Fatalf("expected AudioMessage.ViewOnce forced true via wrapper, got %+v", raw.AudioMessage)
	}
}

func TestParseWAMessageNonViewOnceUnaffected(t *testing.T) {
	s := &Whatsmiau{}
	m := &waE2E.Message{
		Conversation: proto.String("oi"),
	}

	messageType, raw, _ := s.parseWAMessage(m)

	if messageType != "conversation" {
		t.Fatalf("expected conversation, got %q", messageType)
	}
	if raw.Conversation != "oi" {
		t.Fatalf("expected conversation text preserved, got %q", raw.Conversation)
	}
}

// Achado real (2026-08-19): ListMessage não tem case dedicado —
// cai em "unknown". Antes desse fix, raw.RawUnknown ficava sempre vazio,
// perdendo qualquer chance de reconstruir o que o cliente mandou depois.
func TestParseWAMessageUnknownTypeDumpsRawProtoJSON(t *testing.T) {
	s := &Whatsmiau{}
	m := &waE2E.Message{
		ListMessage: &waE2E.ListMessage{
			Title:       proto.String("Escolha uma opção"),
			Description: proto.String("teste"),
		},
	}

	messageType, raw, _ := s.parseWAMessage(m)

	if messageType != "unknown" {
		t.Fatalf("expected unknown, got %q", messageType)
	}
	if len(raw.RawUnknown) == 0 {
		t.Fatalf("expected RawUnknown populated with the raw proto dump, got empty")
	}
	if !strings.Contains(string(raw.RawUnknown), "Escolha uma opção") {
		t.Fatalf("expected RawUnknown to contain the original field content, got %q", string(raw.RawUnknown))
	}
}

func TestParseWAMessageKnownTypeLeavesRawUnknownEmpty(t *testing.T) {
	s := &Whatsmiau{}
	m := &waE2E.Message{
		Conversation: proto.String("oi"),
	}

	_, raw, _ := s.parseWAMessage(m)

	if len(raw.RawUnknown) != 0 {
		t.Fatalf("expected RawUnknown empty for a known/handled type, got %q", string(raw.RawUnknown))
	}
}
