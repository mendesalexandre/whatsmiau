package whatsmiau

import (
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
