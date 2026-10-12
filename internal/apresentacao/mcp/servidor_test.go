package mcp

import (
	"context"
	"encoding/json"
	"net/http"
	"strings"
	"testing"

	sdk "github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/brmodelo/gobrmodelo/internal/aplicacao"
)

// TestFerramentas drives the real HTTP endpoint with the SDK's client, the way
// Claude would, and checks the diagram and the redraw hook behind it.
func TestFerramentas(t *testing.T) {
	s := aplicacao.Nova()
	redesenhos := 0
	s.AoMudar = func() { redesenhos++ }
	url, parar, err := Servir(s, 0) // port 0: any free port
	if err != nil {
		t.Fatal(err)
	}
	defer parar()

	ctx := context.Background()
	cli := sdk.NewClient(&sdk.Implementation{Name: "teste", Version: "0"}, nil)
	sess, err := cli.Connect(ctx, &sdk.StreamableClientTransport{Endpoint: url}, nil)
	if err != nil {
		t.Fatal(err)
	}
	defer sess.Close()

	chamar := func(nome string, args map[string]any) (*sdk.CallToolResult, map[string]any) {
		t.Helper()
		res, err := sess.CallTool(ctx, &sdk.CallToolParams{Name: nome, Arguments: args})
		if err != nil {
			t.Fatalf("%s: %v", nome, err)
		}
		var out map[string]any
		if res.StructuredContent != nil {
			b, _ := json.Marshal(res.StructuredContent)
			json.Unmarshal(b, &out)
		}
		return res, out
	}
	ok := func(nome string, args map[string]any) map[string]any {
		t.Helper()
		res, out := chamar(nome, args)
		if res.IsError {
			t.Fatalf("%s failed: %v", nome, res.Content[0].(*sdk.TextContent).Text)
		}
		return out
	}

	ferramentas, err := sess.ListTools(ctx, nil)
	if err != nil || len(ferramentas.Tools) != 14 {
		t.Fatalf("tools: %v %d", err, len(ferramentas.Tools))
	}

	ok("criar_entidade", map[string]any{"nome": "Cliente"})
	ok("criar_entidade", map[string]any{"nome": "Pedido"})
	ok("adicionar_atributo", map[string]any{"dono": "Cliente", "nome": "cpf", "tipo": "VARCHAR(11)", "identificador": true})
	ok("adicionar_atributo", map[string]any{"dono": "pedido", "nome": "numero", "tipo": "INT", "identificador": true})
	ok("criar_relacionamento", map[string]any{"entidade_a": "Cliente", "entidade_b": "Pedido", "nome": "faz",
		"cardinalidade_a": "(1,1)", "cardinalidade_b": "(0,n)"})
	ok("criar_relacionamento", map[string]any{"entidade_a": "Cliente", "entidade_b": "Cliente", "nome": "indica",
		"cardinalidade_a": "(0,1)", "cardinalidade_b": "(0,n)"})
	if redesenhos != 6 {
		t.Fatalf("every change must redraw the window: %d", redesenhos)
	}

	desc := ok("descrever_diagrama", nil)
	b, _ := json.Marshal(desc)
	for _, want := range []string{`"nome":"Cliente"`, `"tipo":"VARCHAR(11)"`, `"cardinalidade":"(1,1)"`, `"nome":"indica"`} {
		if !strings.Contains(string(b), want) {
			t.Errorf("descrever_diagrama missing %s in %s", want, b)
		}
	}

	// tool errors come back as isError results with a readable message, and change nothing
	antes := len(s.D.Formas)
	for _, c := range []struct {
		nome string
		args map[string]any
		msg  string
	}{
		{"adicionar_atributo", map[string]any{"dono": "Fornecedor", "nome": "x"}, "não há elemento chamado"},
		{"criar_relacionamento", map[string]any{"entidade_a": "Cliente", "entidade_b": "Pedido", "cardinalidade_a": "(2,3)"}, "inválida"},
		{"editar_elemento", map[string]any{"elemento": "Cliente", "tipo": "INT"}, "só se aplicam a atributos"},
		{"gerar_sql", map[string]any{"banco": "Oracle"}, "não suportado"},
	} {
		res, _ := chamar(c.nome, c.args)
		if !res.IsError || !strings.Contains(res.Content[0].(*sdk.TextContent).Text, c.msg) {
			t.Errorf("%s: want error containing %q, got %+v", c.nome, c.msg, res.Content)
		}
	}
	if len(s.D.Formas) != antes {
		t.Fatalf("a failed tool left %d shapes behind", len(s.D.Formas)-antes)
	}

	ok("editar_elemento", map[string]any{"elemento": "numero", "tipo": "BIGINT"})
	logico := ok("gerar_modelo_logico", nil)
	lb, _ := json.Marshal(logico)
	if !strings.Contains(string(lb), `"referencia":"Cliente"`) || !strings.Contains(string(lb), `"tipo":"BIGINT"`) {
		t.Errorf("gerar_modelo_logico: %s", lb)
	}
	sql := ok("gerar_sql", map[string]any{"banco": "sql server"})["sql"].(string)
	if !strings.Contains(sql, "CREATE TABLE [Cliente]") || !strings.Contains(sql, "REFERENCES [Cliente] ([cpf])") {
		t.Errorf("gerar_sql:\n%s", sql)
	}

	arq := t.TempDir() + "/modelo.xml"
	ok("salvar_diagrama", map[string]any{"caminho": arq})
	ok("novo_diagrama", nil)
	ok("abrir_diagrama", map[string]any{"caminho": arq})
	if len(s.Resumo().Entidades) != 2 {
		t.Fatal("reopened diagram lost entities")
	}
	ok("remover_elemento", map[string]any{"elemento": "Pedido"})
	if len(s.Resumo().Entidades) != 1 {
		t.Fatal("remover_elemento")
	}
	ok("desfazer", nil)
	if len(s.Resumo().Entidades) != 2 {
		t.Fatal("desfazer should bring Pedido back")
	}
	ok("refazer", nil)
	if len(s.Resumo().Entidades) != 1 {
		t.Fatal("refazer should remove it again")
	}
	if res, _ := chamar("refazer", nil); !res.IsError {
		t.Fatal("nothing left to redo must be an error")
	}
}

// TestSoLocal: a request reaching the loopback port with a foreign Host header
// (a DNS-rebinding attack from a web page) is refused.
func TestSoLocal(t *testing.T) {
	url, parar, err := Servir(aplicacao.Nova(), 0)
	if err != nil {
		t.Fatal(err)
	}
	defer parar()
	req, _ := http.NewRequest("POST", url, strings.NewReader(`{"jsonrpc":"2.0","id":1,"method":"initialize","params":{}}`))
	req.Host = "evil.example.com"
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json, text/event-stream")
	res, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	res.Body.Close()
	if res.StatusCode != http.StatusForbidden {
		t.Fatalf("status %d, want 403", res.StatusCode)
	}
}
