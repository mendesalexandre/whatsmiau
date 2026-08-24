package whatsmiau

import (
	"strings"
	"testing"

	"go.mau.fi/whatsmeow/proto/waCommon"
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

// Achado real (2026-08-19): tipos de proto sem case dedicado (ex:
// ProductMessage) caem em "unknown". Antes desse fix, raw.RawUnknown ficava
// sempre vazio, perdendo qualquer chance de reconstruir o que o cliente
// mandou depois. ListMessage NÃO serve mais como exemplo aqui — ganhou case
// dedicado em 2026-08-20 (ver TestParseWAMessageListMessage*).
func TestParseWAMessageUnknownTypeDumpsRawProtoJSON(t *testing.T) {
	s := &Whatsmiau{}
	m := &waE2E.Message{
		ProductMessage: &waE2E.ProductMessage{
			Body: proto.String("Escolha um produto"),
		},
	}

	messageType, raw, _ := s.parseWAMessage(m)

	if messageType != "unknown" {
		t.Fatalf("expected unknown, got %q", messageType)
	}
	if len(raw.RawUnknown) == 0 {
		t.Fatalf("expected RawUnknown populated with the raw proto dump, got empty")
	}
	if !strings.Contains(string(raw.RawUnknown), "Escolha um produto") {
		t.Fatalf("expected RawUnknown to contain the original field content, got %q", string(raw.RawUnknown))
	}
}

// Achado real (2026-08-20): conta business enviando um menu ListMessage
// (sections/rows) — protocolo diferente do InteractiveMessage/ButtonsMessage
// já suportado (Fiagril, 2026-08-14), mesma classe de problema.
func TestParseWAMessageListMessageSingleSection(t *testing.T) {
	s := &Whatsmiau{}
	m := &waE2E.Message{
		ListMessage: &waE2E.ListMessage{
			Description: proto.String("Como podemos ajudá-lo?"),
			FooterText:  proto.String("Selecione uma opção"),
			Sections: []*waE2E.ListMessage_Section{
				{
					Rows: []*waE2E.ListMessage_Row{
						{Title: proto.String("Certidão"), RowID: proto.String("row-1")},
						{Title: proto.String("Titulos e Documentos"), RowID: proto.String("row-2"), Description: proto.String("(Cartas de Anuência, Contratos de Cessão, etc)")},
					},
				},
			},
		},
	}

	messageType, raw, _ := s.parseWAMessage(m)

	if messageType != "interactiveMessage" {
		t.Fatalf("expected interactiveMessage, got %q", messageType)
	}
	if raw.InteractiveMessage == nil {
		t.Fatalf("expected InteractiveMessage populated")
	}
	if raw.InteractiveMessage.Body != "Como podemos ajudá-lo?" {
		t.Fatalf("expected body preserved, got %q", raw.InteractiveMessage.Body)
	}
	if raw.InteractiveMessage.Footer != "Selecione uma opção" {
		t.Fatalf("expected footer preserved, got %q", raw.InteractiveMessage.Footer)
	}
	if len(raw.InteractiveMessage.Buttons) != 2 {
		t.Fatalf("expected 2 buttons, got %d", len(raw.InteractiveMessage.Buttons))
	}
	// Section única — sem prefixo de título de section.
	if raw.InteractiveMessage.Buttons[0].DisplayText != "Certidão" {
		t.Fatalf("expected first option 'Certidão' without prefix, got %q", raw.InteractiveMessage.Buttons[0].DisplayText)
	}
	if raw.InteractiveMessage.Buttons[0].Description != "" {
		t.Fatalf("expected first option without description, got %q", raw.InteractiveMessage.Buttons[0].Description)
	}
	if raw.InteractiveMessage.Buttons[1].Id != "row-2" {
		t.Fatalf("expected row id preserved, got %q", raw.InteractiveMessage.Buttons[1].Id)
	}
	if raw.InteractiveMessage.Buttons[1].Description != "(Cartas de Anuência, Contratos de Cessão, etc)" {
		t.Fatalf("expected second option description preserved, got %q", raw.InteractiveMessage.Buttons[1].Description)
	}
}

func TestParseWAMessageListMessageMultipleSectionsPrefixesTitle(t *testing.T) {
	s := &Whatsmiau{}
	m := &waE2E.Message{
		ListMessage: &waE2E.ListMessage{
			Sections: []*waE2E.ListMessage_Section{
				{
					Title: proto.String("Financeiro"),
					Rows: []*waE2E.ListMessage_Row{
						{Title: proto.String("Boleto"), RowID: proto.String("row-1")},
					},
				},
				{
					Title: proto.String("Suporte"),
					Rows: []*waE2E.ListMessage_Row{
						{Title: proto.String("Reclamação"), RowID: proto.String("row-2")},
					},
				},
			},
		},
	}

	_, raw, _ := s.parseWAMessage(m)

	if len(raw.InteractiveMessage.Buttons) != 2 {
		t.Fatalf("expected 2 buttons, got %d", len(raw.InteractiveMessage.Buttons))
	}
	if raw.InteractiveMessage.Buttons[0].DisplayText != "Financeiro: Boleto" {
		t.Fatalf("expected section title prefixed, got %q", raw.InteractiveMessage.Buttons[0].DisplayText)
	}
	if raw.InteractiveMessage.Buttons[1].DisplayText != "Suporte: Reclamação" {
		t.Fatalf("expected section title prefixed, got %q", raw.InteractiveMessage.Buttons[1].DisplayText)
	}
}

// TemplateMessage — terceiro formato de menu de conta Business API, além de
// InteractiveMessage/ButtonsMessage e ListMessage (2026-08-20).
func TestParseWAMessageTemplateMessageHydratedButtons(t *testing.T) {
	s := &Whatsmiau{}
	m := &waE2E.Message{
		TemplateMessage: &waE2E.TemplateMessage{
			HydratedTemplate: &waE2E.TemplateMessage_HydratedFourRowTemplate{
				Title:               &waE2E.TemplateMessage_HydratedFourRowTemplate_HydratedTitleText{HydratedTitleText: "Confirmação de agendamento"},
				HydratedContentText: proto.String("Seu horário está marcado"),
				HydratedFooterText:  proto.String("Cartório XYZ"),
				HydratedButtons: []*waE2E.HydratedTemplateButton{
					{HydratedButton: &waE2E.HydratedTemplateButton_QuickReplyButton{
						QuickReplyButton: &waE2E.HydratedTemplateButton_HydratedQuickReplyButton{
							DisplayText: proto.String("Confirmar"), ID: proto.String("btn-1"),
						},
					}},
					{HydratedButton: &waE2E.HydratedTemplateButton_UrlButton{
						UrlButton: &waE2E.HydratedTemplateButton_HydratedURLButton{
							DisplayText: proto.String("Ver no mapa"), URL: proto.String("https://maps.example.com"),
						},
					}},
					{HydratedButton: &waE2E.HydratedTemplateButton_CallButton{
						CallButton: &waE2E.HydratedTemplateButton_HydratedCallButton{
							DisplayText: proto.String("Ligar"), PhoneNumber: proto.String("+5511999998888"),
						},
					}},
				},
			},
		},
	}

	messageType, raw, _ := s.parseWAMessage(m)

	if messageType != "interactiveMessage" {
		t.Fatalf("expected interactiveMessage, got %q", messageType)
	}
	if raw.InteractiveMessage.Title != "Confirmação de agendamento" {
		t.Fatalf("expected title preserved, got %q", raw.InteractiveMessage.Title)
	}
	if raw.InteractiveMessage.Body != "Seu horário está marcado" {
		t.Fatalf("expected body preserved, got %q", raw.InteractiveMessage.Body)
	}
	if len(raw.InteractiveMessage.Buttons) != 3 {
		t.Fatalf("expected 3 buttons, got %d", len(raw.InteractiveMessage.Buttons))
	}
	if raw.InteractiveMessage.Buttons[0].Type != "quick_reply" || raw.InteractiveMessage.Buttons[0].Id != "btn-1" {
		t.Fatalf("expected quick_reply button with id preserved, got %+v", raw.InteractiveMessage.Buttons[0])
	}
	if raw.InteractiveMessage.Buttons[1].Type != "cta_url" || raw.InteractiveMessage.Buttons[1].Url != "https://maps.example.com" {
		t.Fatalf("expected cta_url button with url preserved, got %+v", raw.InteractiveMessage.Buttons[1])
	}
	if raw.InteractiveMessage.Buttons[2].Type != "cta_call" || raw.InteractiveMessage.Buttons[2].Id != "+5511999998888" {
		t.Fatalf("expected cta_call button with phone preserved, got %+v", raw.InteractiveMessage.Buttons[2])
	}
}

func TestParseWAMessageTemplateMessageDelegatesToInteractiveMessage(t *testing.T) {
	s := &Whatsmiau{}
	m := &waE2E.Message{
		TemplateMessage: &waE2E.TemplateMessage{
			Format: &waE2E.TemplateMessage_InteractiveMessageTemplate{
				InteractiveMessageTemplate: &waE2E.InteractiveMessage{
					Body: &waE2E.InteractiveMessage_Body{Text: proto.String("Escolha uma opção")},
				},
			},
		},
	}

	messageType, raw, _ := s.parseWAMessage(m)

	if messageType != "interactiveMessage" {
		t.Fatalf("expected interactiveMessage, got %q", messageType)
	}
	if raw.InteractiveMessage.Body != "Escolha uma opção" {
		t.Fatalf("expected body from delegated InteractiveMessage, got %q", raw.InteractiveMessage.Body)
	}
}

func TestParseWAMessageTemplateMessageNonHydratedFallsBackToUnknown(t *testing.T) {
	s := &Whatsmiau{}
	m := &waE2E.Message{
		TemplateMessage: &waE2E.TemplateMessage{
			TemplateID: proto.String("template-123"),
			Format: &waE2E.TemplateMessage_FourRowTemplate_{
				FourRowTemplate: &waE2E.TemplateMessage_FourRowTemplate{},
			},
		},
	}

	messageType, raw, _ := s.parseWAMessage(m)

	if messageType != "unknown" {
		t.Fatalf("expected unknown (non-hydrated template needs external lookup), got %q", messageType)
	}
	if len(raw.RawUnknown) == 0 {
		t.Fatalf("expected RawUnknown populated as safety net")
	}
	if !strings.Contains(string(raw.RawUnknown), "template-123") {
		t.Fatalf("expected RawUnknown to contain templateID, got %q", string(raw.RawUnknown))
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

func TestParseWAMessagePinInChatMessage(t *testing.T) {
	s := &Whatsmiau{}
	m := &waE2E.Message{
		PinInChatMessage: &waE2E.PinInChatMessage{
			Type:              waE2E.PinInChatMessage_PIN_FOR_ALL.Enum(),
			SenderTimestampMS: proto.Int64(1787597976949),
			Key: &waCommon.MessageKey{
				ID:        proto.String("3EB0CD61BAEB5A41055352"),
				FromMe:    proto.Bool(false),
				RemoteJID: proto.String("556581316973@s.whatsapp.net"),
			},
		},
	}

	messageType, raw, _ := s.parseWAMessage(m)

	if messageType != "pinInChatMessage" {
		t.Fatalf("expected pinInChatMessage, got %q", messageType)
	}
	if raw.PinInChatMessage == nil {
		t.Fatalf("expected PinInChatMessage populated, got nil")
	}
	if raw.PinInChatMessage.Type != "PIN_FOR_ALL" {
		t.Fatalf("expected type PIN_FOR_ALL, got %q", raw.PinInChatMessage.Type)
	}
	if raw.PinInChatMessage.Key == nil || raw.PinInChatMessage.Key.Id != "3EB0CD61BAEB5A41055352" {
		t.Fatalf("expected key.id 3EB0CD61BAEB5A41055352, got %+v", raw.PinInChatMessage.Key)
	}
	if len(raw.RawUnknown) != 0 {
		t.Fatalf("expected RawUnknown empty (tipo agora conhecido), got %q", string(raw.RawUnknown))
	}
}
