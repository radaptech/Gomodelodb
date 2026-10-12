// Package dominio holds brModelo's diagrams and the rules that govern them:
// the conceptual (ER) model, the logical (relational) model, the conversion
// between them and the table organizer. It knows nothing about files or screens.
package dominio

import (
	"fmt"
	"strings"
)

type Tipo string

const (
	Entidade       Tipo = "Entidade"
	Relacionamento Tipo = "Relacionamento"
	Atributo       Tipo = "Atributo"
)

// Cards in the order of PreCardinalidade.TiposCard (index = Card Valor in XML).
var Cards = []string{"(1,1)", "(0,1)", "(1,n)", "(0,n)"}

type Forma struct {
	ID            int
	Tipo          Tipo
	X, Y, W, H    float32
	Texto         string
	Identificador bool   // Atributo only
	TipoAtributo  string // Atributo only: SQL type the converter gives its field (INT, VARCHAR(80)...)
}

type Ligacao struct {
	ID   int
	A, B int // Forma IDs (PontaA, PontaB)
	Card int // index into Cards; -1 = none (only entidade<->relacionamento gets one)
}

type Diagrama struct {
	Formas   []*Forma
	Ligacoes []*Ligacao
	nextID   int
}

func (d *Diagrama) newID() int {
	if d.nextID < 2 {
		d.nextID = 2 // 1 is the diagram itself
	}
	d.nextID++
	return d.nextID - 1
}

func (d *Diagrama) Add(t Tipo, x, y float32, texto string) *Forma {
	f := &Forma{ID: d.newID(), Tipo: t, X: x, Y: y, Texto: texto}
	switch t {
	case Entidade:
		f.W, f.H = 120, 60
	case Relacionamento:
		f.W, f.H = 100, 60
	case Atributo:
		f.W, f.H = 80, 14
	}
	d.Formas = append(d.Formas, f)
	return f
}

func (d *Diagrama) Forma(id int) *Forma {
	for _, f := range d.Formas {
		if f.ID == id {
			return f
		}
	}
	return nil
}

// Ligar connects two shapes following the ER rules brModelo enforces.
func (d *Diagrama) Ligar(a, b *Forma) (*Ligacao, error) {
	if a == b {
		return nil, fmt.Errorf("não é possível ligar um item a ele mesmo")
	}
	if b.Tipo == Atributo { // atributo always on PontaA, like the Java app
		a, b = b, a
	}
	card := -1
	switch {
	case a.Tipo == Atributo && b.Tipo == Atributo:
		return nil, fmt.Errorf("não é possível ligar dois atributos")
	case a.Tipo == Atributo:
	case a.Tipo != b.Tipo:
		card = 3 // (0,n)
	default:
		return nil, fmt.Errorf("ligue %s a um atributo ou %s", a.Tipo, map[Tipo]Tipo{Entidade: Relacionamento, Relacionamento: Entidade}[a.Tipo])
	}
	l := &Ligacao{ID: d.newID(), A: a.ID, B: b.ID, Card: card}
	d.Ligacoes = append(d.Ligacoes, l)
	return l, nil
}

// Remove deletes a shape (and its links) or a link.
func (d *Diagrama) Remove(id int) {
	fs := d.Formas[:0]
	for _, f := range d.Formas {
		if f.ID != id {
			fs = append(fs, f)
		}
	}
	d.Formas = fs
	ls := d.Ligacoes[:0]
	for _, l := range d.Ligacoes {
		if l.ID != id && l.A != id && l.B != id {
			ls = append(ls, l)
		}
	}
	d.Ligacoes = ls
}

// ProximoID is the next free element ID (persistence writes cardinalities after it).
func (d *Diagrama) ProximoID() int { return max(d.nextID, 2) }

// DefinirProximoID is used by readers after loading elements with their own IDs.
func (d *Diagrama) DefinirProximoID(n int) { d.nextID = n }

// Copia is an independent deep copy (for undo snapshots).
func (d *Diagrama) Copia() *Diagrama {
	c := &Diagrama{nextID: d.nextID, Formas: make([]*Forma, len(d.Formas)), Ligacoes: make([]*Ligacao, len(d.Ligacoes))}
	for i, f := range d.Formas {
		g := *f
		c.Formas[i] = &g
	}
	for i, l := range d.Ligacoes {
		m := *l
		c.Ligacoes[i] = &m
	}
	return c
}

// Assinatura identifies the diagram's full state, positions included: two
// diagrams with the same signature look and behave the same.
func (d *Diagrama) Assinatura() string {
	var b strings.Builder
	for _, f := range d.Formas {
		fmt.Fprintf(&b, "%d|%s|%g,%g,%g,%g|%s|%t|%s\n", f.ID, f.Tipo, f.X, f.Y, f.W, f.H, f.Texto, f.Identificador, f.TipoAtributo)
	}
	for _, l := range d.Ligacoes {
		fmt.Fprintf(&b, "L%d|%d|%d|%d\n", l.ID, l.A, l.B, l.Card)
	}
	return b.String()
}
