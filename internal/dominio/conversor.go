package dominio

import "strings"

// Port of controlador/conversor/conversorConceitualParaLogico.java.
// The Java converter asks questions on ambiguous cases; this one always takes
// the option the Java dialog pre-selects (Opcoes.opcDefault).

const fkPrefixo = "FK_"

type ponta struct {
	e    *Forma
	card int
}

type conversor struct {
	d      *Diagrama
	l      *Logico
	tab    map[int]*Tabela    // Entidade/Relacionamento ID -> Tabela (Links)
	origem map[*Campo]*Campo  // FK field -> PK field it copies (camposOrigem)
	fkDe   map[*Linha]*Tabela // which end of a line receives the FK (origemLigacao)
	erros  []string
}

// Converter turns a conceptual diagram into a logical one. Erros lists what was skipped.
func Converter(d *Diagrama) (l *Logico, erros []string) {
	c := &conversor{d: d, l: &Logico{}, tab: map[int]*Tabela{}, origem: map[*Campo]*Campo{}, fkDe: map[*Linha]*Tabela{}}
	for _, e := range d.Formas {
		if e.Tipo == Entidade {
			t := c.l.AddTabela(e.Texto, e.X, e.Y)
			c.tab[e.ID] = t
			c.atributos(e, t)
		}
	}
	// Java converts self-relationships first (converterAutoRelacionamento)
	for _, auto := range []bool{true, false} {
		for _, r := range d.Formas {
			if r.Tipo == Relacionamento && c.autoRel(r) == auto {
				c.relacionamento(r)
			}
		}
	}
	c.constraints()
	c.renomeieFKs()
	return c.l, c.erros
}

func (c *conversor) vizinhos(f *Forma) (ents []ponta, attrs []*Forma) {
	for _, l := range c.d.Ligacoes {
		o := l.A
		if o == f.ID {
			o = l.B
		} else if l.B != f.ID {
			continue
		}
		switch v := c.d.Forma(o); v.Tipo {
		case Atributo:
			attrs = append(attrs, v)
		case Entidade:
			ents = append(ents, ponta{v, l.Card})
		}
	}
	return
}

func (c *conversor) atributos(f *Forma, t *Tabela) {
	_, attrs := c.vizinhos(f)
	for _, a := range attrs {
		t.Add(a.Texto, a.TipoAtributo, a.Identificador)
	}
}

func prefixo(nome string) string { // Utilidades.IsUpper decides FK_ vs fk_
	if nome == strings.ToUpper(nome) {
		return fkPrefixo
	}
	return strings.ToLower(fkPrefixo)
}

func (c *conversor) relacionamento(r *Forma) {
	ents, _ := c.vizinhos(r)
	if c.autoRel(r) {
		c.autoRelacionamento(r, ents[0].e, ents[0].card, ents[1].card)
		return
	}
	if len(ents) < 2 {
		c.erros = append(c.erros, "Relacionamento ignorado (menos de duas entidades ligadas): "+r.Texto)
		return
	}
	todos11 := true
	for _, p := range ents {
		todos11 = todos11 && p.card == 0
	}
	switch {
	case todos11:
		c.merge(r, ents)
	case len(ents) > 2:
		c.ternario(r, ents)
	default:
		c.binario(r, ents[0], ents[1])
	}
}

// autoRel: linked twice to the same entity and nothing else (PreRelacionamento.isAutoRelacionamento).
func (c *conversor) autoRel(r *Forma) bool {
	ents, _ := c.vizinhos(r)
	return len(ents) == 2 && ents[0].e == ents[1].e
}

func (c *conversor) autoRelacionamento(r, e *Forma, c1, c2 int) {
	c1, c2 = min(c1, c2), max(c1, c2)
	t := c.tab[e.ID]
	_, attrs := c.vizinhos(r)
	// Java's default answer: a self-FK column when one side is at most 1, else a new table
	if c2 == 1 || (c1 <= 1 && c2 == 2) {
		for _, k := range t.Campos {
			if k.Key {
				n := t.Add(t.Nome+"_"+k.Nome, k.Tipo, false)
				n.Fkey = true
				c.origem[n] = k
			}
		}
	} else {
		n := c.l.AddTabela(r.Texto, t.X+t.W+120, t.Y)
		c.link(t, n, 0, c2, n)                       // recursion always has min card (1,1)
		for _, pre := range []string{"_A_", "_B_"} { // the key twice, one per role
			for _, k := range t.Campos {
				if k.Key {
					f := n.Add(t.Nome+pre+k.Nome, k.Tipo, false)
					f.Fkey = true
					c.origem[f] = k
				}
			}
		}
		t = n
		c.tab[r.ID] = n
	}
	for _, a := range attrs {
		t.Add(a.Texto, a.TipoAtributo, a.Identificador)
	}
}

// importaChave copies ori's key fields into dest as FK (ImportaCampoChave).
func (c *conversor) importaChave(ori, dest *Tabela, pre string) {
	if ori == dest {
		return
	}
	for _, k := range ori.Campos {
		if k.Key {
			n := dest.Add(pre+k.Nome, k.Tipo, false)
			n.Fkey = true
			c.origem[n] = k
		}
	}
}

func (c *conversor) link(a, b *Tabela, cardA, cardB int, fk *Tabela) {
	c.fkDe[c.l.Ligar(a, b, cardA, cardB)] = fk
}

func (c *conversor) binario(r *Forma, p1, p2 ponta) {
	if p2.card < p1.card {
		p1, p2 = p2, p1
	}
	t1, t2 := c.tab[p1.e.ID], c.tab[p2.e.ID]
	_, attrs := c.vizinhos(r)
	c1, c2 := p1.card, p2.card
	if (c1 < 2 && c2 > 1) || (c1 == 0 && c2 == 1) {
		// 1:N or 1:1 with one mandatory side: FK goes to t2 (default option 0)
		if t1 == t2 {
			c.erros = append(c.erros, "Relacionamento entre a mesma tabela ignorado: "+r.Texto)
			return
		}
		c.importaChave(t1, t2, prefixo(p1.e.Texto)+p1.e.Texto+"_")
		c.link(t1, t2, c1, c2, t2)
		for _, a := range attrs { // Java default: relationship attrs go with the FK
			t2.Add(a.Texto, a.TipoAtributo, a.Identificador)
		}
		return
	}
	// N:N or (0,1):(0,1): new table
	nome := r.Texto
	if nome == "" {
		nome = p1.e.Texto + "_" + p2.e.Texto
	}
	prin := c.l.AddTabela(nome, r.X, r.Y)
	pf := prefixo(prin.Nome)
	for _, p := range []ponta{{p1.e, c1}, {p2.e, c2}} {
		t := c.tab[p.e.ID]
		lado := 1 // (0,1)
		if p.card == 0 || p.card == 2 {
			lado = 0 // (1,1)
		}
		c.link(prin, t, p.card, lado, prin)
		c.importaChave(t, prin, pf+t.Nome+"_")
	}
	for _, a := range attrs {
		prin.Add(a.Texto, a.TipoAtributo, a.Identificador)
	}
	c.tab[r.ID] = prin
}

func (c *conversor) ternario(r *Forma, ents []ponta) {
	prin := c.l.AddTabela(r.Texto, r.X, r.Y)
	for _, p := range ents {
		t := c.tab[p.e.ID]
		lado := 0 // (1,1)
		if p.card == 1 || p.card == 3 {
			lado = 1 // (0,1)
		}
		c.link(prin, t, lado, p.card, prin)
		c.importaChave(t, prin, t.Nome+"_")
		prin.Nome += "_" + t.Nome
	}
	// ponytail: Java drops a ternary relationship's own attributes; we keep them
	_, attrs := c.vizinhos(r)
	for _, a := range attrs {
		prin.Add(a.Texto, a.TipoAtributo, a.Identificador)
	}
	c.tab[r.ID] = prin
}

// merge: every side is (1,1) -> one table (converterRelacionamentoMerge).
func (c *conversor) merge(r *Forma, ents []ponta) {
	var tabs []*Tabela
	for _, p := range ents {
		if t := c.tab[p.e.ID]; !contem(tabs, t) {
			tabs = append(tabs, t)
		}
	}
	prin := tabs[0]
	_, attrs := c.vizinhos(r)
	for _, a := range attrs {
		prin.Add(a.Texto, a.TipoAtributo, a.Identificador)
	}
	x, y := prin.X, prin.Y
	for _, t := range tabs[1:] {
		novo := map[*Campo]*Campo{}
		for _, a := range t.Campos {
			n := prin.Add(a.Nome, a.Tipo, a.Key)
			n.Fkey = a.Fkey
			novo[a] = n
			if o, ok := c.origem[a]; ok {
				c.origem[n] = o
			}
		}
		for k, o := range c.origem { // FKs that pointed at t now point at prin
			if n, ok := novo[o]; ok {
				c.origem[k] = n
			}
		}
		for id, tt := range c.tab {
			if tt == t {
				c.tab[id] = prin
			}
		}
		c.moverLinhas(t, prin)
		c.l.Tabelas = remove(c.l.Tabelas, t)
		x, y = x+t.X, y+t.Y
		prin.Nome += "_" + t.Nome
	}
	n := float32(len(tabs))
	prin.X, prin.Y = x/n, y/n
	c.tab[r.ID] = prin
}

func (c *conversor) moverLinhas(de, para *Tabela) {
	ls := c.l.Linhas[:0]
	for _, ln := range c.l.Linhas {
		if ln.A == de {
			ln.A = para
		}
		if ln.B == de {
			ln.B = para
		}
		if c.fkDe[ln] == de {
			c.fkDe[ln] = para
		}
		dup := false // RemovaLigacoesIguais
		for _, o := range ls {
			dup = dup || (o.A == ln.A && o.B == ln.B && o.CardA == ln.CardA && o.CardB == ln.CardB)
		}
		if ln.A != ln.B && !dup {
			ls = append(ls, ln)
		}
	}
	c.l.Linhas = ls
}

func contem(ts []*Tabela, t *Tabela) bool {
	for _, x := range ts {
		if x == t {
			return true
		}
	}
	return false
}

func remove(ts []*Tabela, t *Tabela) []*Tabela {
	r := ts[:0]
	for _, x := range ts {
		if x != t {
			r = append(r, x)
		}
	}
	return r
}

func (c *conversor) dono(k *Campo) *Tabela {
	for _, t := range c.l.Tabelas {
		if t.idx(k) >= 0 {
			return t
		}
	}
	return nil
}

// constraints builds the FKs (processeConstraints + condicaoIR).
func (c *conversor) constraints() {
	for _, t := range c.l.Tabelas {
		for i, k := range t.Campos {
			o, ok := c.origem[k]
			if !ok {
				continue
			}
			pk := c.dono(o)
			var fk *FK
			for _, f := range t.FKs {
				if f.Ref == pk && !usa(f, pk.idx(o)) {
					fk = f
				}
			}
			if fk == nil {
				fk = &FK{Ref: pk}
				for _, ln := range c.l.Linhas {
					if c.fkDe[ln] == t && (ln.A == pk || ln.B == pk) {
						fk.Linha = ln
					}
				}
				fk.OnDelete = onDelete(fk.Linha, t)
				t.FKs = append(t.FKs, fk)
			}
			fk.Pares = append(fk.Pares, [2]int{pk.idx(o), i})
		}
	}
}

func onDelete(ln *Linha, fk *Tabela) string {
	if ln == nil {
		return ""
	}
	a, b := ln.CardA, ln.CardB
	if ln.B != fk {
		a, b = b, a
	}
	switch {
	case a == 1 && (b == 3 || b == 1):
		return "SET NULL"
	case a == 1:
		return "CASCADE"
	case a == 0 && (b == 3 || b == 1):
		return "CASCADE"
	case a == 0:
		return "RESTRICT"
	}
	return "NO ACTION"
}

// renomeieFKs: FK fields become FK_<tabela origem>_<campo origem>.
func (c *conversor) renomeieFKs() {
	for _, t := range c.l.Tabelas {
		usados := map[string]bool{}
		for _, k := range t.Campos {
			o, ok := c.origem[k]
			if !ok {
				continue
			}
			n := prefixo(t.Nome) + c.dono(o).Nome + "_" + o.Nome
			for usados[n] {
				n += "_"
			}
			usados[n] = true
			k.Nome = n
		}
	}
}

// usa: fk already maps this PK field; a second copy (self-relationship roles) needs its own FK.
func usa(fk *FK, ref int) bool {
	for _, p := range fk.Pares {
		if p[0] == ref {
			return true
		}
	}
	return false
}
