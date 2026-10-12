package llm

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

var conversa = []Mensagem{
	{Papel: "user", Texto: "crie Cliente"},
	{Papel: "assistant", Texto: "Vou criar.", Chamadas: []Chamada{{ID: "c1", Nome: "criar_entidade", Argumentos: json.RawMessage(`{"nome":"Cliente"}`)}}},
	{Papel: "tool", Resultado: "c1", Texto: `{"id":3}`},
}

var ferramentas = []Ferramenta{{Nome: "criar_entidade", Descricao: "cria", Esquema: json.RawMessage(
	`{"type":"object","additionalProperties":false,"properties":{"nome":{"type":"string"},"x":{"type":["null","number"]}},"required":["nome"]}`)}}

// servidor records the request and answers with resposta.
func servidor(t *testing.T, caminho, resposta string, pedido *map[string]any, cab *http.Header) *httptest.Server {
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != caminho {
			t.Errorf("path %s, want %s", r.URL.Path, caminho)
		}
		*cab = r.Header.Clone()
		b, _ := io.ReadAll(r.Body)
		json.Unmarshal(b, pedido)
		io.WriteString(w, resposta)
	}))
}

func TestAnthropic(t *testing.T) {
	var pedido map[string]any
	var cab http.Header
	srv := servidor(t, "/messages", `{"content":[{"type":"text","text":"Pronto. "},{"type":"tool_use","id":"t2","name":"adicionar_atributo","input":{"dono":"Cliente"}}],"stop_reason":"tool_use"}`, &pedido, &cab)
	defer srv.Close()
	p := Provedores[0]
	p.Base = srv.URL
	r, err := p.Conectar("sk-ant-teste").Responder(context.Background(), "claude-sonnet-5-5", "sistema", conversa, ferramentas)
	if err != nil {
		t.Fatal(err)
	}
	if cab.Get("x-api-key") != "sk-ant-teste" || cab.Get("anthropic-version") == "" {
		t.Errorf("headers %v", cab)
	}
	b, _ := json.Marshal(pedido)
	for _, want := range []string{`"system":"sistema"`, `"type":"tool_use"`, `"type":"tool_result"`, `"tool_use_id":"c1"`, `"input_schema"`} {
		if !strings.Contains(string(b), want) {
			t.Errorf("request missing %s: %s", want, b)
		}
	}
	if r.Texto != "Pronto. " || len(r.Chamadas) != 1 || r.Chamadas[0].Nome != "adicionar_atributo" || string(r.Chamadas[0].Argumentos) != `{"dono":"Cliente"}` {
		t.Fatalf("%+v", r)
	}
}

func TestOpenAICompativel(t *testing.T) {
	var pedido map[string]any
	var cab http.Header
	srv := servidor(t, "/chat/completions", `{"choices":[{"message":{"content":null,"tool_calls":[{"type":"function","function":{"name":"criar_entidade","arguments":"{\"nome\":\"Pedido\"}"}}]}}]}`, &pedido, &cab)
	defer srv.Close()
	p, _ := ProvedorPorNome("Gemini")
	p.Base = srv.URL
	r, err := p.Conectar("AIza-teste").Responder(context.Background(), "gemini-2.5-pro", "sistema", conversa, ferramentas)
	if err != nil {
		t.Fatal(err)
	}
	if cab.Get("Authorization") != "Bearer AIza-teste" {
		t.Errorf("headers %v", cab)
	}
	b, _ := json.Marshal(pedido)
	for _, want := range []string{`"role":"system"`, `"role":"tool"`, `"tool_call_id":"c1"`, `"arguments":"{\"nome\":\"Cliente\"}"`, `"x":{"type":"number"}`} {
		if !strings.Contains(string(b), want) {
			t.Errorf("request missing %s: %s", want, b)
		}
	}
	if strings.Contains(string(b), "additionalProperties") {
		t.Error("schema must be simplified for Gemini")
	}
	// a call without an ID (Gemini/Ollama) still gets one, so its result can point back at it
	if len(r.Chamadas) != 1 || r.Chamadas[0].ID == "" || r.Chamadas[0].Nome != "criar_entidade" {
		t.Fatalf("%+v", r)
	}
}

func TestModelosEErros(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") == "Bearer errada" {
			w.WriteHeader(401)
			io.WriteString(w, `{"error":{"message":"Incorrect API key provided"}}`)
			return
		}
		io.WriteString(w, `{"data":[{"id":"gpt-5"},{"id":"text-embedding-3-small"},{"id":"gpt-4o-audio-preview"},{"id":"o3"},{"id":"dall-e-3"}]}`)
	}))
	defer srv.Close()
	p, _ := ProvedorPorNome("GPT")
	p.Base = srv.URL
	ms, err := p.Conectar("boa").Modelos(context.Background())
	if err != nil || strings.Join(ms, ",") != "gpt-5,o3" {
		t.Fatalf("%v %v", ms, err)
	}
	if p.Padrao(ms) != "gpt-5" {
		t.Error(p.Padrao(ms))
	}
	_, err = p.Conectar("errada").Modelos(context.Background())
	if err == nil || !strings.Contains(err.Error(), "recusou a chave") || !strings.Contains(err.Error(), "Incorrect API key") {
		t.Fatal(err)
	}
	// Ollama not running: a message that says what to do
	o, _ := ProvedorPorNome("Ollama")
	o.Base = "http://localhost:1/v1"
	if _, err := o.Conectar("").Modelos(context.Background()); err == nil || !strings.Contains(err.Error(), "abra o Ollama") {
		t.Fatal(err)
	}
}
