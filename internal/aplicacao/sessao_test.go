package aplicacao

import (
	"path/filepath"
	"strings"
	"testing"

	dom "github.com/brmodelo/gobrmodelo/internal/dominio"
)

func TestCasosDeUso(t *testing.T) {
	s := Nova()
	cli := s.CriarEntidade("Cliente", nil)
	ped := s.CriarEntidade("Pedido", nil)
	if cli.X == ped.X && cli.Y == ped.Y {
		t.Fatal("entities without a position must not land on top of each other")
	}
	if _, err := s.AdicionarAtributo(cli, "cpf", "VARCHAR(11)", true, nil); err != nil {
		t.Fatal(err)
	}
	s.AdicionarAtributo(ped, "numero", "INT", true, nil)
	r, err := s.Relacionar(cli, ped, "faz")
	if err != nil {
		t.Fatal(err)
	}
	if err := s.DefinirCardinalidade(r, cli, "1,1", 0); err != nil {
		t.Fatal(err)
	}
	if err := s.DefinirCardinalidade(r, ped, "(9,9)", 0); err == nil {
		t.Fatal("invalid cardinality must be refused")
	}

	// a relationship whose midpoint is taken (here by an attribute) moves to free paper
	ocupa := s.CriarForma(dom.Atributo, dom.Ponto{X: (cli.X + cli.W + ped.X) / 2, Y: cli.Y + cli.H/2}, "ocupa")
	r2, _ := s.Relacionar(cli, ped, "outro")
	if s.D.Ocupado(r2) {
		t.Fatal("relationship placed on top of something")
	}
	s.Remover(r2.ID)
	s.Remover(ocupa.ID)

	res := s.Resumo()
	if len(res.Entidades) != 2 || len(res.Relacionamentos) != 1 || res.Entidades[0].Atributos[0].Tipo != "VARCHAR(11)" {
		t.Fatalf("%+v", res)
	}
	if p := res.Relacionamentos[0].Participantes; len(p) != 2 || p[0].Cardinalidade != "(1,1)" || p[1].Cardinalidade != "(0,n)" {
		t.Fatalf("participantes: %+v", p)
	}

	// names resolve case-insensitively; ambiguous names ask for the ID
	if f, err := s.Achar("cliente"); err != nil || f != cli {
		t.Fatal(f, err)
	}
	s.CriarEntidade("Cliente", nil)
	if _, err := s.Achar("Cliente"); err == nil || !strings.Contains(err.Error(), "use o ID") {
		t.Fatal("ambiguous name should be refused", err)
	}

	sql, err := s.SQL("postgresql")
	if err != nil || !strings.Contains(sql, `REFERENCES "Cliente" ("cpf")`) {
		t.Fatalf("%v\n%s", err, sql)
	}
	if _, err := s.SQL("oracle"); err == nil {
		t.Fatal("unsupported database must be refused")
	}

	// Fazer runs through Executar and redraws only on success
	redesenhos, executou := 0, 0
	s.Executar = func(f func()) { executou++; f() }
	s.AoMudar = func() { redesenhos++ }
	s.Fazer(func() error { s.Renomear(cli, "Comprador"); return nil })
	s.Fazer(func() error { _, err := s.Relacionar(cli, r, ""); return err })
	if executou != 2 || redesenhos != 1 || cli.Texto != "Comprador" {
		t.Fatalf("executou=%d redesenhos=%d", executou, redesenhos)
	}

	arq := filepath.Join(t.TempDir(), "modelo.xml")
	if err := s.SalvarArquivo(arq); err != nil {
		t.Fatal(err)
	}
	s.Novo()
	if err := s.AbrirArquivo(arq); err != nil || len(s.D.Formas) != len(s.Resumo().Entidades)+3 {
		t.Fatal(err, len(s.D.Formas))
	}
	if s.D.Formas[0].Tipo != dom.Entidade {
		t.Fatal("order preserved")
	}
}

func TestDesfazer(t *testing.T) {
	s := Nova()
	if s.Desfazer() || s.Refazer() {
		t.Fatal("nothing to undo or redo yet")
	}
	s.Marcar()
	a := s.CriarEntidade("A", nil)
	s.Marcar()
	s.Renomear(a, "Alfa")
	s.Marcar() // a mark with no change after it is skipped on undo

	if !s.Desfazer() || s.D.Forma(a.ID).Texto != "A" {
		t.Fatal("undo rename")
	}
	if !s.Desfazer() || len(s.D.Formas) != 0 {
		t.Fatal("undo create")
	}
	if s.Desfazer() {
		t.Fatal("nothing left")
	}
	if !s.Refazer() || !s.Refazer() || s.D.Formas[0].Texto != "Alfa" {
		t.Fatal("redo both")
	}
	s.Desfazer()
	s.Marcar()
	s.CriarEntidade("B", nil)
	if s.PodeRefazer() {
		t.Fatal("a new change clears redo")
	}

	// a group (one assistant request) is one step, however many changes it makes
	antes := len(s.D.Formas)
	s.IniciarGrupo()
	for _, n := range []string{"C", "D", "E"} {
		s.Fazer(func() error { s.CriarEntidade(n, nil); return nil })
	}
	s.FimGrupo()
	if len(s.D.Formas) != antes+3 || !s.Desfazer() || len(s.D.Formas) != antes {
		t.Fatalf("group not undone as one step: %d", len(s.D.Formas))
	}
	// outside a group, every Fazer (one MCP tool call) is its own step
	s.Fazer(func() error { s.CriarEntidade("F", nil); return nil })
	s.Fazer(func() error { s.CriarEntidade("G", nil); return nil })
	s.Desfazer()
	if len(s.D.Formas) != antes+1 {
		t.Fatal("Fazer should mark one step per call")
	}
}
