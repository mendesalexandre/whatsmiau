package dto

// --- sendMedia (extended to support video) ---
// SendMediaRequest already exists in message.go and accepts mediatype.
// Mediatype values: "image" | "document" | "video"

// --- sendPtv (round video note) ---

type SendPtvRequest struct {
	InstanceID       string                `param:"instance" swaggerignore:"true"`
	Number           string                `json:"number,omitempty" validate:"required"`
	Video            string                `json:"video,omitempty" validate:"required"`
	Delay            int                   `json:"delay,omitempty" validate:"omitempty,min=0,max=300000"`
	Quoted           *MessageRequestQuoted `json:"quoted,omitempty"`
	MentionsEveryOne bool                  `json:"mentionsEveryOne,omitempty"`
	Mentioned        []string              `json:"mentioned,omitempty"`
}

type SendPtvResponse struct {
	Key              MessageResponseKey `json:"key"`
	Status           string             `json:"status"`
	MessageType      string             `json:"messageType"`
	MessageTimestamp int                `json:"messageTimestamp"`
	InstanceId       string             `json:"instanceId"`
}

// --- sendSticker ---

type SendStickerRequest struct {
	InstanceID        string                `param:"instance" swaggerignore:"true"`
	Number            string                `json:"number,omitempty" validate:"required"`
	Sticker           string                `json:"sticker,omitempty" validate:"required"`
	Delay             int                   `json:"delay,omitempty" validate:"omitempty,min=0,max=300000"`
	Quoted            *MessageRequestQuoted `json:"quoted,omitempty"`
	MentionsEveryOne  bool                  `json:"mentionsEveryOne,omitempty"`
	Mentioned         []string              `json:"mentioned,omitempty"`
	NotConvertSticker bool                  `json:"notConvertSticker,omitempty"`
}

type SendStickerResponse struct {
	Key              MessageResponseKey `json:"key"`
	Status           string             `json:"status"`
	MessageType      string             `json:"messageType"`
	MessageTimestamp int                `json:"messageTimestamp"`
	InstanceId       string             `json:"instanceId"`
}

// sendLocation já existe em message.go (implementação anterior ao upstream
// "all-message-types") — mantida como está pra não quebrar o contrato que o
// CartZap já usa em produção. Duplicata do upstream removida daqui.

// --- sendContact ---

type SendContactItem struct {
	FullName     string `json:"fullName,omitempty" validate:"required"`
	Wuid         string `json:"wuid,omitempty"`
	PhoneNumber  string `json:"phoneNumber,omitempty" validate:"required"`
	Organization string `json:"organization,omitempty"`
	Email        string `json:"email,omitempty"`
	URL          string `json:"url,omitempty"`
}

type SendContactRequest struct {
	InstanceID       string                `param:"instance" swaggerignore:"true"`
	Number           string                `json:"number,omitempty" validate:"required"`
	Contact          []SendContactItem     `json:"contact,omitempty" validate:"required,min=1,dive"`
	Delay            int                   `json:"delay,omitempty" validate:"omitempty,min=0,max=300000"`
	Quoted           *MessageRequestQuoted `json:"quoted,omitempty"`
	MentionsEveryOne bool                  `json:"mentionsEveryOne,omitempty"`
	Mentioned        []string              `json:"mentioned,omitempty"`
}

type SendContactResponse struct {
	Key              MessageResponseKey `json:"key"`
	Status           string             `json:"status"`
	MessageType      string             `json:"messageType"`
	MessageTimestamp int                `json:"messageTimestamp"`
	InstanceId       string             `json:"instanceId"`
}

// --- sendPoll ---

type SendPollRequest struct {
	InstanceID       string                `param:"instance" swaggerignore:"true"`
	Number           string                `json:"number,omitempty" validate:"required"`
	Name             string                `json:"name,omitempty" validate:"required"`
	SelectableCount  int                   `json:"selectableCount,omitempty" validate:"min=0,max=10"`
	Values           []string              `json:"values,omitempty" validate:"required,min=2,max=10"`
	Delay            int                   `json:"delay,omitempty" validate:"omitempty,min=0,max=300000"`
	Quoted           *MessageRequestQuoted `json:"quoted,omitempty"`
	MentionsEveryOne bool                  `json:"mentionsEveryOne,omitempty"`
	Mentioned        []string              `json:"mentioned,omitempty"`
}

type SendPollResponse struct {
	Key              MessageResponseKey `json:"key"`
	Status           string             `json:"status"`
	MessageType      string             `json:"messageType"`
	MessageTimestamp int                `json:"messageTimestamp"`
	InstanceId       string             `json:"instanceId"`
}

// --- sendStatus ---

type SendStatusRequest struct {
	InstanceID      string   `param:"instance" swaggerignore:"true"`
	Type            string   `json:"type,omitempty" validate:"required,oneof=text image audio video"`
	Content         string   `json:"content,omitempty" validate:"required"`
	Caption         string   `json:"caption,omitempty"`
	BackgroundColor string   `json:"backgroundColor,omitempty"`
	Font            int      `json:"font,omitempty" validate:"omitempty,min=0,max=5"`
	StatusJidList   []string `json:"statusJidList,omitempty"`
	AllContacts     bool     `json:"allContacts,omitempty"`
}

type SendStatusResponse struct {
	Key              MessageResponseKey `json:"key"`
	Status           string             `json:"status"`
	MessageType      string             `json:"messageType"`
	MessageTimestamp int                `json:"messageTimestamp"`
	InstanceId       string             `json:"instanceId"`
}
