package llm

import (
	"context"
	"encoding/json"
	"net/http"
	"sort"
	"strings"
)

// openai speaks Chat Completions (OpenAI, Gemini's compatible endpoint, Ollama).
type openai struct {
	p     Provedor
	chave string
	http  *http.Client
}

func (o *openai) cabecalhos() map[string]string {
	if o.chave == "" {
		return nil
	}
	return map[string]string{"Authorization": "Bearer " + o.chave}
}

type chamadaOpenAI struct {
	ID       string `json:"id"`
	Type     string `json:"type"`
	Function struct {
		Name      string `json:"name"`
		Arguments string `json:"arguments"`
	} `json:"function"`
}

type msgOpenAI struct {
	Role       string          `json:"role"`
	Content    *string         `json:"content"`
	ToolCalls  []chamadaOpenAI `json:"tool_calls,omitempty"`
	ToolCallID string          `json:"tool_call_id,omitempty"`
}

func texto(s string) *string { return &s }

func (o *openai) Responder(ctx context.Context, modelo, sistema string, msgs []Mensagem, ferramentas []Ferramenta) (Resposta, error) {
	conv := []msgOpenAI{{Role: "system", Content: texto(sistema)}}
	for _, m := range msgs {
		switch m.Papel {
		case "user":
			conv = append(conv, msgOpenAI{Role: "user", Content: texto(m.Texto)})
		case "assistant":
			mo := msgOpenAI{Role: "assistant"}
			if m.Texto != "" || len(m.Chamadas) == 0 {
				mo.Content = texto(m.Texto)
			}
			for _, c := range m.Chamadas {
				co := chamadaOpenAI{ID: c.ID, Type: "function"}
				co.Function.Name, co.Function.Arguments = c.Nome, string(objeto(c.Argumentos))
				mo.ToolCalls = append(mo.ToolCalls, co)
			}
			conv = append(conv, mo)
		case "tool":
			t := m.Texto
			if m.Erro {
				t = "ERRO: " + t
			}
			conv = append(conv, msgOpenAI{Role: "tool", Content: texto(t), ToolCallID: m.Resultado})
		}
	}
	tools := make([]map[string]any, len(ferramentas))
	for i, f := range ferramentas {
		tools[i] = map[string]any{"type": "function", "function": map[string]any{
			"name": f.Nome, "description": f.Descricao, "parameters": simplificar(f.Esquema)}}
	}
	var res struct {
		Choices []struct {
			Message struct {
				Content   string          `json:"content"`
				ToolCalls []chamadaOpenAI `json:"tool_calls"`
			} `json:"message"`
		} `json:"choices"`
	}
	corpo := map[string]any{"model": modelo, "messages": conv, "tools": tools}
	if err := postJSON(ctx, o.http, o.p.Base+"/chat/completions", o.cabecalhos(), corpo, &res, o.p.Nome); err != nil {
		return Resposta{}, err
	}
	var r Resposta
	if len(res.Choices) == 0 {
		return r, nil
	}
	msg := res.Choices[0].Message
	r.Texto = msg.Content
	for i, c := range msg.ToolCalls {
		id := c.ID
		if id == "" { // Ollama and Gemini may omit it; the reply must still point at the call
			id = c.Function.Name + "_" + string(rune('a'+i))
		}
		r.Chamadas = append(r.Chamadas, Chamada{id, c.Function.Name, json.RawMessage(objeto(json.RawMessage(c.Function.Arguments)))})
	}
	return r, nil
}

func (o *openai) Modelos(ctx context.Context) ([]string, error) {
	var res struct {
		Data []struct {
			ID string `json:"id"`
		} `json:"data"`
	}
	if err := getJSON(ctx, o.http, o.p.Base+"/models", o.cabecalhos(), &res, o.p.Nome); err != nil {
		return nil, err
	}
	var ids []string
	for _, m := range res.Data {
		id := strings.TrimPrefix(m.ID, "models/") // Gemini lists "models/gemini-..."
		if o.p.filtro(id) {
			ids = append(ids, id)
		}
	}
	sort.Strings(ids)
	return ids, nil
}

// simplificar reduces a JSON Schema to the subset every Chat Completions
// implementation accepts: no additionalProperties/$schema, and a single type
// instead of ["null", "x"] (Gemini rejects both).
func simplificar(esquema json.RawMessage) any {
	var v any
	if json.Unmarshal(esquema, &v) != nil {
		return map[string]any{"type": "object"}
	}
	var limpar func(any) any
	limpar = func(v any) any {
		switch x := v.(type) {
		case map[string]any:
			delete(x, "additionalProperties")
			delete(x, "$schema")
			if ts, ok := x["type"].([]any); ok {
				for _, t := range ts {
					if t != "null" {
						x["type"] = t
						break
					}
				}
			}
			for k, sub := range x {
				x[k] = limpar(sub)
			}
		case []any:
			for i := range x {
				x[i] = limpar(x[i])
			}
		}
		return v
	}
	return limpar(v)
}
