// Package mcp exposes the open diagram to MCP clients (Claude Code, Claude
// Desktop...) over streamable HTTP on localhost. Every tool goes through the
// application Sessao, and changes run on the window's thread, so what Claude
// does appears on screen as it happens.
package mcp

import (
	"context"
	"errors"
	"fmt"
	"net"
	"net/http"
	"strconv"

	sdk "github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/brmodelo/gobrmodelo/internal/aplicacao"
	dom "github.com/brmodelo/gobrmodelo/internal/dominio"
)

const instrucoes = `Este servidor controla o diagrama aberto no brModelo (Go), uma ferramenta de modelagem de banco de dados na notação de Peter Chen / Carlos Heuser.

Fluxo recomendado: chame descrever_diagrama para ver o que existe (com IDs), crie entidades, adicione atributos (marque o identificador, que vira chave primária, e informe o tipo SQL), ligue entidades com criar_relacionamento e defina as cardinalidades. Depois gere_modelo_logico para ver as tabelas e gerar_sql para a migration.

Elementos podem ser referenciados pelo ID ou pelo nome. Cardinalidades válidas: (1,1), (0,1), (1,n), (0,n). Cada mudança aparece na hora na janela do usuário.`

type idMsg struct {
	ID       int    `json:"id,omitempty" jsonschema:"ID do elemento criado ou alterado"`
	Mensagem string `json:"mensagem"`
}

type vazio struct{}

type entradaEntidade struct {
	Nome string   `json:"nome" jsonschema:"nome da entidade, por exemplo Cliente"`
	X    *float64 `json:"x,omitempty" jsonschema:"posição horizontal em pixels; omita para posicionar automaticamente"`
	Y    *float64 `json:"y,omitempty" jsonschema:"posição vertical em pixels; omita para posicionar automaticamente"`
}

type entradaAtributo struct {
	Dono          string `json:"dono" jsonschema:"ID ou nome da entidade ou do relacionamento dono do atributo"`
	Nome          string `json:"nome" jsonschema:"nome do atributo"`
	Tipo          string `json:"tipo,omitempty" jsonschema:"tipo SQL da coluna, por exemplo INT, VARCHAR(80), DATE"`
	Identificador bool   `json:"identificador,omitempty" jsonschema:"verdadeiro se o atributo identifica a entidade (vira chave primária)"`
}

type entradaRelacionamento struct {
	EntidadeA      string `json:"entidade_a" jsonschema:"ID ou nome da primeira entidade"`
	EntidadeB      string `json:"entidade_b" jsonschema:"ID ou nome da segunda entidade; a mesma da primeira cria um autorrelacionamento"`
	Nome           string `json:"nome,omitempty" jsonschema:"nome do relacionamento, por exemplo realiza"`
	CardinalidadeA string `json:"cardinalidade_a,omitempty" jsonschema:"cardinalidade do lado da entidade_a: (1,1), (0,1), (1,n) ou (0,n); padrão (0,n)"`
	CardinalidadeB string `json:"cardinalidade_b,omitempty" jsonschema:"cardinalidade do lado da entidade_b; padrão (0,n)"`
}

type entradaCardinalidade struct {
	Relacionamento string `json:"relacionamento" jsonschema:"ID ou nome do relacionamento"`
	Entidade       string `json:"entidade" jsonschema:"ID ou nome da entidade participante"`
	Cardinalidade  string `json:"cardinalidade" jsonschema:"(1,1), (0,1), (1,n) ou (0,n)"`
	Ocorrencia     int    `json:"ocorrencia,omitempty" jsonschema:"em autorrelacionamentos, 1 ou 2 escolhe o papel; omita nos demais casos"`
}

type entradaEditar struct {
	Elemento      string  `json:"elemento" jsonschema:"ID ou nome do elemento"`
	Nome          *string `json:"nome,omitempty" jsonschema:"novo nome"`
	Tipo          *string `json:"tipo,omitempty" jsonschema:"novo tipo SQL (só atributos)"`
	Identificador *bool   `json:"identificador,omitempty" jsonschema:"marca ou desmarca o atributo como identificador (só atributos)"`
}

type entradaRef struct {
	Elemento string `json:"elemento" jsonschema:"ID ou nome do elemento a remover (suas ligações também são removidas)"`
}

type entradaSQL struct {
	Banco string `json:"banco" jsonschema:"PostgreSQL, MySQL ou SQL Server"`
}

type saidaSQL struct {
	SQL string `json:"sql"`
}

type entradaArquivo struct {
	Caminho string `json:"caminho" jsonschema:"caminho absoluto do arquivo .xml do brModelo"`
}

type campoLogico struct {
	Nome  string `json:"nome"`
	Tipo  string `json:"tipo,omitempty"`
	Chave string `json:"chave,omitempty"`
}

type fkLogica struct {
	Campos     []string `json:"campos"`
	Referencia string   `json:"referencia"`
	OnDelete   string   `json:"on_delete,omitempty"`
}

type tabelaLogica struct {
	Nome   string        `json:"nome"`
	Campos []campoLogico `json:"campos"`
	FKs    []fkLogica    `json:"chaves_estrangeiras,omitempty"`
}

type saidaLogico struct {
	Tabelas []tabelaLogica `json:"tabelas"`
	Avisos  []string       `json:"avisos,omitempty"`
}

// NovoServidor registers the brModelo tools on an MCP server bound to s.
func NovoServidor(s *aplicacao.Sessao) *sdk.Server {
	srv := sdk.NewServer(&sdk.Implementation{Name: "brmodelo", Title: "brModelo (Go)", Version: "1.0.0"},
		&sdk.ServerOptions{Instructions: instrucoes})

	// ler runs a read on the diagram's thread without asking for a redraw.
	ler := func(f func()) { s.Executar(f) }
	achar := func(ref string) (*dom.Forma, error) { return s.Achar(ref) }
	somenteLeitura := &sdk.ToolAnnotations{ReadOnlyHint: true}

	sdk.AddTool(srv, &sdk.Tool{Name: "descrever_diagrama", Annotations: somenteLeitura,
		Description: "Descreve o diagrama conceitual aberto: entidades com atributos, relacionamentos com participantes e cardinalidades, e os IDs de tudo."},
		func(ctx context.Context, _ *sdk.CallToolRequest, _ vazio) (*sdk.CallToolResult, aplicacao.Resumo, error) {
			var r aplicacao.Resumo
			ler(func() { r = s.Resumo() })
			return nil, r, nil
		})

	sdk.AddTool(srv, &sdk.Tool{Name: "novo_diagrama",
		Description: "Descarta o diagrama aberto e começa um vazio. Alterações não salvas são perdidas."},
		func(ctx context.Context, _ *sdk.CallToolRequest, _ vazio) (*sdk.CallToolResult, idMsg, error) {
			err := s.Fazer(func() error { s.Novo(); return nil })
			return nil, idMsg{Mensagem: "diagrama vazio criado"}, err
		})

	sdk.AddTool(srv, &sdk.Tool{Name: "criar_entidade", Description: "Cria uma entidade no diagrama."},
		func(ctx context.Context, _ *sdk.CallToolRequest, in entradaEntidade) (*sdk.CallToolResult, idMsg, error) {
			var p *dom.Ponto
			if in.X != nil && in.Y != nil {
				p = &dom.Ponto{X: float32(*in.X), Y: float32(*in.Y)}
			}
			var f *dom.Forma
			err := s.Fazer(func() error { f = s.CriarEntidade(in.Nome, p); return nil })
			return nil, idMsg{f.ID, fmt.Sprintf("entidade %q criada", f.Texto)}, err
		})

	sdk.AddTool(srv, &sdk.Tool{Name: "adicionar_atributo",
		Description: "Adiciona um atributo a uma entidade ou relacionamento, já ligado a ele. Marque identificador=true para o atributo que vira chave primária."},
		func(ctx context.Context, _ *sdk.CallToolRequest, in entradaAtributo) (*sdk.CallToolResult, idMsg, error) {
			var a *dom.Forma
			err := s.Fazer(func() error {
				dono, err := achar(in.Dono)
				if err != nil {
					return err
				}
				a, err = s.AdicionarAtributo(dono, in.Nome, in.Tipo, in.Identificador, nil)
				return err
			})
			if err != nil {
				return nil, idMsg{}, err
			}
			return nil, idMsg{a.ID, fmt.Sprintf("atributo %q adicionado", a.Texto)}, nil
		})

	sdk.AddTool(srv, &sdk.Tool{Name: "criar_relacionamento",
		Description: "Cria um relacionamento entre duas entidades (ou um autorrelacionamento, repetindo a entidade), posicionado entre elas, com as cardinalidades de cada lado."},
		func(ctx context.Context, _ *sdk.CallToolRequest, in entradaRelacionamento) (*sdk.CallToolResult, idMsg, error) {
			var r *dom.Forma
			err := s.Fazer(func() error {
				a, err := achar(in.EntidadeA)
				if err != nil {
					return err
				}
				b, err := achar(in.EntidadeB)
				if err != nil {
					return err
				}
				if r, err = s.Relacionar(a, b, in.Nome); err != nil {
					return err
				}
				// sides in creation order: occurrence 1 is entidade_a, 2 is entidade_b (matters for self-relationships)
				for i, c := range []struct {
					e    *dom.Forma
					card string
				}{{a, in.CardinalidadeA}, {b, in.CardinalidadeB}} {
					if c.card == "" {
						continue
					}
					oc := 0
					if a == b {
						oc = i + 1
					}
					if err := s.DefinirCardinalidade(r, c.e, c.card, oc); err != nil {
						s.Remover(r.ID)
						return err
					}
				}
				return nil
			})
			if err != nil {
				return nil, idMsg{}, err
			}
			return nil, idMsg{r.ID, fmt.Sprintf("relacionamento %q criado", r.Texto)}, nil
		})

	sdk.AddTool(srv, &sdk.Tool{Name: "definir_cardinalidade",
		Description: "Define a cardinalidade de uma entidade em um relacionamento."},
		func(ctx context.Context, _ *sdk.CallToolRequest, in entradaCardinalidade) (*sdk.CallToolResult, idMsg, error) {
			err := s.Fazer(func() error {
				r, err := achar(in.Relacionamento)
				if err != nil {
					return err
				}
				e, err := achar(in.Entidade)
				if err != nil {
					return err
				}
				return s.DefinirCardinalidade(r, e, in.Cardinalidade, in.Ocorrencia)
			})
			if err != nil {
				return nil, idMsg{}, err
			}
			return nil, idMsg{Mensagem: "cardinalidade definida"}, nil
		})

	sdk.AddTool(srv, &sdk.Tool{Name: "editar_elemento",
		Description: "Renomeia um elemento e, em atributos, muda o tipo SQL ou se é identificador."},
		func(ctx context.Context, _ *sdk.CallToolRequest, in entradaEditar) (*sdk.CallToolResult, idMsg, error) {
			var f *dom.Forma
			err := s.Fazer(func() error {
				var err error
				if f, err = achar(in.Elemento); err != nil {
					return err
				}
				if (in.Tipo != nil || in.Identificador != nil) && f.Tipo != dom.Atributo {
					return errors.New("tipo e identificador só se aplicam a atributos")
				}
				if in.Nome != nil {
					s.Renomear(f, *in.Nome)
				}
				if in.Tipo != nil {
					f.TipoAtributo = *in.Tipo
				}
				if in.Identificador != nil {
					f.Identificador = *in.Identificador
				}
				return nil
			})
			if err != nil {
				return nil, idMsg{}, err
			}
			return nil, idMsg{f.ID, fmt.Sprintf("%s %q atualizado", f.Tipo, f.Texto)}, nil
		})

	sdk.AddTool(srv, &sdk.Tool{Name: "remover_elemento",
		Description: "Remove uma entidade, relacionamento ou atributo, junto com as ligações dele."},
		func(ctx context.Context, _ *sdk.CallToolRequest, in entradaRef) (*sdk.CallToolResult, idMsg, error) {
			var nome string
			err := s.Fazer(func() error {
				f, err := achar(in.Elemento)
				if err != nil {
					return err
				}
				nome = f.Texto
				s.Remover(f.ID)
				return nil
			})
			if err != nil {
				return nil, idMsg{}, err
			}
			return nil, idMsg{Mensagem: fmt.Sprintf("%q removido", nome)}, nil
		})

	// desfazer/refazer skip Fazer: marking a step first would clear the redo stack
	historico := func(nome, desc string, op func() bool, feito, nada string) {
		sdk.AddTool(srv, &sdk.Tool{Name: nome, Description: desc},
			func(ctx context.Context, _ *sdk.CallToolRequest, _ vazio) (*sdk.CallToolResult, idMsg, error) {
				ok := false
				s.Executar(func() {
					if ok = op(); ok && s.AoMudar != nil {
						s.AoMudar()
					}
				})
				if !ok {
					return nil, idMsg{}, errors.New(nada)
				}
				return nil, idMsg{Mensagem: feito}, nil
			})
	}
	historico("desfazer", "Desfaz a última alteração do diagrama (a do usuário ou a sua). Use para voltar atrás de um erro.",
		s.Desfazer, "última alteração desfeita", "não há nada para desfazer")
	historico("refazer", "Refaz a última alteração desfeita.", s.Refazer, "alteração refeita", "não há nada para refazer")

	sdk.AddTool(srv, &sdk.Tool{Name: "gerar_modelo_logico",
		Description: "Converte o diagrama conceitual no modelo lógico (tabelas, chaves primárias e estrangeiras) com as regras do brModelo, abre a janela do modelo lógico para o usuário e devolve as tabelas."},
		func(ctx context.Context, _ *sdk.CallToolRequest, _ vazio) (*sdk.CallToolResult, saidaLogico, error) {
			var out saidaLogico
			ler(func() {
				l, avisos := s.GerarLogico()
				out = descreverLogico(l, avisos)
			})
			return nil, out, nil
		})

	sdk.AddTool(srv, &sdk.Tool{Name: "gerar_sql", Annotations: somenteLeitura,
		Description: "Gera a migration SQL (CREATE TABLE e chaves estrangeiras) do modelo lógico para PostgreSQL, MySQL ou SQL Server e devolve o texto."},
		func(ctx context.Context, _ *sdk.CallToolRequest, in entradaSQL) (*sdk.CallToolResult, saidaSQL, error) {
			var sql string
			var err error
			ler(func() { sql, err = s.SQL(in.Banco) })
			return nil, saidaSQL{sql}, err
		})

	sdk.AddTool(srv, &sdk.Tool{Name: "salvar_diagrama",
		Description: "Salva o diagrama conceitual em um arquivo .xml no formato do brModelo (abre também no brModelo Java)."},
		func(ctx context.Context, _ *sdk.CallToolRequest, in entradaArquivo) (*sdk.CallToolResult, idMsg, error) {
			var err error
			ler(func() { err = s.SalvarArquivo(in.Caminho) })
			if err != nil {
				return nil, idMsg{}, fmt.Errorf("não foi possível salvar %s: %w", in.Caminho, err)
			}
			return nil, idMsg{Mensagem: "salvo em " + in.Caminho}, nil
		})

	sdk.AddTool(srv, &sdk.Tool{Name: "abrir_diagrama",
		Description: "Abre um diagrama conceitual .xml do brModelo na janela, substituindo o atual."},
		func(ctx context.Context, _ *sdk.CallToolRequest, in entradaArquivo) (*sdk.CallToolResult, idMsg, error) {
			err := s.Fazer(func() error { return s.AbrirArquivo(in.Caminho) })
			if err != nil {
				return nil, idMsg{}, fmt.Errorf("não foi possível abrir %s: %w", in.Caminho, err)
			}
			return nil, idMsg{Mensagem: "aberto " + in.Caminho}, nil
		})

	return srv
}

func descreverLogico(l *dom.Logico, avisos []string) saidaLogico {
	out := saidaLogico{Tabelas: []tabelaLogica{}, Avisos: avisos}
	for _, t := range l.Tabelas {
		tl := tabelaLogica{Nome: t.Nome, Campos: []campoLogico{}}
		for _, c := range t.Campos {
			k := ""
			switch {
			case c.Key && c.Fkey:
				k = "PK, FK"
			case c.Key:
				k = "PK"
			case c.Fkey:
				k = "FK"
			}
			tl.Campos = append(tl.Campos, campoLogico{c.Nome, c.Tipo, k})
		}
		for _, fk := range t.FKs {
			f := fkLogica{Referencia: fk.Ref.Nome, OnDelete: fk.OnDelete}
			for _, p := range fk.Pares {
				if p[1] >= 0 && p[1] < len(t.Campos) {
					f.Campos = append(f.Campos, t.Campos[p[1]].Nome)
				}
			}
			tl.FKs = append(tl.FKs, f)
		}
		out.Tabelas = append(out.Tabelas, tl)
	}
	return out
}

// Endereco is where the server listens unless BRMODELO_MCP_PORTA says otherwise.
const PortaPadrao = 7421

// Servir starts the MCP endpoint on 127.0.0.1:porta/mcp in the background.
// Only loopback is bound, and the SDK rejects requests whose Host header isn't
// local (DNS-rebinding protection), so web pages can't drive the app.
func Servir(s *aplicacao.Sessao, porta int) (url string, parar func(), err error) {
	ln, err := net.Listen("tcp", net.JoinHostPort("127.0.0.1", strconv.Itoa(porta)))
	if err != nil {
		return "", nil, err
	}
	srv := NovoServidor(s)
	mux := http.NewServeMux()
	mux.Handle("/mcp", sdk.NewStreamableHTTPHandler(func(*http.Request) *sdk.Server { return srv }, nil))
	h := &http.Server{Handler: mux}
	go h.Serve(ln)
	return fmt.Sprintf("http://%s/mcp", ln.Addr()), func() { h.Close() }, nil
}
