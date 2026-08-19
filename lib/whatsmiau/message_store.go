package whatsmiau

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"time"

	"go.uber.org/zap"
)

// messageStore persiste uma cópia rasa de toda mensagem que passa pelo
// WhatsMiau (recebida ou enviada, ao vivo ou via history sync), pra dar
// suporte aos endpoints Evolution-compat /chat/findChats e
// /chat/findMessages — usados pelo backfill do CartZap (sync:mensagens)
// quando o webhook perde uma janela de mensagens (queda de rede, config
// errada, etc) e precisa reconstruir o que passou sem depender de
// re-parear a sessão via QR.
//
// Deliberadamente simples: uma tabela só, sem normalizar chat/mensagem em
// tabelas separadas — "chats" é derivado via SELECT DISTINCT remote_jid.
// Sem cleanup automático por enquanto (TODO: reter só os últimos N dias,
// se o volume crescer demais).
type messageStore struct {
	db *sql.DB
}

const createMessageStoreTableSQLPostgres = `
CREATE TABLE IF NOT EXISTS whatsmiau_stored_messages (
	id                BIGSERIAL PRIMARY KEY,
	instance_id       TEXT NOT NULL,
	remote_jid        TEXT NOT NULL,
	message_id        TEXT NOT NULL,
	from_me           BOOLEAN NOT NULL DEFAULT FALSE,
	participant       TEXT,
	push_name         TEXT,
	message_type      TEXT NOT NULL,
	message_timestamp BIGINT,
	message_json      TEXT NOT NULL,
	created_at        TIMESTAMPTZ NOT NULL DEFAULT now(),
	UNIQUE (instance_id, message_id)
);
CREATE INDEX IF NOT EXISTS idx_whatsmiau_stored_messages_chat
	ON whatsmiau_stored_messages (instance_id, remote_jid, message_timestamp DESC);
`

const createMessageStoreTableSQLSQLite = `
CREATE TABLE IF NOT EXISTS whatsmiau_stored_messages (
	id                INTEGER PRIMARY KEY AUTOINCREMENT,
	instance_id       TEXT NOT NULL,
	remote_jid        TEXT NOT NULL,
	message_id        TEXT NOT NULL,
	from_me           BOOLEAN NOT NULL DEFAULT 0,
	participant       TEXT,
	push_name         TEXT,
	message_type      TEXT NOT NULL,
	message_timestamp INTEGER,
	message_json      TEXT NOT NULL,
	created_at        TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
	UNIQUE (instance_id, message_id)
);
CREATE INDEX IF NOT EXISTS idx_whatsmiau_stored_messages_chat
	ON whatsmiau_stored_messages (instance_id, remote_jid, message_timestamp DESC);
`

func newMessageStore(db *sql.DB, dialect string) (*messageStore, error) {
	ddl := createMessageStoreTableSQLPostgres
	if dialect != "postgres" {
		ddl = createMessageStoreTableSQLSQLite
	}

	if _, err := db.Exec(ddl); err != nil {
		return nil, fmt.Errorf("failed to create whatsmiau_stored_messages table: %w", err)
	}

	return &messageStore{db: db}, nil
}

// storedMessage é o formato interno usado pra ler/escrever uma linha —
// já bate 1:1 com o shape "record" que o Evolution-compat /findMessages
// devolve (key/messageTimestamp/messageType/message/pushName).
type storedMessage struct {
	MessageID        string
	RemoteJID        string
	FromMe           bool
	Participant      string
	PushName         string
	MessageType      string
	MessageTimestamp int64
	MessageJSON      json.RawMessage
}

// upsert grava (ou ignora, se já existir) uma mensagem. Best-effort — quem
// chama loga o erro mas nunca deixa isso derrubar o fluxo principal
// (emissão do webhook).
func (s *messageStore) upsert(ctx context.Context, instanceID string, msg storedMessage) error {
	if msg.MessageID == "" || msg.RemoteJID == "" {
		return nil
	}

	query := `
		INSERT INTO whatsmiau_stored_messages
			(instance_id, remote_jid, message_id, from_me, participant, push_name, message_type, message_timestamp, message_json)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9)
		ON CONFLICT (instance_id, message_id) DO NOTHING
	`

	_, err := s.db.ExecContext(ctx, query,
		instanceID, msg.RemoteJID, msg.MessageID, msg.FromMe, msg.Participant, msg.PushName,
		msg.MessageType, msg.MessageTimestamp, string(msg.MessageJSON),
	)
	return err
}

// findChats retorna os JIDs distintos com mensagens armazenadas pra essa
// instância, mais recentes primeiro.
func (s *messageStore) findChats(ctx context.Context, instanceID string, limit int) ([]string, error) {
	if limit <= 0 || limit > 500 {
		limit = 200
	}

	rows, err := s.db.QueryContext(ctx, `
		SELECT remote_jid, MAX(message_timestamp) AS last_ts
		FROM whatsmiau_stored_messages
		WHERE instance_id = $1
		GROUP BY remote_jid
		ORDER BY last_ts DESC
		LIMIT $2
	`, instanceID, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var jids []string
	for rows.Next() {
		var jid string
		var lastTs sql.NullInt64
		if err := rows.Scan(&jid, &lastTs); err != nil {
			return nil, err
		}
		jids = append(jids, jid)
	}
	return jids, rows.Err()
}

// findMessages retorna mensagens armazenadas de um chat, paginadas, mais
// recentes primeiro (mesma convenção do endpoint Evolution original).
func (s *messageStore) findMessages(ctx context.Context, instanceID, remoteJID string, page, limit int) (records []storedMessage, total int, err error) {
	if page <= 0 {
		page = 1
	}
	if limit <= 0 || limit > 200 {
		limit = 50
	}

	if err = s.db.QueryRowContext(ctx, `
		SELECT COUNT(*) FROM whatsmiau_stored_messages
		WHERE instance_id = $1 AND remote_jid = $2
	`, instanceID, remoteJID).Scan(&total); err != nil {
		return nil, 0, err
	}

	offset := (page - 1) * limit
	rows, err := s.db.QueryContext(ctx, `
		SELECT message_id, remote_jid, from_me, COALESCE(participant, ''), COALESCE(push_name, ''),
		       message_type, COALESCE(message_timestamp, 0), message_json
		FROM whatsmiau_stored_messages
		WHERE instance_id = $1 AND remote_jid = $2
		ORDER BY message_timestamp DESC
		LIMIT $3 OFFSET $4
	`, instanceID, remoteJID, limit, offset)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()

	for rows.Next() {
		var m storedMessage
		var messageJSON string
		if err = rows.Scan(&m.MessageID, &m.RemoteJID, &m.FromMe, &m.Participant, &m.PushName,
			&m.MessageType, &m.MessageTimestamp, &messageJSON); err != nil {
			return nil, 0, err
		}
		m.MessageJSON = json.RawMessage(messageJSON)
		records = append(records, m)
	}
	return records, total, rows.Err()
}

// FindChatsRecord/FindMessagesRecord — formato Evolution-compat que o
// CartZap (WhatsMiauService::getChats/getMessages) já sabe interpretar.
// "message" é o WookMessageRaw bruto que também vai no webhook em tempo
// real — mesmo parser dos dois lados.
type FindChatsRecord struct {
	RemoteJid string `json:"remoteJid"`
}

type FindMessagesKey struct {
	Id          string `json:"id"`
	FromMe      bool   `json:"fromMe"`
	RemoteJid   string `json:"remoteJid"`
	Participant string `json:"participant,omitempty"`
}

type FindMessagesRecord struct {
	Key              FindMessagesKey `json:"key"`
	PushName         string          `json:"pushName,omitempty"`
	MessageTimestamp int64           `json:"messageTimestamp"`
	MessageType      string          `json:"messageType"`
	Message          json.RawMessage `json:"message"`
}

type FindMessagesResponse struct {
	Total   int                  `json:"total"`
	Pages   int                  `json:"pages"`
	Records []FindMessagesRecord `json:"records"`
}

// FindChats — endpoint Evolution-compat pra listar chats com mensagens
// armazenadas. Dá suporte ao backfill do CartZap (sync:mensagens).
func (s *Whatsmiau) FindChats(ctx context.Context, instanceID string) ([]FindChatsRecord, error) {
	if s.messageStore == nil {
		return nil, fmt.Errorf("message store not available")
	}

	jids, err := s.messageStore.findChats(ctx, instanceID, 200)
	if err != nil {
		return nil, err
	}

	records := make([]FindChatsRecord, 0, len(jids))
	for _, jid := range jids {
		records = append(records, FindChatsRecord{RemoteJid: jid})
	}
	return records, nil
}

// FindMessages — endpoint Evolution-compat pra listar mensagens de um chat,
// paginado, mais recentes primeiro.
func (s *Whatsmiau) FindMessages(ctx context.Context, instanceID, remoteJID string, page, limit int) (*FindMessagesResponse, error) {
	if s.messageStore == nil {
		return nil, fmt.Errorf("message store not available")
	}

	stored, total, err := s.messageStore.findMessages(ctx, instanceID, remoteJID, page, limit)
	if err != nil {
		return nil, err
	}

	pages := 0
	if limit > 0 {
		pages = (total + limit - 1) / limit
	}

	records := make([]FindMessagesRecord, 0, len(stored))
	for _, m := range stored {
		records = append(records, FindMessagesRecord{
			Key: FindMessagesKey{
				Id:          m.MessageID,
				FromMe:      m.FromMe,
				RemoteJid:   m.RemoteJID,
				Participant: m.Participant,
			},
			PushName:         m.PushName,
			MessageTimestamp: m.MessageTimestamp,
			MessageType:      m.MessageType,
			Message:          m.MessageJSON,
		})
	}

	return &FindMessagesResponse{Total: total, Pages: pages, Records: records}, nil
}

// storeMessageAsync persiste em goroutine própria — nunca bloqueia nem
// falha o caminho principal (emissão do webhook). Falha vira só um log.
func (s *Whatsmiau) storeMessageAsync(instanceID string, data *WookMessageData) {
	if s.messageStore == nil || data == nil || data.Key == nil {
		return
	}

	msgJSON, err := json.Marshal(data.Message)
	if err != nil {
		zap.L().Debug("message store: failed to marshal message", zap.Error(err))
		return
	}

	msg := storedMessage{
		MessageID:        data.Key.Id,
		RemoteJID:        data.Key.RemoteJid,
		FromMe:           data.Key.FromMe,
		Participant:      data.Key.Participant,
		PushName:         data.PushName,
		MessageType:      data.MessageType,
		MessageTimestamp: int64(data.MessageTimestamp),
		MessageJSON:      msgJSON,
	}

	go func() {
		ctx, cancel := context.WithTimeout(context.Background(), time.Second*10)
		defer cancel()
		if err := s.messageStore.upsert(ctx, instanceID, msg); err != nil {
			zap.L().Debug("message store: failed to persist message", zap.String("instance", instanceID), zap.Error(err))
		}
	}()
}
