package dominio

import "testing"

func campos(t *Tabela) (r []string) {
	for _, c := range t.Campos {
		r = append(r, c.Nome)
	}
	return
}

func ligar(t *testing.T, d *Diagrama, a, b *Forma, card int) {
	l, err := d.Ligar(a, b)
	if err != nil {
		t.Fatal(err)
	}
	l.Card = card
}

func TestConverter(t *testing.T) {
	d := &Diagrama{}
	dep := d.Add(Entidade, 0, 0, "Departamento")
	emp := d.Add(Entidade, 300, 0, "Empregado")
	prj := d.Add(Entidade, 300, 300, "Projeto")
	lot := d.Add(Relacionamento, 150, 0, "lotacao")
	atua := d.Add(Relacionamento, 300, 150, "atua")
	for _, p := range []struct {
		e    *Forma
		nome string
	}{{dep, "cod"}, {emp, "mat"}, {prj, "id"}} {
		a := d.Add(Atributo, 0, 0, p.nome)
		a.Identificador, a.TipoAtributo = true, "INT"
		ligar(t, d, p.e, a, -1)
	}
	ligar(t, d, emp, d.Add(Atributo, 0, 0, "nome"), -1)
	ligar(t, d, atua, d.Add(Atributo, 0, 0, "horas"), -1)
	ligar(t, d, dep, lot, 0) // (1,1)
	ligar(t, d, emp, lot, 3) // (0,n)
	ligar(t, d, emp, atua, 3)
	ligar(t, d, prj, atua, 3)

	l, erros := Converter(d)
	if len(erros) > 0 {
		t.Fatal(erros)
	}
	if len(l.Tabelas) != 4 || len(l.Linhas) != 3 {
		t.Fatalf("%d tabelas %d linhas", len(l.Tabelas), len(l.Linhas))
	}
	// 1:N -> FK in Empregado
	e := l.Tabelas[1]
	if got := campos(e); len(got) != 3 || got[2] != "fk_Departamento_cod" || !e.Campos[2].Fkey || e.Campos[0].Tipo != "INT" || e.Campos[2].Tipo != "INT" {
		t.Fatalf("Empregado: %v", got)
	}
	if len(e.FKs) != 1 || e.FKs[0].Ref.Nome != "Departamento" || e.FKs[0].OnDelete != "CASCADE" || e.FKs[0].Linha == nil {
		t.Fatalf("Empregado FK: %+v", e.FKs)
	}
	// N:N -> new table with both FKs and the relationship's attribute
	at := l.Tabelas[3]
	if at.Nome != "atua" || len(at.FKs) != 2 {
		t.Fatalf("atua: %s %v %d", at.Nome, campos(at), len(at.FKs))
	}
	if got := campos(at); got[0] != "fk_Empregado_mat" || got[1] != "fk_Projeto_id" || got[2] != "horas" {
		t.Fatalf("atua campos: %v", got)
	}

}

func TestConverterMerge11(t *testing.T) {
	d := &Diagrama{}
	p := d.Add(Entidade, 0, 0, "Pessoa")
	c := d.Add(Entidade, 200, 0, "CPF")
	r := d.Add(Relacionamento, 100, 0, "tem")
	ligar(t, d, p, d.Add(Atributo, 0, 0, "nome"), -1)
	ligar(t, d, c, d.Add(Atributo, 0, 0, "numero"), -1)
	ligar(t, d, p, r, 0)
	ligar(t, d, c, r, 0)
	l, _ := Converter(d)
	if len(l.Tabelas) != 1 || l.Tabelas[0].Nome != "Pessoa_CPF" || len(l.Tabelas[0].Campos) != 2 || l.Tabelas[0].X != 100 {
		t.Fatalf("%+v", l.Tabelas[0])
	}
}

func TestConverterTernario(t *testing.T) {
	d := &Diagrama{}
	r := d.Add(Relacionamento, 50, 50, "venda")
	for _, n := range []string{"Cliente", "Produto", "Loja"} {
		e := d.Add(Entidade, 0, 0, n)
		a := d.Add(Atributo, 0, 0, "id")
		a.Identificador = true
		ligar(t, d, e, a, -1)
		ligar(t, d, e, r, 3)
	}
	l, _ := Converter(d)
	v := l.Tabelas[3]
	if v.Nome != "venda_Cliente_Produto_Loja" || len(v.FKs) != 3 || len(l.Linhas) != 3 || v.Campos[2].Nome != "fk_Loja_id" {
		t.Fatalf("%s %v", v.Nome, campos(v))
	}
}

func TestConverterAutoRelacionamento(t *testing.T) {
	novo := func(c1, c2 int) *Logico {
		d := &Diagrama{}
		e := d.Add(Entidade, 0, 0, "Pessoa")
		a := d.Add(Atributo, 0, 0, "cpf")
		a.Identificador = true
		ligar(t, d, e, a, -1)
		r := d.Add(Relacionamento, 0, 0, "casa")
		ligar(t, d, e, r, c1)
		ligar(t, d, e, r, c2)
		l, erros := Converter(d)
		if len(erros) > 0 {
			t.Fatal(erros)
		}
		return l
	}
	// (0,1):(0,1) -> self FK column in the same table
	l := novo(1, 1)
	p := l.Tabelas[0]
	if len(l.Tabelas) != 1 || len(p.Campos) != 2 || p.Campos[1].Nome != "fk_Pessoa_cpf" || len(p.FKs) != 1 || p.FKs[0].Ref != p {
		t.Fatalf("%v %+v", campos(p), p.FKs)
	}
	// (0,n):(0,n) -> new table with the key twice, two FKs
	l = novo(3, 3)
	n := l.Tabelas[1]
	if len(l.Tabelas) != 2 || n.Nome != "casa" || len(n.Campos) != 2 || len(n.FKs) != 2 || len(l.Linhas) != 1 {
		t.Fatalf("%s %v %d FKs", n.Nome, campos(n), len(n.FKs))
	}
	if n.Campos[0].Nome == n.Campos[1].Nome {
		t.Fatalf("duplicate field names %v", campos(n))
	}
}

func TestOrganizar(t *testing.T) {
	l := &Logico{}
	a := l.AddTabela("A", 0, 0)
	b := l.AddTabela("B", 100, 10) // overlapping, same row
	c := l.AddTabela("C", 0, 40)   // overlapping, same column
	if !l.Organizar() {
		t.Fatal("should move")
	}
	for _, p := range [][2]*Tabela{{a, b}, {a, c}, {b, c}} {
		x, y := p[0], p[1]
		gapX := max(y.X-(x.X+x.W), x.X-(y.X+y.W))
		gapY := max(y.Y-(x.Y+x.H), x.Y-(y.Y+y.H))
		if gapX < distLeft && gapY < distTop {
			t.Fatalf("still too close: %+v %+v", x, y)
		}
	}
	if l.Organizar() {
		t.Fatal("second run should be a no-op")
	}
}

func TestLigarRules(t *testing.T) {
	d := &Diagrama{}
	e := d.Add(Entidade, 0, 0, "E")
	e2 := d.Add(Entidade, 0, 0, "E2")
	r := d.Add(Relacionamento, 0, 0, "R")
	a := d.Add(Atributo, 0, 0, "a")
	b := d.Add(Atributo, 0, 0, "b")
	if l, err := d.Ligar(e, a); err != nil || l.A != a.ID || l.Card != -1 {
		t.Fatal("atributo must be PontaA, no card", l, err)
	}
	if l, err := d.Ligar(e, r); err != nil || l.Card != 3 {
		t.Fatal("entidade-relacionamento gets (0,n)", l, err)
	}
	for _, p := range [][2]*Forma{{a, b}, {e, e2}, {e, e}} {
		if _, err := d.Ligar(p[0], p[1]); err == nil {
			t.Fatal("should refuse", p[0].Tipo, p[1].Tipo)
		}
	}
	d.Remove(e.ID)
	if len(d.Ligacoes) != 0 {
		t.Fatal("links to removed shape must go")
	}
}

func TestCopiaEAssinatura(t *testing.T) {
	d := &Diagrama{}
	e := d.Add(Entidade, 10, 10, "E")
	a := d.Add(Atributo, 0, 0, "a")
	d.Ligar(e, a)
	c := d.Copia()
	if c.Assinatura() != d.Assinatura() {
		t.Fatal("copy differs")
	}
	c.Formas[0].X = 99
	c.Ligacoes[0].Card = 2
	if d.Formas[0].X != 10 || d.Ligacoes[0].Card != -1 {
		t.Fatal("copy shares memory with the original")
	}
	if c.Assinatura() == d.Assinatura() {
		t.Fatal("a moved shape must change the signature")
	}
	if n := c.Add(Entidade, 0, 0, "F"); n.ID != d.Add(Entidade, 0, 0, "F").ID {
		t.Fatal("copy must continue the same ID sequence")
	}
}
