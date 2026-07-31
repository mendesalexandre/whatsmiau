package whatsmiau

import (
	"sync"
	"time"
)

// webhookErrorEntry é um registro de falha PERMANENTE de entrega de webhook
// (depois de esgotar os retries do processEmit). Guardado só em memória —
// não sobrevive a restart do processo, igual ao equivalente do uazapi.
type webhookErrorEntry struct {
	Created    time.Time `json:"created"`
	InstanceID string    `json:"instanceId"`
	URL        string    `json:"url"`
	Event      string    `json:"event"`
	Attempts   int       `json:"attempts"`
	StatusCode int       `json:"statusCode,omitempty"`
	Error      string    `json:"error"`
}

// webhookErrorBuffer mantém os últimos N erros por instância. Buffer
// circular simples protegido por mutex — volume esperado é baixo (só
// registra depois de TODAS as tentativas de retry falharem).
type webhookErrorBuffer struct {
	mu      sync.Mutex
	perInst map[string][]webhookErrorEntry
	max     int
}

func newWebhookErrorBuffer(max int) *webhookErrorBuffer {
	return &webhookErrorBuffer{
		perInst: make(map[string][]webhookErrorEntry),
		max:     max,
	}
}

func (b *webhookErrorBuffer) add(entry webhookErrorEntry) {
	b.mu.Lock()
	defer b.mu.Unlock()

	list := b.perInst[entry.InstanceID]
	// Mais recente primeiro.
	list = append([]webhookErrorEntry{entry}, list...)
	if len(list) > b.max {
		list = list[:b.max]
	}
	b.perInst[entry.InstanceID] = list
}

func (b *webhookErrorBuffer) list(instanceID string) []webhookErrorEntry {
	b.mu.Lock()
	defer b.mu.Unlock()

	list := b.perInst[instanceID]
	out := make([]webhookErrorEntry, len(list))
	copy(out, list)
	return out
}

// WebhookErrors retorna os últimos erros de entrega permanente (depois de
// esgotar os retries) do webhook local dessa instância, mais recente
// primeiro. Só em memória — não sobrevive a restart do processo.
func (s *Whatsmiau) WebhookErrors(instanceID string) []webhookErrorEntry {
	if s.webhookErrors == nil {
		return []webhookErrorEntry{}
	}
	return s.webhookErrors.list(instanceID)
}
