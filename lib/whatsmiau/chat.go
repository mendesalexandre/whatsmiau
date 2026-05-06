package whatsmiau

import (
	"errors"
	"strings"
	"time"

	"go.mau.fi/whatsmeow"
	"go.mau.fi/whatsmeow/proto/waE2E"
	"go.mau.fi/whatsmeow/types"
	"go.uber.org/zap"
	"golang.org/x/net/context"
	"google.golang.org/protobuf/proto"
)

// ErrEmptyNumber is returned when FetchProfilePictureUrl is called with a
// blank number. Exported so the controller can map it to HTTP 400.
var ErrEmptyNumber = errors.New("number is empty")

type ReadMessageRequest struct {
	MessageIDs []string   `json:"message_ids"`
	InstanceID string     `json:"instance_id"`
	RemoteJID  *types.JID `json:"remote_jid"`
	Sender     *types.JID `json:"sender"`
}

func (s *Whatsmiau) ReadMessage(data *ReadMessageRequest) error {
	client, ok := s.clients.Load(data.InstanceID)
	if !ok {
		return whatsmeow.ErrClientIsNil
	}

	sender := *data.RemoteJID
	if data.Sender != nil {
		sender = *data.Sender
	}

	return client.MarkRead(context.TODO(), data.MessageIDs, time.Now(), *data.RemoteJID, sender)
}

type ChatPresenceRequest struct {
	InstanceID string                  `json:"instance_id"`
	RemoteJID  *types.JID              `json:"remote_jid"`
	Presence   types.ChatPresence      `json:"presence"`
	Media      types.ChatPresenceMedia `json:"media"`
}

func (s *Whatsmiau) ChatPresence(data *ChatPresenceRequest) error {
	client, ok := s.clients.Load(data.InstanceID)
	if !ok {
		return whatsmeow.ErrClientIsNil
	}

	return client.SendChatPresence(context.TODO(), *data.RemoteJID, data.Presence, data.Media)
}

type NumberExistsRequest struct {
	InstanceID string   `json:"instance_id"`
	Numbers    []string `json:"numbers"`
}

type NumberExistsResponse []Exists

type Exists struct {
	Exists bool   `json:"exists"`
	Jid    string `json:"jid"`
	Lid    string `json:"lid"`
	Number string `json:"number"`
}

func (s *Whatsmiau) NumberExists(ctx context.Context, data *NumberExistsRequest) (NumberExistsResponse, error) {
	client, ok := s.clients.Load(data.InstanceID)
	if !ok {
		return nil, whatsmeow.ErrClientIsNil
	}

	resp, err := client.IsOnWhatsApp(context.TODO(), data.Numbers)
	if err != nil {
		return nil, err
	}

	var results []Exists
	for _, item := range resp {
		jid, lid := s.GetJidLid(ctx, data.InstanceID, item.JID)

		results = append(results, Exists{
			Exists: item.IsIn,
			Jid:    jid,
			Lid:    lid,
			Number: item.Query,
		})
	}

	return results, nil
}

type FetchProfilePictureUrlRequest struct {
	InstanceID string `json:"instance_id"`
	Number     string `json:"number"`
}

type FetchProfilePictureUrlResponse struct {
	Wuid              string `json:"wuid"`
	ProfilePictureURL string `json:"profilePictureUrl"`
}

// FetchProfilePictureUrl resolves a raw phone number to a WhatsApp JID and
// returns the full-resolution profile picture URL. Mirrors Evolution API's
// POST /chat/fetchProfilePictureUrl/{instance}.
func (s *Whatsmiau) FetchProfilePictureUrl(ctx context.Context, data *FetchProfilePictureUrlRequest) (*FetchProfilePictureUrlResponse, error) {
	client, ok := s.clients.Load(data.InstanceID)
	if !ok {
		return nil, whatsmeow.ErrClientIsNil
	}

	number := strings.TrimSpace(data.Number)
	if number == "" {
		return nil, ErrEmptyNumber
	}

	// Accept already-JID inputs ("5511...@s.whatsapp.net", group JIDs, etc).
	var jid types.JID
	if strings.Contains(number, "@") {
		parsed, err := types.ParseJID(number)
		if err != nil {
			return nil, err
		}
		jid = parsed
	} else {
		digits := strings.Map(func(r rune) rune {
			if r >= '0' && r <= '9' {
				return r
			}
			return -1
		}, number)
		if digits == "" {
			return nil, ErrEmptyNumber
		}
		jid = types.NewJID(digits, types.DefaultUserServer)
		// Tries Brazilian 9th-digit fallback when applicable.
		jid = s.resolveJID(ctx, client, jid)
	}

	pic, err := client.GetProfilePictureInfo(ctx, jid, &whatsmeow.GetProfilePictureParams{
		Preview:     false,
		IsCommunity: false,
	})
	if err != nil {
		// whatsmeow returns ErrProfilePictureUnauthorized / Not404d via error.
		// Keep compat with Evolution: return empty URL instead of 500 when the
		// profile exists but the picture isn't visible.
		zap.L().Debug("FetchProfilePictureUrl: pic lookup failed",
			zap.String("instance", data.InstanceID),
			zap.String("jid", jid.String()),
			zap.Error(err))
	}

	resp := &FetchProfilePictureUrlResponse{Wuid: jid.String()}
	if pic != nil {
		resp.ProfilePictureURL = pic.URL
	}
	return resp, nil
}

// RevokeMessageRequest — apaga uma mensagem pra todos no WhatsApp ("revoke").
// Funciona apenas em mensagens enviadas pelo próprio cliente e dentro da
// janela de ~2 dias do WhatsApp. Quem decide se está dentro da janela é o
// próprio servidor do WhatsApp — aqui só montamos o protocolo.
type RevokeMessageRequest struct {
	InstanceID string     `json:"instance_id"`
	RemoteJID  *types.JID `json:"remote_jid"`
	MessageID  string     `json:"message_id"`
}

func (s *Whatsmiau) RevokeMessage(data *RevokeMessageRequest) error {
	client, ok := s.clients.Load(data.InstanceID)
	if !ok {
		return whatsmeow.ErrClientIsNil
	}

	if client.Store.ID == nil {
		return whatsmeow.ErrNotLoggedIn
	}

	if data.RemoteJID == nil {
		return whatsmeow.ErrUnknownServer
	}

	// BuildRevoke: pra mensagens em chat 1-1, sender deve ser EmptyJID; pra grupos,
	// é o próprio JID do cliente. whatsmeow lida com ambos os casos internamente.
	sender := types.EmptyJID
	if data.RemoteJID.Server == types.GroupServer {
		sender = client.Store.ID.ToNonAD()
	}

	revoke := client.BuildRevoke(*data.RemoteJID, sender, data.MessageID)
	_, err := client.SendMessage(context.TODO(), *data.RemoteJID, revoke)
	return err
}

// EditMessageRequest — edita uma mensagem já enviada. WhatsApp permite
// editar até ~15min após o envio (enforced server-side). Funciona apenas
// em mensagens enviadas pelo próprio cliente. Pra V1 só aceita texto
// (Conversation); legenda de mídia exige outro shape de Message.
type EditMessageRequest struct {
	InstanceID string     `json:"instance_id"`
	RemoteJID  *types.JID `json:"remote_jid"`
	MessageID  string     `json:"message_id"`
	NewText    string     `json:"new_text"`
}

func (s *Whatsmiau) EditMessage(data *EditMessageRequest) error {
	client, ok := s.clients.Load(data.InstanceID)
	if !ok {
		return whatsmeow.ErrClientIsNil
	}

	if client.Store.ID == nil {
		return whatsmeow.ErrNotLoggedIn
	}

	if data.RemoteJID == nil {
		return whatsmeow.ErrUnknownServer
	}

	if strings.TrimSpace(data.NewText) == "" {
		return errors.New("new_text is empty")
	}

	if data.MessageID == "" {
		return errors.New("message_id is empty")
	}

	newContent := &waE2E.Message{
		Conversation: proto.String(data.NewText),
	}

	edited := client.BuildEdit(*data.RemoteJID, data.MessageID, newContent)
	_, err := client.SendMessage(context.TODO(), *data.RemoteJID, edited)
	return err
}

func (s *Whatsmiau) resolveJID(ctx context.Context, client *whatsmeow.Client, jid types.JID) types.JID {
	if jid.Server != types.DefaultUserServer {
		return jid
	}

	alternate := buildBrazilianAlternate(jid.User)
	if alternate == "" {
		return jid
	}

	resp, err := client.IsOnWhatsApp(ctx, []string{jid.User, alternate})
	if err != nil {
		zap.L().Warn("resolveJID: failed to check number on WhatsApp", zap.String("number", jid.User), zap.Error(err))
		return jid
	}

	for _, item := range resp {
		if item.IsIn {
			resolved := jid
			resolved.User = item.JID.User
			if resolved.User != jid.User {
				zap.L().Debug("resolveJID: brazilian number resolved", zap.String("from", jid.User), zap.String("to", resolved.User))
			}
			return resolved
		}
	}

	return jid
}

// UpdatePrivacySettingsRequest changes one or more privacy settings on the
// linked WhatsApp account. Common keys mirror what the WhatsApp app shows
// under Settings > Privacy:
//
//	readreceipts -> "all" or "none"
//	profile      -> "all" | "contacts" | "contact_blacklist" | "none"
//	groupadd     -> "all" | "contacts" | "contact_blacklist"
//	last         -> "all" | "contacts" | "contact_blacklist" | "none"
//	status       -> "all" | "contacts" | "contact_blacklist" | "none"
//	online       -> "all" | "match_last_seen"
//	calladd      -> "all" | "known"
//
// Settings are applied independently and we keep going on partial failure
// so a typo in one field doesn't block the others. Errors are returned for
// observability.
type UpdatePrivacySettingsRequest struct {
	InstanceID   string `json:"instance_id"`
	ReadReceipts string `json:"readreceipts,omitempty"`
	Profile      string `json:"profile,omitempty"`
	GroupAdd     string `json:"groupadd,omitempty"`
	Last         string `json:"last,omitempty"`
	Status       string `json:"status,omitempty"`
	Online       string `json:"online,omitempty"`
	CallAdd      string `json:"calladd,omitempty"`
}

// UpdatePrivacySettingsResult lists which keys actually changed and any
// per-key errors. Callers can decide to surface partial success as 200 or
// downgrade to 4xx based on their semantics.
type UpdatePrivacySettingsResult struct {
	Applied map[string]string `json:"applied"`
	Errors  map[string]string `json:"errors,omitempty"`
}

func (s *Whatsmiau) UpdatePrivacySettings(ctx context.Context, data *UpdatePrivacySettingsRequest) (*UpdatePrivacySettingsResult, error) {
	client, ok := s.clients.Load(data.InstanceID)
	if !ok {
		return nil, whatsmeow.ErrClientIsNil
	}

	if client.Store == nil || client.Store.ID == nil {
		return nil, whatsmeow.ErrNotLoggedIn
	}

	pairs := []struct {
		name  types.PrivacySettingType
		value string
	}{
		{types.PrivacySettingTypeReadReceipts, data.ReadReceipts},
		{types.PrivacySettingTypeProfile, data.Profile},
		{types.PrivacySettingTypeGroupAdd, data.GroupAdd},
		{types.PrivacySettingTypeLastSeen, data.Last},
		{types.PrivacySettingTypeStatus, data.Status},
		{types.PrivacySettingTypeOnline, data.Online},
		{types.PrivacySettingTypeCallAdd, data.CallAdd},
	}

	result := &UpdatePrivacySettingsResult{
		Applied: map[string]string{},
		Errors:  map[string]string{},
	}

	for _, p := range pairs {
		if p.value == "" {
			continue
		}
		_, err := client.SetPrivacySetting(ctx, p.name, types.PrivacySetting(p.value))
		if err != nil {
			result.Errors[string(p.name)] = err.Error()
			continue
		}
		result.Applied[string(p.name)] = p.value
	}

	return result, nil
}
