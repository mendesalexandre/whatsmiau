package dto

type ReadMessagesRequest struct {
	InstanceID   string                    `param:"instance" validate:"required" swaggerignore:"true"`
	ReadMessages []ReadMessagesRequestItem `json:"readMessages" validate:"required,min=1"`
}
type ReadMessagesRequestItem struct {
	RemoteJid string `json:"remoteJid" validate:"required"`
	//FromMe    bool   `json:"fromMe"` ignored
	Sender string `json:"sender"` // required if group
	ID     string `json:"id" validate:"required"`
}

type SendPresenceRequestPresence string

const (
	PresenceComposing SendPresenceRequestPresence = "composing"
	PresenceAvailable SendPresenceRequestPresence = "available"
)

type SendPresenceRequestType string

const (
	PresenceTypeText  SendPresenceRequestType = "text"
	PresenceTypeAudio SendPresenceRequestType = "audio"
)

type SendChatPresenceRequest struct {
	InstanceID string                      `param:"instance" validate:"required" swaggerignore:"true"`
	Number     string                      `json:"number"`
	Delay      int                         `json:"delay,omitempty" validate:"omitempty,min=0,max=300000"`
	Presence   SendPresenceRequestPresence `json:"presence"`
	Type       SendPresenceRequestType     `json:"type"`
}

type SendChatPresenceResponse struct {
	Presence SendPresenceRequestPresence `json:"presence"`
}

type NumberExistsRequest struct {
	Numbers []string `json:"numbers"     validate:"required,min=1,dive,required"`
}

// FetchProfilePictureUrlRequest mirrors the Evolution API v2 payload for
// POST /chat/fetchProfilePictureUrl/{instance}.
type FetchProfilePictureUrlRequest struct {
	Number string `json:"number" validate:"required"`
}

// FetchProfilePictureUrlResponse mirrors Evolution's response shape so the
// same client code works unchanged against both backends.
type FetchProfilePictureUrlResponse struct {
	Wuid              string `json:"wuid"`
	ProfilePictureURL string `json:"profilePictureUrl"`
}

// FetchProfileRequest mirrors the Evolution API v2 payload for
// POST /chat/fetchProfile/{instance}.
type FetchProfileRequest struct {
	Number string `json:"number" validate:"required"`
}

// FetchProfileResponse mirrors Evolution's response shape so the same
// client code works unchanged against both backends.
type FetchProfileResponse struct {
	Wuid         string `json:"wuid"`
	Name         string `json:"name"`
	PushName     string `json:"pushName"`
	BusinessName string `json:"businessName"`
	IsBusiness   bool   `json:"isBusiness"`
}

// DeleteMessageForEveryoneRequest — payload pra apagar (revoke) uma mensagem
// no WhatsApp pra todos os participantes. Aceita o formato Evolution API:
// { key: { remoteJid, fromMe, id } }.
type DeleteMessageForEveryoneRequest struct {
	InstanceID string                             `param:"instance" validate:"required" swaggerignore:"true"`
	Key        DeleteMessageForEveryoneRequestKey `json:"key" validate:"required"`
}

type DeleteMessageForEveryoneRequestKey struct {
	RemoteJid string `json:"remoteJid" validate:"required"`
	FromMe    bool   `json:"fromMe"`
	ID        string `json:"id" validate:"required"`
}

// BlockContactRequest — payload pra bloquear/desbloquear um contato na
// conta WhatsApp conectada. Number aceita formato solto (com ou sem
// @s.whatsapp.net) — mesma convenção de outros endpoints de chat.
type BlockContactRequest struct {
	InstanceID string `param:"instance" validate:"required" swaggerignore:"true"`
	Number     string `json:"number" validate:"required"`
	Unblock    bool   `json:"unblock"`
}

// EditMessageRequest — payload pra editar uma mensagem já enviada.
// WhatsApp permite editar até ~15min após envio (enforced server-side).
// V1 só aceita texto (Conversation).
type EditMessageRequest struct {
	InstanceID string                `param:"instance" validate:"required" swaggerignore:"true"`
	Key        EditMessageRequestKey `json:"key" validate:"required"`
	Message    EditMessageContent    `json:"message" validate:"required"`
}

type EditMessageRequestKey struct {
	RemoteJid string `json:"remoteJid" validate:"required"`
	FromMe    bool   `json:"fromMe"`
	ID        string `json:"id" validate:"required"`
}

type EditMessageContent struct {
	Conversation string `json:"conversation" validate:"required"`
}

// UpdatePrivacySettingsRequest mirrors Evolution's
// POST /chat/updatePrivacySettings/{instance}. Each field is optional —
// only fields explicitly set are applied. Empty string = unchanged.
//
// Valid values per field (whatsmeow types/user.go):
//
//	readreceipts: "all" | "none"
//	profile:      "all" | "contacts" | "contact_blacklist" | "none"
//	groupadd:     "all" | "contacts" | "contact_blacklist"
//	last:         "all" | "contacts" | "contact_blacklist" | "none"
//	status:       "all" | "contacts" | "contact_blacklist" | "none"
//	online:       "all" | "match_last_seen"
//	calladd:      "all" | "known"
type UpdatePrivacySettingsRequest struct {
	InstanceID   string `param:"instance" validate:"required" swaggerignore:"true"`
	ReadReceipts string `json:"readreceipts,omitempty" validate:"omitempty,oneof=all none"`
	Profile      string `json:"profile,omitempty" validate:"omitempty,oneof=all contacts contact_blacklist none"`
	GroupAdd     string `json:"groupadd,omitempty" validate:"omitempty,oneof=all contacts contact_blacklist"`
	Last         string `json:"last,omitempty" validate:"omitempty,oneof=all contacts contact_blacklist none"`
	Status       string `json:"status,omitempty" validate:"omitempty,oneof=all contacts contact_blacklist none"`
	Online       string `json:"online,omitempty" validate:"omitempty,oneof=all match_last_seen"`
	CallAdd      string `json:"calladd,omitempty" validate:"omitempty,oneof=all known"`
}

type UpdatePrivacySettingsResponse struct {
	Applied map[string]string `json:"applied"`
	Errors  map[string]string `json:"errors,omitempty"`
}
