// Package llm talks to language-model providers with tool calling. Two wire
// protocols cover them all: Anthropic's Messages API (Claude) and the OpenAI
// Chat Completions API, which OpenAI, Google (Gemini's OpenAI-compatible
// endpoint) and Ollama all speak.
package llm

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

// Mensagem is one turn of a conversation, in a provider-neutral shape.
type Mensagem struct {
	Papel     string    // "user", "assistant" or "tool"
	Texto     string    // text of user/assistant turns, or the tool result
	Chamadas  []Chamada // tools the assistant asked to run
	Resultado string    // for Papel "tool": the ID of the call this answers
	Erro      bool      // for Papel "tool": the call failed
}

// Chamada is a tool call requested by the model.
type Chamada struct {
	ID         string
	Nome       string
	Argumentos json.RawMessage
}

// Ferramenta is a tool offered to the model, with its JSON Schema.
type Ferramenta struct {
	Nome      string
	Descricao string
	Esquema   json.RawMessage
}

// Resposta is one model turn: text, tool calls, or both.
type Resposta struct {
	Texto    string
	Chamadas []Chamada
}

// Cliente is a provider ready to answer, already authenticated.
type Cliente interface {
	Responder(ctx context.Context, modelo, sistema string, msgs []Mensagem, ferramentas []Ferramenta) (Resposta, error)
	Modelos(ctx context.Context) ([]string, error)
}

// Provedor describes a supported service.
type Provedor struct {
	Nome         string // shown to the user
	anthropic    bool   // Messages API instead of Chat Completions
	Base         string
	PrecisaChave bool
	ChaveURL     string   // where to create a key
	Preferidos   []string // default model: the first available one containing one of these
	filtro       func(id string) bool
}

var Provedores = []Provedor{
	{Nome: "Claude", anthropic: true, Base: "https://api.anthropic.com/v1", PrecisaChave: true,
		ChaveURL: "console.anthropic.com", Preferidos: []string{"claude-sonnet-5-5", "sonnet", "claude"},
		filtro: func(id string) bool { return strings.HasPrefix(id, "claude") }},
	{Nome: "GPT", Base: "https://api.openai.com/v1", PrecisaChave: true,
		ChaveURL: "platform.openai.com", Preferidos: []string{"gpt-5", "gpt"},
		filtro: func(id string) bool {
			if !strings.HasPrefix(id, "gpt") && !(len(id) > 1 && id[0] == 'o' && id[1] >= '0' && id[1] <= '9') {
				return false
			}
			for _, fora := range []string{"audio", "realtime", "tts", "transcribe", "image", "search", "embedding", "instruct"} {
				if strings.Contains(id, fora) {
					return false
				}
			}
			return true
		}},
	{Nome: "Gemini", Base: "https://generativelanguage.googleapis.com/v1beta/openai", PrecisaChave: true,
		ChaveURL: "aistudio.google.com", Preferidos: []string{"gemini-2.5-pro", "pro", "gemini"},
		filtro: func(id string) bool {
			return strings.HasPrefix(id, "gemini") && !strings.Contains(id, "embedding") && !strings.Contains(id, "image") && !strings.Contains(id, "tts")
		}},
	{Nome: "Ollama", Base: "http://localhost:11434/v1", Preferidos: []string{"qwen", "llama"},
		filtro: func(string) bool { return true }},
}

// ProvedorPorNome finds a provider by its display name.
func ProvedorPorNome(nome string) (Provedor, bool) {
	for _, p := range Provedores {
		if p.Nome == nome {
			return p, true
		}
	}
	return Provedor{}, false
}

// Conectar returns a client for p authenticated with chave (ignored by Ollama).
func (p Provedor) Conectar(chave string) Cliente {
	h := &http.Client{Timeout: 3 * time.Minute}
	if p.anthropic {
		return &anthropic{p: p, chave: chave, http: h}
	}
	return &openai{p: p, chave: chave, http: h}
}

// Padrao picks the default model among those available.
func (p Provedor) Padrao(modelos []string) string {
	for _, pref := range p.Preferidos {
		for _, m := range modelos {
			if strings.Contains(m, pref) {
				return m
			}
		}
	}
	if len(modelos) > 0 {
		return modelos[0]
	}
	return ""
}

// erroHTTP turns a provider's error response into a readable message.
func erroHTTP(provedor string, res *http.Response) error {
	corpo, _ := io.ReadAll(io.LimitReader(res.Body, 4096))
	var e struct {
		Error struct {
			Message string `json:"message"`
		} `json:"error"`
	}
	msg := strings.TrimSpace(string(corpo))
	if json.Unmarshal(corpo, &e) == nil && e.Error.Message != "" {
		msg = e.Error.Message
	}
	switch res.StatusCode {
	case http.StatusUnauthorized, http.StatusForbidden:
		return fmt.Errorf("%s recusou a chave de API (%d): %s", provedor, res.StatusCode, msg)
	case http.StatusTooManyRequests:
		return fmt.Errorf("%s: limite de uso atingido, tente de novo em instantes (%s)", provedor, msg)
	}
	return fmt.Errorf("%s respondeu %d: %s", provedor, res.StatusCode, msg)
}

func postJSON(ctx context.Context, h *http.Client, url string, cab map[string]string, corpo, resposta any, provedor string) error {
	b, err := json.Marshal(corpo)
	if err != nil {
		return err
	}
	req, err := http.NewRequestWithContext(ctx, "POST", url, strings.NewReader(string(b)))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	for k, v := range cab {
		req.Header.Set(k, v)
	}
	return fazer(h, req, resposta, provedor)
}

func getJSON(ctx context.Context, h *http.Client, url string, cab map[string]string, resposta any, provedor string) error {
	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return err
	}
	for k, v := range cab {
		req.Header.Set(k, v)
	}
	return fazer(h, req, resposta, provedor)
}

func fazer(h *http.Client, req *http.Request, resposta any, provedor string) error {
	res, err := h.Do(req)
	if err != nil {
		if strings.Contains(err.Error(), "connection refused") && strings.Contains(req.URL.Host, "localhost") {
			return fmt.Errorf("%s não está rodando em %s; abra o Ollama e tente de novo", provedor, req.URL.Host)
		}
		return fmt.Errorf("sem conexão com %s: %w", provedor, err)
	}
	defer res.Body.Close()
	if res.StatusCode/100 != 2 {
		return erroHTTP(provedor, res)
	}
	return json.NewDecoder(res.Body).Decode(resposta)
}
