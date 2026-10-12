// Package sqlgen writes a SQL migration (CREATE TABLE + ALTER TABLE ... FOREIGN
// KEY) from a logical diagram, for PostgreSQL, MySQL or SQL Server.
package sqlgen

import (
	"fmt"
	"regexp"
	"strings"
	"time"

	dom "github.com/brmodelo/gobrmodelo/internal/dominio"
)

// Dialeto is the target database of a generated migration.
type Dialeto int

const (
	PostgreSQL Dialeto = iota
	MySQL
	SQLServer
)

var Dialetos = []string{"PostgreSQL", "MySQL", "SQL Server"}

func (d Dialeto) String() string { return Dialetos[d] }

func (d Dialeto) citar(nome string) string {
	switch d {
	case MySQL:
		return "`" + strings.ReplaceAll(nome, "`", "``") + "`"
	case SQLServer:
		return "[" + strings.ReplaceAll(nome, "]", "]]") + "]"
	}
	return `"` + strings.ReplaceAll(nome, `"`, `""`) + `"`
}

var tipoBase = regexp.MustCompile(`^\s*([A-Za-z ]+?)\s*(\(.*\))?\s*$`)

// tipo translates a field type to the dialect; an empty type becomes INT for
// keys and VARCHAR(255) otherwise.
func (d Dialeto) tipo(c *dom.Campo) string {
	t := strings.TrimSpace(c.Tipo)
	if t == "" {
		if c.Key || c.Fkey {
			return "INT"
		}
		return "VARCHAR(255)"
	}
	m := tipoBase.FindStringSubmatch(t)
	if m == nil {
		return t // something we don't know: trust the author
	}
	base, args := strings.ToUpper(m[1]), m[2]
	switch d {
	case PostgreSQL:
		switch base {
		case "FLOAT", "DOUBLE":
			return "DOUBLE PRECISION"
		case "DATETIME":
			return "TIMESTAMP"
		case "TINYINT":
			return "SMALLINT"
		}
	case MySQL:
		switch base {
		case "SERIAL":
			return "INT AUTO_INCREMENT"
		case "VARCHAR":
			if args == "" {
				return "VARCHAR(255)"
			}
		}
	case SQLServer:
		switch base {
		case "BOOLEAN", "BOOL":
			return "BIT"
		case "TEXT":
			return "NVARCHAR(MAX)"
		case "TIMESTAMP", "DATETIME":
			return "DATETIME2"
		case "DOUBLE":
			return "FLOAT"
		case "VARCHAR":
			if args == "" {
				return "VARCHAR(255)"
			}
		}
	}
	return base + args
}

// acao maps an ON DELETE rule; SQL Server has no RESTRICT, NO ACTION does the same there.
func (d Dialeto) acao(s string) string {
	s = strings.ToUpper(strings.TrimSpace(s))
	if d == SQLServer && s == "RESTRICT" {
		return "NO ACTION"
	}
	return s
}

// Gerar writes a migration that creates every table with its primary key, then
// adds the foreign keys with ALTER TABLE, so table order and cycles (including
// self-relationships) never matter. Repeated table names get a numeric suffix.
func Gerar(l *dom.Logico, d Dialeto, agora time.Time) string {
	nomes := map[*dom.Tabela]string{}
	usados := map[string]int{}
	for _, t := range l.Tabelas {
		n := t.Nome
		if usados[strings.ToLower(n)]++; usados[strings.ToLower(n)] > 1 {
			n = fmt.Sprintf("%s_%d", n, usados[strings.ToLower(n)])
		}
		nomes[t] = n
	}

	var b strings.Builder
	fmt.Fprintf(&b, "-- Migration gerada pelo brModelo (Go) para %s em %s.\n", d, agora.Format("02/01/2006 15:04"))
	b.WriteString("-- Cria as tabelas com suas chaves primárias e, em seguida, as chaves estrangeiras.\n\n")
	switch d {
	case PostgreSQL:
		b.WriteString("BEGIN;\n\n")
	case SQLServer:
		b.WriteString("BEGIN TRANSACTION;\n\n")
	}

	for _, t := range l.Tabelas {
		if len(t.Campos) == 0 {
			fmt.Fprintf(&b, "-- dom.Tabela %s não tem campos e foi ignorada.\n\n", nomes[t])
			continue
		}
		fmt.Fprintf(&b, "CREATE TABLE %s (\n", d.citar(nomes[t]))
		var linhas, pk []string
		for _, c := range t.Campos {
			col := "    " + d.citar(c.Nome) + " " + d.tipo(c)
			if c.Key {
				col += " NOT NULL"
				pk = append(pk, d.citar(c.Nome))
			}
			linhas = append(linhas, col)
		}
		if len(pk) > 0 {
			linhas = append(linhas, fmt.Sprintf("    CONSTRAINT %s PRIMARY KEY (%s)", d.citar("pk_"+nomes[t]), strings.Join(pk, ", ")))
		}
		b.WriteString(strings.Join(linhas, ",\n"))
		b.WriteString("\n)")
		if d == MySQL {
			b.WriteString(" ENGINE=InnoDB")
		}
		b.WriteString(";\n\n")
	}

	for _, t := range l.Tabelas {
		if len(t.Campos) == 0 {
			continue
		}
		porRef := map[*dom.Tabela]int{} // a second FK to the same table (two roles) gets _2
		for _, fk := range t.FKs {
			var de, para []string
			for _, p := range fk.Pares {
				if p[0] < 0 || p[0] >= len(fk.Ref.Campos) || p[1] < 0 || p[1] >= len(t.Campos) {
					continue
				}
				de = append(de, d.citar(t.Campos[p[1]].Nome))
				para = append(para, d.citar(fk.Ref.Campos[p[0]].Nome))
			}
			if len(de) == 0 || len(fk.Ref.Campos) == 0 {
				continue
			}
			nome := "fk_" + nomes[t] + "_" + nomes[fk.Ref]
			if porRef[fk.Ref]++; porRef[fk.Ref] > 1 {
				nome += fmt.Sprintf("_%d", porRef[fk.Ref])
			}
			fmt.Fprintf(&b, "ALTER TABLE %s ADD CONSTRAINT %s\n    FOREIGN KEY (%s) REFERENCES %s (%s)",
				d.citar(nomes[t]), d.citar(nome), strings.Join(de, ", "), d.citar(nomes[fk.Ref]), strings.Join(para, ", "))
			a := d.acao(fk.OnDelete)
			if d == SQLServer && fk.Ref == t && a != "" {
				a = "NO ACTION" // SQL Server refuses cascading paths on a self-reference
			}
			if a != "" {
				b.WriteString("\n    ON DELETE " + a)
			}
			b.WriteString(";\n\n")
		}
	}

	if d != MySQL { // MySQL commits DDL implicitly
		b.WriteString("COMMIT;\n")
	}
	return b.String()
}

// NomeMigration is the conventional timestamped file name for a migration.
func NomeMigration(agora time.Time) string {
	return agora.Format("20060102150405") + "_criar_tabelas.sql"
}
