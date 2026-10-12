// Package assistente is the in-app chat: a language model that edits the open
// diagram. It is an MCP client of the app's own MCP server (connected in
// memory), so it uses exactly the tools Claude Code gets over HTTP.
package assistente

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	sdk "github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/brmodelo/gobrmodelo/internal/aplicacao"
	"github.com/brmodelo/gobrmodelo/internal/apresentacao/mcp"
	"github.com/brmodelo/gobrmodelo/internal/infra/llm"
)

// maxPassos bounds one request: model turns with tool calls before giving up.
const maxPassos = 30

const estilo = `

Você conversa com o usuário dentro do próprio brModelo, num painel ao lado do diagrama. Responda em português do Brasil, em poucas frases: o diagrama na tela já mostra o resultado, então não repita listas de tudo o que criou. Quando o pedido for ambíguo, faça escolhas razoáveis de modelagem e diga quais foram. Antes de alterar um diagrama que já tem conteúdo, use descrever_diagrama para conhecer os IDs.`

// Evento is something the panel shows while a request runs.
type Evento struct {
	Ferramenta string // set for a tool call: its name
	Resumo     string // the call's main arguments, for display
	Falhou     bool   // the tool returned an error
	ID         int    // element the tool created or changed, when it reports one
	Texto      string // assistant text (when Ferramenta is empty)
}

type Assistente struct {
	mcp         *sdk.ClientSession
	ferramentas []llm.Ferramenta
	sistema     string
	historico   []llm.Mensagem
}

// Novo connects an assistant to session s through an in-memory MCP pair.
func Novo(s *aplicacao.Sessao) (*Assistente, error) {
	ctx := context.Background()
	ct, st := sdk.NewInMemoryTransports()
	if _, err := mcp.NovoServidor(s).Connect(ctx, st, nil); err != nil {
		return nil, err
	}
	cs, err := sdk.NewClient(&sdk.Implementation{Name: "brmodelo-painel", Version: "1.0.0"}, nil).Connect(ctx, ct, nil)
	if err != nil {
		return nil, err
	}
	a := &Assistente{mcp: cs, sistema: cs.InitializeResult().Instructions + estilo}
	lista, err := cs.ListTools(ctx, nil)
	if err != nil {
		return nil, err
	}
	for _, t := range lista.Tools {
		esq, _ := json.Marshal(t.InputSchema)
		a.ferramentas = append(a.ferramentas, llm.Ferramenta{Nome: t.Name, Descricao: t.Description, Esquema: esq})
	}
	return a, nil
}

// Limpar forgets the conversation (the diagram stays as it is).
func (a *Assistente) Limpar() { a.historico = nil }

// Enviar sends one request and runs the tool loop until the model answers
// without calling tools. ev is called for every tool call and text, from this
// goroutine; it must hand UI work to the UI thread itself.
func (a *Assistente) Enviar(ctx context.Context, c llm.Cliente, modelo, pedido string, ev func(Evento)) error {
	inicio := len(a.historico)
	a.historico = append(a.historico, llm.Mensagem{Papel: "user", Texto: pedido})
	for range maxPassos {
		r, err := c.Responder(ctx, modelo, a.sistema, a.historico, a.ferramentas)
		if err != nil {
			a.historico = a.historico[:inicio] // drop the unanswered turn so the next request starts clean
			return err
		}
		a.historico = append(a.historico, llm.Mensagem{Papel: "assistant", Texto: r.Texto, Chamadas: r.Chamadas})
		if t := strings.TrimSpace(r.Texto); t != "" {
			ev(Evento{Texto: t})
		}
		if len(r.Chamadas) == 0 {
			return nil
		}
		for _, ch := range r.Chamadas {
			res, falhou := a.executar(ctx, ch)
			var saida struct {
				ID int `json:"id"`
			}
			json.Unmarshal([]byte(res), &saida)
			ev(Evento{Ferramenta: ch.Nome, Resumo: resumir(ch.Argumentos), Falhou: falhou, ID: saida.ID})
			a.historico = append(a.historico, llm.Mensagem{Papel: "tool", Resultado: ch.ID, Texto: res, Erro: falhou})
		}
	}
	return fmt.Errorf("o pedido passou de %d passos sem terminar; tente dividir em partes menores", maxPassos)
}

func (a *Assistente) executar(ctx context.Context, ch llm.Chamada) (string, bool) {
	var args map[string]any
	if err := json.Unmarshal(ch.Argumentos, &args); err != nil {
		return "argumentos inválidos: " + err.Error(), true
	}
	res, err := a.mcp.CallTool(ctx, &sdk.CallToolParams{Name: ch.Nome, Arguments: args})
	if err != nil {
		return err.Error(), true
	}
	var b strings.Builder
	for _, c := range res.Content {
		if t, ok := c.(*sdk.TextContent); ok {
			b.WriteString(t.Text)
		}
	}
	return b.String(), res.IsError
}

// resumir picks the arguments a person cares about: "Cliente", "cpf → Cliente"...
func resumir(args json.RawMessage) string {
	var m map[string]any
	json.Unmarshal(args, &m)
	var partes []string
	for _, k := range []string{"nome", "elemento", "entidade_a", "entidade_b", "entidade", "relacionamento", "cardinalidade", "dono", "tipo", "banco", "caminho"} {
		if v, ok := m[k]; ok && fmt.Sprint(v) != "" {
			partes = append(partes, fmt.Sprint(v))
		}
	}
	return strings.Join(partes, " · ")
}
