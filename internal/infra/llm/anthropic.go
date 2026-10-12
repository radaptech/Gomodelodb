package llm

import (
	"context"
	"encoding/json"
	"net/http"
	"sort"
)

// anthropic speaks the Messages API.
type anthropic struct {
	p     Provedor
	chave string
	http  *http.Client
}

func (a *anthropic) cabecalhos() map[string]string {
	return map[string]string{"x-api-key": a.chave, "anthropic-version": "2023-06-01"}
}

type blocoAnthropic struct {
	Type      string          `json:"type"`
	Text      string          `json:"text,omitempty"`
	ID        string          `json:"id,omitempty"`
	Name      string          `json:"name,omitempty"`
	Input     json.RawMessage `json:"input,omitempty"`
	ToolUseID string          `json:"tool_use_id,omitempty"`
	Content   string          `json:"content,omitempty"`
	IsError   bool            `json:"is_error,omitempty"`
}

type msgAnthropic struct {
	Role    string           `json:"role"`
	Content []blocoAnthropic `json:"content"`
}

type ferramentaAnthropic struct {
	Name        string          `json:"name"`
	Description string          `json:"description"`
	InputSchema json.RawMessage `json:"input_schema"`
}

func (a *anthropic) Responder(ctx context.Context, modelo, sistema string, msgs []Mensagem, ferramentas []Ferramenta) (Resposta, error) {
	var conv []msgAnthropic
	for _, m := range msgs {
		switch m.Papel {
		case "user":
			conv = append(conv, msgAnthropic{"user", []blocoAnthropic{{Type: "text", Text: m.Texto}}})
		case "assistant":
			var bs []blocoAnthropic
			if m.Texto != "" {
				bs = append(bs, blocoAnthropic{Type: "text", Text: m.Texto})
			}
			for _, c := range m.Chamadas {
				bs = append(bs, blocoAnthropic{Type: "tool_use", ID: c.ID, Name: c.Nome, Input: objeto(c.Argumentos)})
			}
			conv = append(conv, msgAnthropic{"assistant", bs})
		case "tool": // results go back as a user turn; consecutive ones share it
			b := blocoAnthropic{Type: "tool_result", ToolUseID: m.Resultado, Content: m.Texto, IsError: m.Erro}
			if n := len(conv); n > 0 && conv[n-1].Role == "user" && conv[n-1].Content[0].Type == "tool_result" {
				conv[n-1].Content = append(conv[n-1].Content, b)
			} else {
				conv = append(conv, msgAnthropic{"user", []blocoAnthropic{b}})
			}
		}
	}
	tools := make([]ferramentaAnthropic, len(ferramentas))
	for i, f := range ferramentas {
		tools[i] = ferramentaAnthropic{f.Nome, f.Descricao, f.Esquema}
	}
	corpo := map[string]any{"model": modelo, "max_tokens": 4096, "system": sistema, "messages": conv, "tools": tools}
	var res struct {
		Content []blocoAnthropic `json:"content"`
	}
	if err := postJSON(ctx, a.http, a.p.Base+"/messages", a.cabecalhos(), corpo, &res, a.p.Nome); err != nil {
		return Resposta{}, err
	}
	var r Resposta
	for _, b := range res.Content {
		switch b.Type {
		case "text":
			r.Texto += b.Text
		case "tool_use":
			r.Chamadas = append(r.Chamadas, Chamada{b.ID, b.Name, b.Input})
		}
	}
	return r, nil
}

func (a *anthropic) Modelos(ctx context.Context) ([]string, error) {
	var res struct {
		Data []struct {
			ID string `json:"id"`
		} `json:"data"`
	}
	if err := getJSON(ctx, a.http, a.p.Base+"/models?limit=100", a.cabecalhos(), &res, a.p.Nome); err != nil {
		return nil, err
	}
	var ids []string
	for _, m := range res.Data {
		if a.p.filtro(m.ID) {
			ids = append(ids, m.ID)
		}
	}
	sort.Strings(ids)
	return ids, nil
}

// objeto makes sure tool input is a JSON object ({} for empty arguments).
func objeto(j json.RawMessage) json.RawMessage {
	if len(j) == 0 || string(j) == "null" {
		return json.RawMessage("{}")
	}
	return j
}
