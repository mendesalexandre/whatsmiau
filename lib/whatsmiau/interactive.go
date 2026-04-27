package whatsmiau

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"strings"
	"time"

	"go.mau.fi/whatsmeow"
	waBinary "go.mau.fi/whatsmeow/binary"
	"go.mau.fi/whatsmeow/proto/waE2E"
	"go.mau.fi/whatsmeow/types"
	"google.golang.org/protobuf/proto"
)

// SendInteractiveCopyCodeButton represents one cta_copy button. The end user
// taps it on WhatsApp and the CopyCode is copied to the clipboard.
type SendInteractiveCopyCodeButton struct {
	DisplayText string `json:"display_text"`
	CopyCode    string `json:"copy_code"`
}

// SendInteractiveCopyCodeRequest builds an InteractiveMessage with an
// optional document/image header, a body text, an optional footer and one or
// more cta_copy buttons (the "Copy code" pill that ships with WhatsApp
// Business templates).
type SendInteractiveCopyCodeRequest struct {
	InstanceID string     `json:"instance_id"`
	RemoteJID  *types.JID `json:"remote_jid"`

	// Header (optional). Type accepts "document", "image" or "" (no media).
	HeaderType     string `json:"header_type"`
	HeaderMediaURL string `json:"header_media_url"`
	HeaderMimetype string `json:"header_mimetype"`
	HeaderFileName string `json:"header_file_name"`
	HeaderTitle    string `json:"header_title"`
	HeaderSubtitle string `json:"header_subtitle"`

	Body   string `json:"body"`
	Footer string `json:"footer"`

	Buttons []SendInteractiveCopyCodeButton `json:"buttons"`
}

type SendInteractiveCopyCodeResponse struct {
	ID        string    `json:"id"`
	CreatedAt time.Time `json:"created_at"`
}

func (s *Whatsmiau) SendInteractiveCopyCode(ctx context.Context, data *SendInteractiveCopyCodeRequest) (*SendInteractiveCopyCodeResponse, error) {
	client, ok := s.clients.Load(data.InstanceID)
	if !ok {
		return nil, whatsmeow.ErrClientIsNil
	}

	if data.RemoteJID == nil {
		return nil, fmt.Errorf("remote_jid is required")
	}

	if strings.TrimSpace(data.Body) == "" {
		return nil, fmt.Errorf("body is required")
	}

	if len(data.Buttons) == 0 {
		return nil, fmt.Errorf("at least one button is required")
	}

	resolved := s.resolveJID(ctx, client, *data.RemoteJID)
	data.RemoteJID = &resolved

	// Build cta_copy buttons. WhatsApp expects each button as a NativeFlowButton
	// with name="cta_copy" and a JSON-encoded buttonParamsJson with
	// {display_text, copy_code}.
	protoButtons := make([]*waE2E.InteractiveMessage_NativeFlowMessage_NativeFlowButton, 0, len(data.Buttons))
	for _, b := range data.Buttons {
		display := strings.TrimSpace(b.DisplayText)
		code := strings.TrimSpace(b.CopyCode)
		if display == "" || code == "" {
			continue
		}
		params := map[string]interface{}{
			"display_text": display,
			"copy_code":    code,
		}
		paramsJSON, err := json.Marshal(params)
		if err != nil {
			return nil, fmt.Errorf("marshal cta_copy params: %w", err)
		}
		protoButtons = append(protoButtons, &waE2E.InteractiveMessage_NativeFlowMessage_NativeFlowButton{
			Name:             proto.String("cta_copy"),
			ButtonParamsJSON: proto.String(string(paramsJSON)),
		})
	}
	if len(protoButtons) == 0 {
		return nil, fmt.Errorf("no valid cta_copy buttons")
	}

	// Optional media header — uploaded so WhatsApp can render it inline.
	var header *waE2E.InteractiveMessage_Header
	headerType := strings.ToLower(strings.TrimSpace(data.HeaderType))
	hasMedia := false
	if headerType != "" && headerType != "none" && data.HeaderMediaURL != "" {
		resMedia, err := s.getCtx(ctx, data.HeaderMediaURL)
		if err != nil {
			return nil, fmt.Errorf("fetch header media: %w", err)
		}
		dataBytes, err := io.ReadAll(resMedia.Body)
		if err != nil {
			return nil, fmt.Errorf("read header media: %w", err)
		}

		var mediaType whatsmeow.MediaType
		switch headerType {
		case "document":
			mediaType = whatsmeow.MediaDocument
		case "image":
			mediaType = whatsmeow.MediaImage
		default:
			return nil, fmt.Errorf("header_type must be 'document' or 'image'")
		}

		uploaded, err := client.Upload(ctx, dataBytes, mediaType)
		if err != nil {
			return nil, fmt.Errorf("upload header media: %w", err)
		}

		header = &waE2E.InteractiveMessage_Header{
			HasMediaAttachment: proto.Bool(true),
		}
		if data.HeaderTitle != "" {
			header.Title = proto.String(data.HeaderTitle)
		}
		if data.HeaderSubtitle != "" {
			header.Subtitle = proto.String(data.HeaderSubtitle)
		}

		switch headerType {
		case "document":
			doc := &waE2E.DocumentMessage{
				URL:           proto.String(uploaded.URL),
				Mimetype:      proto.String(data.HeaderMimetype),
				FileSHA256:    uploaded.FileSHA256,
				FileLength:    proto.Uint64(uploaded.FileLength),
				MediaKey:      uploaded.MediaKey,
				FileEncSHA256: uploaded.FileEncSHA256,
				DirectPath:    proto.String(uploaded.DirectPath),
			}
			if data.HeaderFileName != "" {
				doc.FileName = proto.String(data.HeaderFileName)
			}
			header.Media = &waE2E.InteractiveMessage_Header_DocumentMessage{
				DocumentMessage: doc,
			}
		case "image":
			img := &waE2E.ImageMessage{
				URL:           proto.String(uploaded.URL),
				Mimetype:      proto.String(data.HeaderMimetype),
				FileSHA256:    uploaded.FileSHA256,
				FileLength:    proto.Uint64(uploaded.FileLength),
				MediaKey:      uploaded.MediaKey,
				FileEncSHA256: uploaded.FileEncSHA256,
				DirectPath:    proto.String(uploaded.DirectPath),
			}
			header.Media = &waE2E.InteractiveMessage_Header_ImageMessage{
				ImageMessage: img,
			}
		}
		hasMedia = true
	} else if data.HeaderTitle != "" || data.HeaderSubtitle != "" {
		// Header sem mídia — só título/subtítulo
		header = &waE2E.InteractiveMessage_Header{
			HasMediaAttachment: proto.Bool(false),
		}
		if data.HeaderTitle != "" {
			header.Title = proto.String(data.HeaderTitle)
		}
		if data.HeaderSubtitle != "" {
			header.Subtitle = proto.String(data.HeaderSubtitle)
		}
	}

	interactiveMsg := &waE2E.InteractiveMessage{
		Body: &waE2E.InteractiveMessage_Body{
			Text: proto.String(data.Body),
		},
		InteractiveMessage: &waE2E.InteractiveMessage_NativeFlowMessage_{
			NativeFlowMessage: &waE2E.InteractiveMessage_NativeFlowMessage{
				MessageVersion: proto.Int32(1),
				Buttons:        protoButtons,
			},
		},
	}
	if header != nil {
		interactiveMsg.Header = header
	}
	if data.Footer != "" {
		interactiveMsg.Footer = &waE2E.InteractiveMessage_Footer{
			Text: proto.String(data.Footer),
		}
	}

	// Send as a top-level InteractiveMessage (same approach as SendPixPayment —
	// no FutureProofMessage wrapper). The biz extra-node tells WhatsApp this is
	// a native_flow with mixed button types, which is what cta_copy expects.
	message := &waE2E.Message{InteractiveMessage: interactiveMsg}

	extraNodes := []waBinary.Node{{
		Tag: "biz",
		Content: []waBinary.Node{{
			Tag: "interactive",
			Attrs: waBinary.Attrs{
				"type": "native_flow",
				"v":    "1",
			},
			Content: []waBinary.Node{{
				Tag: "native_flow",
				Attrs: waBinary.Attrs{
					"v":    "9",
					"name": "mixed",
				},
			}},
		}},
	}}

	res, err := client.SendMessage(ctx, *data.RemoteJID, message, whatsmeow.SendRequestExtra{
		AdditionalNodes: &extraNodes,
	})
	if err != nil {
		return nil, err
	}

	_ = hasMedia // reserved for future biz-node tweaks if WhatsApp requires media-aware nodes

	return &SendInteractiveCopyCodeResponse{
		ID:        res.ID,
		CreatedAt: res.Timestamp,
	}, nil
}
