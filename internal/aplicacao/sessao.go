// Package aplicacao holds the open diagram and the use cases that change it.
// Both the window and the MCP server go through here, so a shape created by
// Claude is sized, placed and linked exactly like one created with the mouse.
package aplicacao

import (
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"

	dom "github.com/brmodelo/gobrmodelo/internal/dominio"
	"github.com/brmodelo/gobrmodelo/internal/infra/sqlgen"
	"github.com/brmodelo/gobrmodelo/internal/infra/xmlbrm"
)

// Sessao is the diagram being edited plus the hooks the presentation plugs in.
type Sessao struct {
	D *dom.Diagrama

	// Medir returns a label's width in pixels; the window measures its real
	// fonts, tests and headless runs use an estimate.
	Medir func(texto string, negrito bool) float32
	// Executar runs a change on the thread that owns the diagram (the UI
	// thread in the app); by default it just calls the function.
	Executar func(func())
	// AoMudar tells the presentation to redraw after a change made outside it.
	AoMudar func()
	// AoGerarLogico shows a freshly converted logical model.
	AoGerarLogico func(l *dom.Logico, erros []string)

	desfazer, refazer []*dom.Diagrama // undo/redo snapshots
	grupo             int             // >0: inside a group, changes share one undo step
}

// limiteDesfazer bounds memory: snapshots are full copies of the diagram.
const limiteDesfazer = 100

// Marcar records the current state as an undo point; call it right before a
// change. Inside a group only the group's start counts.
func (s *Sessao) Marcar() {
	if s.grupo > 0 {
		return
	}
	if n := len(s.desfazer); n > 0 && s.desfazer[n-1].Assinatura() == s.D.Assinatura() {
		return // nothing changed since the last mark
	}
	s.desfazer = append(s.desfazer, s.D.Copia())
	if len(s.desfazer) > limiteDesfazer {
		s.desfazer = s.desfazer[1:]
	}
	s.refazer = nil
}

// IniciarGrupo makes everything until FimGrupo a single undo step (one
// request to the assistant, however many tools it runs).
func (s *Sessao) IniciarGrupo() {
	s.Marcar()
	s.grupo++
}

func (s *Sessao) FimGrupo() {
	if s.grupo > 0 {
		s.grupo--
	}
}

// Desfazer goes back one step; false when there is nothing to undo.
func (s *Sessao) Desfazer() bool {
	for len(s.desfazer) > 0 {
		ant := s.desfazer[len(s.desfazer)-1]
		s.desfazer = s.desfazer[:len(s.desfazer)-1]
		if ant.Assinatura() == s.D.Assinatura() {
			continue // a mark where nothing ended up changing
		}
		s.refazer = append(s.refazer, s.D)
		s.D = ant
		return true
	}
	return false
}

// Refazer reapplies the last undone step; false when there is none.
func (s *Sessao) Refazer() bool {
	if len(s.refazer) == 0 {
		return false
	}
	s.desfazer = append(s.desfazer, s.D)
	s.D = s.refazer[len(s.refazer)-1]
	s.refazer = s.refazer[:len(s.refazer)-1]
	return true
}

func (s *Sessao) PodeDesfazer() bool { return len(s.desfazer) > 0 }
func (s *Sessao) PodeRefazer() bool  { return len(s.refazer) > 0 }

func Nova() *Sessao {
	return &Sessao{
		D:        &dom.Diagrama{},
		Medir:    func(t string, _ bool) float32 { return 7 * float32(len([]rune(t))) },
		Executar: func(f func()) { f() },
	}
}

// Fazer runs a change through Executar and then asks for a redraw; it is how
// callers outside the UI thread (the MCP server) touch the diagram.
func (s *Sessao) Fazer(f func() error) error {
	var err error
	s.Executar(func() {
		s.Marcar()
		err = f()
		if err == nil && s.AoMudar != nil {
			s.AoMudar()
		}
	})
	return err
}

// Ajustar resizes a shape to fit its text.
func (s *Sessao) Ajustar(f *dom.Forma) {
	switch f.Tipo {
	case dom.Atributo:
		f.W = 16 + s.Medir(f.Texto, false)
	case dom.Relacionamento:
		f.W = max(100, s.Medir(f.Texto, true)+50)
	default:
		f.W = max(120, s.Medir(f.Texto, true)+20)
	}
}

var nomePadrao = map[dom.Tipo]string{dom.Entidade: "Entidade", dom.Relacionamento: "Rel", dom.Atributo: "atributo"}

// CriarForma adds a shape at p; an empty name gets the tool's default.
func (s *Sessao) CriarForma(t dom.Tipo, p dom.Ponto, nome string) *dom.Forma {
	if nome == "" {
		nome = nomePadrao[t]
	}
	f := s.D.Add(t, p.X, p.Y, nome)
	s.Ajustar(f)
	return f
}

// CriarEntidade adds an entity at p, or in the next free spot when p is nil.
func (s *Sessao) CriarEntidade(nome string, p *dom.Ponto) *dom.Forma {
	if p == nil {
		livre := s.D.LivreParaEntidade(120, 60)
		p = &livre
	}
	return s.CriarForma(dom.Entidade, *p, nome)
}

// AdicionarAtributo creates an attribute attached to dono, on the side nearest
// perto; without perto, attributes alternate right and left of the owner.
func (s *Sessao) AdicionarAtributo(dono *dom.Forma, nome, tipo string, chave bool, perto *dom.Ponto) (*dom.Forma, error) {
	if dono.Tipo == dom.Atributo {
		return nil, fmt.Errorf("um atributo não pode ter atributos; escolha uma entidade ou um relacionamento")
	}
	if perto == nil {
		n := 0
		for _, l := range s.D.Ligacoes {
			if l.B == dono.ID && s.D.Forma(l.A).Tipo == dom.Atributo {
				n++
			}
		}
		p := dom.Ponto{X: dono.X + dono.W - 1, Y: dono.Y + dono.H/2}
		if n%2 == 1 {
			p.X = dono.X + 1
		}
		perto = &p
	}
	a := s.CriarForma(dom.Atributo, *perto, nome)
	a.TipoAtributo, a.Identificador = tipo, chave
	s.D.Anexar(a, dono, *perto)
	return a, nil
}

// Relacionar creates a relationship between two entities (the same one twice
// for a self-relationship), placed between them and linked to both.
func (s *Sessao) Relacionar(e1, e2 *dom.Forma, nome string) (*dom.Forma, error) {
	if e1.Tipo != dom.Entidade || e2.Tipo != dom.Entidade {
		return nil, fmt.Errorf("um relacionamento liga entidades; %q e %q não são ambas entidades", e1.Texto, e2.Texto)
	}
	r := s.CriarForma(dom.Relacionamento, dom.Ponto{}, nome)
	dom.PosicionarEntre(r, e1, e2)
	s.D.Desviar(r) // not on top of an attribute or another relationship
	if _, err := s.D.Ligar(e1, r); err != nil {
		return nil, err
	}
	if _, err := s.D.Ligar(e2, r); err != nil {
		return nil, err
	}
	return r, nil
}

// Ligar links two shapes; two entities get a relationship between them, which is returned.
func (s *Sessao) Ligar(a, b *dom.Forma) (*dom.Forma, error) {
	if a.Tipo == dom.Entidade && b.Tipo == dom.Entidade {
		return s.Relacionar(a, b, "")
	}
	_, err := s.D.Ligar(a, b)
	return nil, err
}

// Renomear changes a shape's text and refits it.
func (s *Sessao) Renomear(f *dom.Forma, nome string) {
	f.Texto = nome
	s.Ajustar(f)
}

// DefinirCardinalidade sets the cardinality where entidade meets relacionamento.
// ocorrencia (1 or 2) picks the side of a self-relationship; 0 means the first.
func (s *Sessao) DefinirCardinalidade(rel, ent *dom.Forma, card string, ocorrencia int) error {
	c := indiceCard(card)
	if c < 0 {
		return fmt.Errorf("cardinalidade %q inválida; use uma de %s", card, strings.Join(dom.Cards, ", "))
	}
	n := 0
	for _, l := range s.D.Ligacoes {
		if l.Card >= 0 && ((l.A == rel.ID && l.B == ent.ID) || (l.A == ent.ID && l.B == rel.ID)) {
			if n++; ocorrencia <= 1 || n == ocorrencia {
				l.Card = c
				return nil
			}
		}
	}
	return fmt.Errorf("%q não está ligada a %q", ent.Texto, rel.Texto)
}

func indiceCard(s string) int {
	s = strings.ReplaceAll(strings.ToLower(strings.TrimSpace(s)), " ", "")
	if !strings.HasPrefix(s, "(") {
		s = "(" + s + ")"
	}
	for i, c := range dom.Cards {
		if c == s {
			return i
		}
	}
	return -1
}

// Achar finds a shape by ID ("12") or by name (case-insensitive).
func (s *Sessao) Achar(ref string) (*dom.Forma, error) {
	ref = strings.TrimSpace(ref)
	if id, err := strconv.Atoi(ref); err == nil {
		if f := s.D.Forma(id); f != nil {
			return f, nil
		}
		return nil, fmt.Errorf("não há elemento com ID %d", id)
	}
	var achados []*dom.Forma
	for _, f := range s.D.Formas {
		if strings.EqualFold(f.Texto, ref) {
			achados = append(achados, f)
		}
	}
	switch len(achados) {
	case 0:
		return nil, fmt.Errorf("não há elemento chamado %q", ref)
	case 1:
		return achados[0], nil
	}
	ids := make([]string, len(achados))
	for i, f := range achados {
		ids[i] = fmt.Sprintf("%d (%s)", f.ID, f.Tipo)
	}
	return nil, fmt.Errorf("há %d elementos chamados %q: use o ID, um de %s", len(achados), ref, strings.Join(ids, ", "))
}

// Remover deletes a shape (with its links) or a link.
func (s *Sessao) Remover(id int) { s.D.Remove(id) }

// Novo starts an empty diagram.
func (s *Sessao) Novo() { s.D = &dom.Diagrama{} }

func (s *Sessao) AbrirArquivo(caminho string) error {
	f, err := os.Open(caminho)
	if err != nil {
		return err
	}
	defer f.Close()
	d, err := xmlbrm.LerConceitual(f)
	if err != nil {
		return err
	}
	s.D = d
	return nil
}

func (s *Sessao) SalvarArquivo(caminho string) error {
	f, err := os.Create(caminho)
	if err != nil {
		return err
	}
	if err := xmlbrm.SalvarConceitual(f, s.D); err != nil {
		f.Close()
		return err
	}
	return f.Close()
}

// GerarLogico converts the diagram and hands the result to the presentation.
func (s *Sessao) GerarLogico() (*dom.Logico, []string) {
	l, erros := dom.Converter(s.D)
	if s.AoGerarLogico != nil {
		s.AoGerarLogico(l, erros)
	}
	return l, erros
}

// SQL converts the diagram and writes the migration for the named database.
func (s *Sessao) SQL(banco string) (string, error) {
	for i, nome := range sqlgen.Dialetos {
		if strings.EqualFold(strings.ReplaceAll(nome, " ", ""), strings.ReplaceAll(banco, " ", "")) {
			l, _ := dom.Converter(s.D)
			return sqlgen.Gerar(l, sqlgen.Dialeto(i), time.Now()), nil
		}
	}
	return "", fmt.Errorf("banco %q não suportado; use PostgreSQL, MySQL ou SQL Server", banco)
}
