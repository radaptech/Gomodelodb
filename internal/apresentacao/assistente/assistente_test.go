package assistente

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"testing"

	"github.com/brmodelo/gobrmodelo/internal/aplicacao"
	"github.com/brmodelo/gobrmodelo/internal/infra/llm"
)

// roteiro is a fake model that replays scripted turns and records what it saw.
type roteiro struct {
	turnos  []llm.Resposta
	vistos  [][]llm.Mensagem
	sistema string
	tools   []llm.Ferramenta
	falha   error
}

func (r *roteiro) Responder(_ context.Context, _, sistema string, msgs []llm.Mensagem, f []llm.Ferramenta) (llm.Resposta, error) {
	if r.falha != nil {
		return llm.Resposta{}, r.falha
	}
	r.sistema, r.tools = sistema, f
	r.vistos = append(r.vistos, append([]llm.Mensagem(nil), msgs...))
	t := r.turnos[0]
	r.turnos = r.turnos[1:]
	return t, nil
}

func (r *roteiro) Modelos(context.Context) ([]string, error) { return []string{"fake"}, nil }

func chamada(id, nome, args string) llm.Chamada {
	return llm.Chamada{ID: id, Nome: nome, Argumentos: json.RawMessage(args)}
}

func TestPedidoRodaFerramentas(t *testing.T) {
	s := aplicacao.Nova()
	a, err := Novo(s)
	if err != nil {
		t.Fatal(err)
	}
	m := &roteiro{turnos: []llm.Resposta{
		{Texto: "Vou criar as entidades.", Chamadas: []llm.Chamada{
			chamada("1", "criar_entidade", `{"nome":"Livro"}`),
			chamada("2", "criar_entidade", `{"nome":"Autor"}`),
		}},
		{Chamadas: []llm.Chamada{
			chamada("3", "adicionar_atributo", `{"dono":"Livro","nome":"isbn","tipo":"VARCHAR(13)","identificador":true}`),
			chamada("4", "criar_relacionamento", `{"entidade_a":"Autor","entidade_b":"Livro","nome":"escreve","cardinalidade_a":"(1,n)"}`),
			chamada("5", "adicionar_atributo", `{"dono":"Editora","nome":"x"}`), // fails: no such owner
		}},
		{Texto: "Pronto: Autor escreve Livro, com ISBN como chave."},
	}}
	var eventos []Evento
	if err := a.Enviar(context.Background(), m, "fake", "modele livros e autores", func(e Evento) { eventos = append(eventos, e) }); err != nil {
		t.Fatal(err)
	}

	// the diagram changed through the same tools Claude Code uses
	res := s.Resumo()
	if len(res.Entidades) != 2 || len(res.Relacionamentos) != 1 || res.Entidades[0].Atributos[0].Nome != "isbn" {
		t.Fatalf("%+v", res)
	}
	// the model saw every tool, the MCP instructions, and each result (errors flagged)
	if len(m.tools) != 14 || !strings.Contains(m.sistema, "descrever_diagrama") || !strings.Contains(m.sistema, "português") {
		t.Fatalf("tools=%d sistema=%q", len(m.tools), m.sistema)
	}
	ultimo := m.vistos[2]
	if r := ultimo[len(ultimo)-1]; r.Papel != "tool" || r.Resultado != "5" || !r.Erro || !strings.Contains(r.Texto, "Editora") {
		t.Fatalf("tool error not reported back: %+v", r)
	}
	// the panel got text, each call with a readable summary, and the failure
	var linhas []string
	for _, e := range eventos {
		if e.Ferramenta != "" {
			linhas = append(linhas, e.Ferramenta+" "+e.Resumo)
		}
	}
	if len(eventos) != 7 || linhas[2] != "adicionar_atributo isbn · Livro · VARCHAR(13)" || !eventos[5].Falhou {
		t.Fatalf("%q %+v", linhas, eventos)
	}

	// the conversation continues with history; a provider error leaves it as it was
	n := len(a.historico)
	m.falha = errors.New("Claude recusou a chave de API")
	if err := a.Enviar(context.Background(), m, "fake", "e editoras?", func(Evento) {}); err == nil {
		t.Fatal("provider error must surface")
	}
	if len(a.historico) != n {
		t.Fatal("failed request left a dangling turn in the history")
	}
	a.Limpar()
	if len(a.historico) != 0 {
		t.Fatal("Limpar")
	}
}
