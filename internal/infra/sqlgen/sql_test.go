package sqlgen

import (
	"strings"
	"testing"
	"time"

	dom "github.com/brmodelo/gobrmodelo/internal/dominio"
)

func TestSQL(t *testing.T) {
	d := &dom.Diagrama{}
	dep := d.Add(dom.Entidade, 0, 0, "Departamento")
	emp := d.Add(dom.Entidade, 300, 0, "Empregado")
	for _, p := range []struct {
		e          *dom.Forma
		nome, tipo string
		chave      bool
	}{{dep, "cod", "INT", true}, {dep, "nome", "VARCHAR(80)", false}, {emp, "mat", "INT", true}, {emp, "ativo", "BOOLEAN", false}, {emp, "obs", "", false}} {
		a := d.Add(dom.Atributo, 0, 0, p.nome)
		a.Identificador, a.TipoAtributo = p.chave, p.tipo
		ligar(t, d, p.e, a, -1)
	}
	lot := d.Add(dom.Relacionamento, 0, 0, "lotacao")
	ligar(t, d, dep, lot, 0)
	ligar(t, d, emp, lot, 3)
	chefia := d.Add(dom.Relacionamento, 0, 0, "chefia") // self-relationship with a FK column
	ligar(t, d, emp, chefia, 1)
	ligar(t, d, emp, chefia, 1)
	l, erros := dom.Converter(d)
	if len(erros) > 0 {
		t.Fatal(erros)
	}
	agora := time.Date(2026, 10, 12, 14, 30, 0, 0, time.UTC)

	pg := Gerar(l, PostgreSQL, agora)
	for _, want := range []string{
		`CREATE TABLE "Departamento" (`,
		`"cod" INT NOT NULL`,
		`"nome" VARCHAR(80)`,
		`"obs" VARCHAR(255)`, // no type given
		`CONSTRAINT "pk_Departamento" PRIMARY KEY ("cod")`,
		`ALTER TABLE "Empregado" ADD CONSTRAINT "fk_Empregado_Departamento"`,
		`FOREIGN KEY ("fk_Departamento_cod") REFERENCES "Departamento" ("cod")`,
		`REFERENCES "Empregado" ("mat")`, // self-relationship
		"BEGIN;", "COMMIT;",
	} {
		if !strings.Contains(pg, want) {
			t.Errorf("PostgreSQL missing %q in:\n%s", want, pg)
		}
	}
	// foreign keys come after every CREATE TABLE, so order never matters
	if strings.LastIndex(pg, "CREATE TABLE") > strings.Index(pg, "ALTER TABLE") {
		t.Error("ALTER TABLE before the last CREATE TABLE")
	}

	my := Gerar(l, MySQL, agora)
	for _, want := range []string{"CREATE TABLE `Empregado` (", "`ativo` BOOLEAN", ") ENGINE=InnoDB;"} {
		if !strings.Contains(my, want) {
			t.Errorf("MySQL missing %q in:\n%s", want, my)
		}
	}
	if strings.Contains(my, "COMMIT") {
		t.Error("MySQL migration should not wrap DDL in a transaction")
	}

	ms := Gerar(l, SQLServer, agora)
	for _, want := range []string{"CREATE TABLE [Empregado] (", "[ativo] BIT", "BEGIN TRANSACTION;"} {
		if !strings.Contains(ms, want) {
			t.Errorf("SQL Server missing %q in:\n%s", want, ms)
		}
	}
	if strings.Contains(ms, "RESTRICT") || strings.Contains(ms, "CASCADE\n    ;") {
		t.Errorf("SQL Server has no RESTRICT:\n%s", ms)
	}
	if NomeMigration(agora) != "20261012143000_criar_tabelas.sql" {
		t.Error(NomeMigration(agora))
	}
}

func ligar(t *testing.T, d *dom.Diagrama, a, b *dom.Forma, card int) {
	l, err := d.Ligar(a, b)
	if err != nil {
		t.Fatal(err)
	}
	l.Card = card
}
