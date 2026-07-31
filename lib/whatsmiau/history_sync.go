package whatsmiau

import (
	"context"
	"fmt"
	"time"

	"go.mau.fi/whatsmeow"
	"go.mau.fi/whatsmeow/types"
)

// RequestHistorySyncData é o pedido de histórico sob demanda de um chat
// específico — diferente do resync completo automático que roda ao
// reconectar. Usa BuildHistorySyncRequest do whatsmeow, que manda uma
// mensagem de controle (peer message) pro próprio dispositivo primário
// pedindo `count` mensagens anteriores à mensagem-âncora informada.
//
// A resposta chega de forma assíncrona como *events.HistorySync (tipo
// ON_DEMAND) e é processada pelo MESMO pipeline de emitHistoryMessages —
// nenhuma mudança necessária lá, ela já trata qualquer HistorySync
// genericamente. Requer que o celular (dispositivo primário) esteja com
// internet/em primeiro plano pra responder.
type RequestHistorySyncData struct {
	InstanceID          string     `json:"instance_id"`
	ChatJID              *types.JID `json:"chat_jid"`
	OldestMsgID          string     `json:"oldest_msg_id"`
	OldestMsgFromMe      bool       `json:"oldest_msg_from_me"`
	OldestMsgTimestamp   int64      `json:"oldest_msg_timestamp"` // unix seconds
	Count                int        `json:"count"`
}

type RequestHistorySyncResponse struct {
	ID        string    `json:"id"`
	CreatedAt time.Time `json:"created_at"`
}

func (s *Whatsmiau) RequestHistorySync(ctx context.Context, data *RequestHistorySyncData) (*RequestHistorySyncResponse, error) {
	client, ok := s.clients.Load(data.InstanceID)
	if !ok {
		return nil, whatsmeow.ErrClientIsNil
	}

	if data.ChatJID == nil {
		return nil, fmt.Errorf("chat_jid is required")
	}
	if data.OldestMsgID == "" {
		return nil, fmt.Errorf("oldest_msg_id is required")
	}
	if data.Count <= 0 || data.Count > 100 {
		return nil, fmt.Errorf("count must be between 1 and 100")
	}

	anchor := &types.MessageInfo{
		MessageSource: types.MessageSource{
			Chat:     *data.ChatJID,
			IsFromMe: data.OldestMsgFromMe,
		},
		ID:        types.MessageID(data.OldestMsgID),
		Timestamp: time.Unix(data.OldestMsgTimestamp, 0),
	}

	msg := client.BuildHistorySyncRequest(anchor, data.Count)

	resp, err := client.SendPeerMessage(ctx, msg)
	if err != nil {
		return nil, err
	}

	return &RequestHistorySyncResponse{
		ID:        resp.ID,
		CreatedAt: resp.Timestamp,
	}, nil
}
