package dominio

import (
	"sort"
	"strconv"
)

// Logical (relational) diagram — diagramas/logico in the Java app.

type Campo struct {
	Nome, Tipo string
	Key, Fkey  bool
}

// FK mirrors a Constraint of type tpFK.
type FK struct {
	Ref      *Tabela
	Pares    [][2]int // {index in Ref.Campos, index in own Campos}
	Linha    *Linha
	OnDelete string
}

type Tabela struct {
	ID         int
	X, Y, W, H float32
	Nome       string
	Campos     []*Campo
	FKs        []*FK
}

// Linha is a LogicoLinha; CardA sits next to A, CardB next to B.
type Linha struct {
	ID           int
	A, B         *Tabela
	CardA, CardB int
}

type Logico struct {
	Tabelas []*Tabela
	Linhas  []*Linha
	nextID  int
}

func (l *Logico) newID() int {
	if l.nextID < 2 {
		l.nextID = 2
	}
	l.nextID++
	return l.nextID - 1
}

func (l *Logico) AddTabela(nome string, x, y float32) *Tabela {
	t := &Tabela{ID: l.newID(), Nome: nome, X: x, Y: y, W: 150}
	t.ajuste()
	l.Tabelas = append(l.Tabelas, t)
	return t
}

func (t *Tabela) ajuste() { t.H = 30 + 22*float32(len(t.Campos)) }

// Add appends a field, renaming it like Tabela.NomeieCampo (x, x_1, x_2...).
func (t *Tabela) Add(nome, tipo string, key bool) *Campo {
	n := nome
	for i := 1; t.campo(n) != nil; i++ {
		n = nome + "_" + strconv.Itoa(i)
	}
	c := &Campo{Nome: n, Tipo: tipo, Key: key}
	t.Campos = append(t.Campos, c)
	t.ajuste()
	return c
}

func (t *Tabela) campo(nome string) *Campo {
	for _, c := range t.Campos {
		if c.Nome == nome {
			return c
		}
	}
	return nil
}

func (t *Tabela) idx(c *Campo) int {
	for i, x := range t.Campos {
		if x == c {
			return i
		}
	}
	return -1
}

func (l *Logico) Ligar(a, b *Tabela, cardA, cardB int) *Linha {
	ln := &Linha{ID: l.newID(), A: a, B: b, CardA: cardA, CardB: cardB}
	l.Linhas = append(l.Linhas, ln)
	return ln
}

func (l *Logico) Tabela(id int) *Tabela {
	for _, t := range l.Tabelas {
		if t.ID == id {
			return t
		}
	}
	return nil
}

// Chaves returns the indexes of the key fields.
func (t *Tabela) Chaves() []int {
	var r []int
	for i, c := range t.Campos {
		if c.Key {
			r = append(r, i)
		}
	}
	return r
}

// Indice is the position of c among t's fields, or -1.
func (t *Tabela) Indice(c *Campo) int { return t.idx(c) }

// ProximoID / DefinirProximoID: see Diagrama.
func (l *Logico) ProximoID() int         { return max(l.nextID, 2) }
func (l *Logico) DefinirProximoID(n int) { l.nextID = n }

// Distances kept between tables by Organizar (DiagramaLogico.CONS_DISTANCIA_*).
const (
	distLeft = 160
	distTop  = 80
)

// Organizar is DiagramaLogico.OrganizeTabelas: tables that are too close get
// pushed right/down, one pass in list order and one sorted by X. W/H must be
// current. Returns whether anything moved.
func (l *Logico) Organizar() bool {
	moveu := false
	passada := func(ts []*Tabela) {
		for _, a := range ts {
			for _, b := range ts {
				if a != b && apliqueDistancia(a, b) {
					moveu = true
				}
			}
		}
	}
	passada(l.Tabelas)
	ord := append([]*Tabela(nil), l.Tabelas...)
	sort.SliceStable(ord, func(i, j int) bool { return ord[i].X < ord[j].X })
	passada(ord)
	return moveu
}

// mapaPosi is Forma.MapaPosi: where b sits relative to a.
//
//	1  2  3
//	0  a  4
//	7  6  5
func mapaPosi(a, b *Tabela) int {
	if abs(a.X-b.X) < max(a.W, b.W) { // same column
		if a.Y > b.Y {
			return 2
		}
		return 6
	}
	if abs(a.Y-b.Y) < max(a.H, b.H) { // same row
		if a.X > b.X {
			return 0
		}
		return 4
	}
	switch {
	case a.X < b.X && a.Y < b.Y:
		return 5
	case a.X < b.X:
		return 3
	case a.Y > b.Y:
		return 1
	}
	return 7
}

func abs(v float32) float32 { return max(v, -v) }

func apliqueDistancia(a, b *Tabela) bool {
	var x, y float32
	distX, distY := b.X-(a.X+a.W), b.Y-(a.Y+a.H)
	switch mapaPosi(a, b) {
	case 6:
		if distY < distTop {
			y = distTop - distY
		}
	case 5:
		if distY < distTop {
			y = distTop - distY
		}
		if distX < distLeft {
			x = distLeft - distX
		}
		if x > y { // move only the smaller amount
			x = 0
			if distX < distLeft/4 { // unless it's really close
				x = distLeft / 4
			}
		} else {
			y = 0
			if distY < distTop/4 {
				y = distTop / 4
			}
		}
	case 3, 4:
		if distX < distLeft {
			x = distLeft - distX
		}
	}
	b.X, b.Y = b.X+x, b.Y+y
	return x > 0 || y > 0
}
