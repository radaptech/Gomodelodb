// Package xmlbrm reads and writes diagrams in the XML format of brModelo's
// Java app (util/XMLGenerate.java), so files open in both versions.
package xmlbrm

import (
	"encoding/xml"
	"fmt"
	"io"
	"strconv"
	"strings"

	dom "github.com/brmodelo/gobrmodelo/internal/dominio"
)

// ---- XML ----

type node struct {
	XMLName xml.Name
	Attrs   []xml.Attr `xml:",any,attr"`
	Text    string     `xml:",chardata"`
	Nodes   []node     `xml:",any"`
}

func (n node) attr(name string) string {
	for _, a := range n.Attrs {
		if a.Name.Local == name {
			return a.Value
		}
	}
	return ""
}

func (n node) child(name string) node {
	for _, c := range n.Nodes {
		if c.XMLName.Local == name {
			return c
		}
	}
	return node{}
}

func (n node) num(name, attr string) float32 {
	v, _ := strconv.Atoi(n.child(name).attr(attr))
	return float32(v)
}

// LerConceitual reads a brModelo conceptual XML. Elements this port doesn't know
// (Especializacao, Uniao, EntidadeAssociativa, Texto...) are skipped.
func LerConceitual(r io.Reader) (*dom.Diagrama, error) {
	root, err := parse(r)
	if err != nil {
		return nil, err
	}
	return conceitualFrom(root)
}

func parse(r io.Reader) (root node, err error) {
	err = xml.NewDecoder(r).Decode(&root)
	return
}

// Abrir reads either kind of brModelo XML: returns *dom.Diagrama or *dom.Logico.
func Abrir(r io.Reader) (any, error) {
	root, err := parse(r)
	if err != nil {
		return nil, err
	}
	if root.attr("TIPO") == "tpLogico" {
		return logicoFrom(root)
	}
	return conceitualFrom(root)
}

func conceitualFrom(root node) (*dom.Diagrama, error) {
	if t := root.attr("TIPO"); t != "tpConceitual" {
		return nil, fmt.Errorf("tipo de diagrama não suportado: %q", t)
	}
	d := &dom.Diagrama{}
	maxID := 1
	for _, n := range root.Nodes {
		id, _ := strconv.Atoi(n.attr("ID"))
		maxID = max(maxID, id)
		switch t := dom.Tipo(n.XMLName.Local); t {
		case dom.Entidade, dom.Relacionamento, dom.Atributo:
			d.Formas = append(d.Formas, &dom.Forma{
				ID: id, Tipo: t, Texto: strings.TrimSpace(n.child("Texto").Text),
				X: n.num("Bounds", "Left"), Y: n.num("Bounds", "Top"),
				W: n.num("Bounds", "Width"), H: n.num("Bounds", "Height"),
				Identificador: n.child("Identificador").attr("Valor") == "true",
				TipoAtributo:  n.child("TipoAtributo").attr("Valor"),
			})
		case "Ligacao":
			a, _ := strconv.Atoi(n.child("Ligacoes").attr("PontaA"))
			b, _ := strconv.Atoi(n.child("Ligacoes").attr("PontaB"))
			l := &dom.Ligacao{ID: id, A: a, B: b, Card: -1}
			if c := n.child("Cardinalidade"); c.XMLName.Local != "" {
				l.Card, _ = strconv.Atoi(c.child("Card").attr("Valor"))
				maxID = max(maxID, atoi(c.attr("ID")))
			}
			d.Ligacoes = append(d.Ligacoes, l)
		}
	}
	// drop links whose ends we skipped
	ls := d.Ligacoes[:0]
	for _, l := range d.Ligacoes {
		if d.Forma(l.A) != nil && d.Forma(l.B) != nil {
			ls = append(ls, l)
		}
	}
	d.Ligacoes = ls
	d.DefinirProximoID(maxID + 1)
	return d, nil
}

func atoi(s string) int { v, _ := strconv.Atoi(s); return v }

func esc(s string) string {
	var b strings.Builder
	xml.EscapeText(&b, []byte(s))
	return b.String()
}

func bounds(x, y, w, h float32) string {
	return fmt.Sprintf(`<Bounds Height="%d" Left="%d" Top="%d" Width="%d"/>`, int(h), int(x), int(y), int(w))
}

const fonte = `<Fonte Estilo="1" Nome="Arial" Tamanho="12"/>`

// SalvarConceitual writes the diagram so the Java brModelo can open it.
func SalvarConceitual(w io.Writer, d *dom.Diagrama) error {
	var b strings.Builder
	b.WriteString(`<?xml version="1.0" encoding="UTF-8" standalone="no"?>` + "\n")
	b.WriteString(`<DIAGRAMA ID="1" TIPO="tpConceitual" UniversalUnicID="">` + "\n")
	for _, f := range d.Formas {
		fmt.Fprintf(&b, `<%s ID="%d">%s<Texto>%s</Texto><Observacao/><Dicionario/>%s<Ancorado Valor="false"/>`,
			f.Tipo, f.ID, bounds(f.X, f.Y, f.W, f.H), esc(f.Texto), fonte)
		switch f.Tipo {
		case dom.Entidade:
			b.WriteString(`<AtributosOcultos Valor=""/>`)
		case dom.Atributo:
			fmt.Fprintf(&b, `<DirecaoFromInspector Valor="0"/><Autosize Valor="true"/><Identificador Valor="%t"/><Opcional Valor="false"/><Multivalorado Valor="false"/><CardMinFromString Valor="1"/><CardMaxFromString Valor="n"/><TipoAtributo Valor="%s"/>`, f.Identificador, esc(f.TipoAtributo))
		}
		fmt.Fprintf(&b, "</%s>\n", f.Tipo)
	}
	cardID := d.ProximoID() + len(d.Ligacoes) + 1 // cardinalities need IDs too; keep clear of shape IDs
	for _, l := range d.Ligacoes {
		a, c := d.Forma(l.A), d.Forma(l.B)
		ax, ay := a.X+a.W/2, a.Y+a.H/2
		cx, cy := c.X+c.W/2, c.Y+c.H/2
		fmt.Fprintf(&b, `<Ligacao ID="%d"><Tag LinhaMestre="-1"/><Dashed Valor="false"/><Ancorado Valor="false"/><TemSetaPontaA Valor="false"/><TemSetaPontaB Valor="false"/><SetaAberta Valor="true"/><SetaLargura Valor="10"/><Inteligente Valor="true"/><Largura Valor="1"/><Ligacoes PontaA="%d" PontaB="%d"/><Pontos><Ponto Left="%d" Top="%d"/><Ponto Left="%d" Top="%d"/></Pontos>`,
			l.ID, l.A, l.B, int(ax), int(ay), int(cx), int(cy))
		if l.Card >= 0 {
			fmt.Fprintf(&b, `<Cardinalidade ID="%d">%s<Texto>%s</Texto><Observacao/><Dicionario/>%s<Ancorado Valor="false"/><TamanhoAutmatico Valor="true"/><Card Valor="%d"/><MovimentacaoManual Valor="false"/><Papel Valor=""/></Cardinalidade>`,
				cardID, bounds((ax+cx)/2, (ay+cy)/2, 30, 14), dom.Cards[l.Card], fonte, l.Card)
			cardID++
		}
		b.WriteString("</Ligacao>\n")
	}
	b.WriteString("</DIAGRAMA>\n")
	_, err := io.WriteString(w, b.String())
	return err
}

// ---- XML ----

// LerLogico reads a brModelo logical XML.
func LerLogico(r io.Reader) (*dom.Logico, error) {
	root, err := parse(r)
	if err != nil {
		return nil, err
	}
	return logicoFrom(root)
}

func logicoFrom(root node) (*dom.Logico, error) {
	if t := root.attr("TIPO"); t != "tpLogico" {
		return nil, fmt.Errorf("tipo de diagrama não suportado: %q", t)
	}
	l := &dom.Logico{}
	maxID := 1
	type fkRef struct {
		fk         *dom.FK
		tab, linha int
	}
	var refs []fkRef
	for _, n := range root.Nodes {
		id := atoi(n.attr("ID"))
		maxID = max(maxID, id)
		switch n.XMLName.Local {
		case "Tabela":
			t := &dom.Tabela{ID: id, Nome: strings.TrimSpace(n.child("Texto").Text),
				X: n.num("Bounds", "Left"), Y: n.num("Bounds", "Top"), W: n.num("Bounds", "Width"), H: n.num("Bounds", "Height")}
			for _, c := range n.child("Campos").Nodes {
				t.Campos = append(t.Campos, &dom.Campo{Nome: c.child("Texto").Text, Tipo: c.child("Tipo").Text,
					Key: c.child("Key").attr("Valor") == "true", Fkey: c.child("Fkey").attr("Valor") == "true"})
			}
			for _, c := range n.child("Constraints").Nodes {
				if c.child("Tipo").attr("Valor") != "2" {
					continue // PK/UNIQUE are derived from dom.Campo.Key
				}
				o := c.child("ConstraintOrigem")
				fk := &dom.FK{OnDelete: c.child("DdlOnDelete").Text}
				ori, dst := lista(o.attr("CamposOrigem")), lista(o.attr("CamposDestino"))
				for i := range min(len(ori), len(dst)) {
					fk.Pares = append(fk.Pares, [2]int{ori[i], dst[i]})
				}
				t.FKs = append(t.FKs, fk)
				refs = append(refs, fkRef{fk, atoi(strings.Split(o.attr("ID"), ",")[0]), atoi(c.child("LigacaoParaOrigem").attr("ID"))})
			}
			l.Tabelas = append(l.Tabelas, t)
		case "LogicoLinha":
			ln := &dom.Linha{ID: id, A: &dom.Tabela{ID: atoi(n.child("Ligacoes").attr("PontaA"))}, B: &dom.Tabela{ID: atoi(n.child("Ligacoes").attr("PontaB"))}}
			var cards []int
			for _, c := range n.Nodes {
				if c.XMLName.Local == "LogicoCardinalidade" {
					cards = append(cards, atoi(c.child("Card").attr("Valor")))
					maxID = max(maxID, atoi(c.attr("ID")))
				}
			}
			if len(cards) == 2 {
				ln.CardA, ln.CardB = cards[0], cards[1]
			}
			l.Linhas = append(l.Linhas, ln)
		}
	}
	// resolve references now that every table is known
	ls := l.Linhas[:0]
	for _, ln := range l.Linhas {
		ln.A, ln.B = l.Tabela(ln.A.ID), l.Tabela(ln.B.ID)
		if ln.A != nil && ln.B != nil {
			ls = append(ls, ln)
		}
	}
	l.Linhas = ls
	for _, r := range refs {
		r.fk.Ref = l.Tabela(r.tab)
		for _, ln := range l.Linhas {
			if ln.ID == r.linha {
				r.fk.Linha = ln
			}
		}
	}
	for _, t := range l.Tabelas {
		fks := t.FKs[:0]
		for _, fk := range t.FKs {
			if fk.Ref != nil {
				fks = append(fks, fk)
			}
		}
		t.FKs = fks
	}
	l.DefinirProximoID(maxID + 1)
	return l, nil
}

// lista parses Java's Arrays.toString: "[0, 1]".
func lista(s string) []int {
	var r []int
	for _, p := range strings.Split(strings.Trim(s, "[] "), ",") {
		if p = strings.TrimSpace(p); p != "" {
			r = append(r, atoi(p))
		}
	}
	return r
}

func javaLista(v []int) string {
	s := make([]string, len(v))
	for i, x := range v {
		s[i] = strconv.Itoa(x)
	}
	return "[" + strings.Join(s, ", ") + "]"
}

const constraintIni = `<Constraint><Nome Valor=""/><Nomeada Valor="false"/><Tipo Valor="%d"/><Dicionario/><Observacao/><DdlOnUpdate/><DdlOnDelete>%s</DdlOnDelete><LigacaoParaOrigem ID="%d"/><ConstraintOrigem CamposDestino="%s" CamposOrigem="%s" ID="%s"/></Constraint>`

// SalvarLogico writes a logical diagram in brModelo's XML.
func SalvarLogico(w io.Writer, l *dom.Logico) error {
	var b strings.Builder
	b.WriteString(`<?xml version="1.0" encoding="UTF-8" standalone="no"?>` + "\n")
	b.WriteString(`<DIAGRAMA ID="1" TIPO="tpLogico" UniversalUnicID="">` + "\n")
	for _, t := range l.Tabelas {
		fmt.Fprintf(&b, `<Tabela ID="%d">%s<Texto>%s</Texto><Observacao/><Dicionario/>%s<Ancorado Valor="false"/><Delimite Valor="true"/><Roundrect Valor="22"/><Gradiente Valor="true"/><GradienteStartColor Valor="0,0,0,255"/><GradienteEndColor Valor="204,204,204,255"/><CorBorda Valor="0,0,0,255"/><GDirecao Valor="0"/><Alfa Valor="25"/><Autosize Valor="true"/><ShowDDL Valor="false"/><ShowInPlain Valor="false"/><MostrarConstraints Valor="true"/><PlainIR Valor="false"/><Campos>`,
			t.ID, bounds(t.X, t.Y, t.W, t.H), esc(t.Nome), fonte)
		for _, c := range t.Campos {
			fmt.Fprintf(&b, `<Campo><Texto>%s</Texto><Tipo>%s</Tipo><Complemento/><Dicionario/><Observacao/><Key Valor="%t"/><Fkey Valor="%t"/><Unique Valor="false"/></Campo>`,
				esc(c.Nome), esc(c.Tipo), c.Key, c.Fkey)
		}
		b.WriteString("</Campos><Constraints>")
		if pk := t.Chaves(); len(pk) > 0 { // always first, so FKs reference it as index 0
			nulos := make([]int, len(pk))
			for i := range nulos {
				nulos[i] = -1
			}
			fmt.Fprintf(&b, constraintIni, 0, "", -1, javaLista(nulos), javaLista(pk), "-1,-1")
		}
		for _, fk := range t.FKs {
			var ori, dst []int
			for _, p := range fk.Pares {
				ori, dst = append(ori, p[0]), append(dst, p[1])
			}
			linha, ref := -1, "-1,-1"
			if fk.Linha != nil {
				linha = fk.Linha.ID
			}
			if len(fk.Ref.Chaves()) > 0 {
				ref = fmt.Sprintf("%d,0", fk.Ref.ID)
			}
			fmt.Fprintf(&b, constraintIni, 2, esc(fk.OnDelete), linha, javaLista(dst), javaLista(ori), ref)
		}
		b.WriteString("</Constraints></Tabela>\n")
	}
	cardID := l.ProximoID() + 2*len(l.Linhas) + 1
	for _, ln := range l.Linhas {
		ax, ay := ln.A.X+ln.A.W/2, ln.A.Y+ln.A.H/2
		bx, by := ln.B.X+ln.B.W/2, ln.B.Y+ln.B.H/2
		// LogicoLinha.ajusteSeta
		setaA, setaB := ln.CardA >= ln.CardB, ln.CardA <= ln.CardB
		fmt.Fprintf(&b, `<LogicoLinha ID="%d"><Tag LinhaMestre="-1"/><Dashed Valor="false"/><Ancorado Valor="false"/><TemSetaPontaA Valor="%t"/><TemSetaPontaB Valor="%t"/><SetaAberta Valor="true"/><SetaLargura Valor="10"/><Inteligente Valor="true"/><SetaAutomatica Valor="true"/><Largura Valor="1"/><Ligacoes PontaA="%d" PontaB="%d"/><Pontos><Ponto Left="%d" Top="%d"/><Ponto Left="%d" Top="%d"/></Pontos>`,
			ln.ID, setaA, setaB, ln.A.ID, ln.B.ID, int(ax), int(ay), int(bx), int(by))
		for i, c := range []int{ln.CardA, ln.CardB} {
			fmt.Fprintf(&b, `<LogicoCardinalidade ID="%d">%s<Texto>Cardinalidade_%d</Texto><Observacao/><Dicionario/>%s<Ancorado Valor="false"/><TamanhoAutmatico Valor="true"/><Card Valor="%d"/><MovimentacaoManual Valor="false"/><Papel Valor=""/></LogicoCardinalidade>`,
				cardID, bounds([]float32{ax, bx}[i], []float32{ay, by}[i], 35, 15), i+1, fonte, c)
			cardID++
		}
		b.WriteString("</LogicoLinha>\n")
	}
	b.WriteString("</DIAGRAMA>\n")
	_, err := io.WriteString(w, b.String())
	return err
}
