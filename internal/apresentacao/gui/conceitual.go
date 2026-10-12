// Package gui is the desktop presentation: the ABNT-monograph window built
// with Fyne. It draws the session's diagram and turns clicks and drags into
// the application's use cases.
package gui

import (
	"encoding/xml"
	"errors"
	"fmt"
	"image/color"
	"io"
	"math"
	"strconv"
	"strings"
	"time"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/canvas"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/dialog"
	"fyne.io/fyne/v2/driver/desktop"
	"fyne.io/fyne/v2/layout"
	"fyne.io/fyne/v2/storage"
	"fyne.io/fyne/v2/widget"

	"github.com/brmodelo/gobrmodelo/internal/aplicacao"
	"github.com/brmodelo/gobrmodelo/internal/apresentacao/assistente"
	dom "github.com/brmodelo/gobrmodelo/internal/dominio"
	"github.com/brmodelo/gobrmodelo/internal/infra/segredos"
	"github.com/brmodelo/gobrmodelo/internal/infra/xmlbrm"
)

const (
	modoSel   = "Selecionar"
	modoLigar = "Ligar"
)

type board struct {
	widget.BaseWidget
	s          *aplicacao.Sessao
	win        fyne.Window
	modo       string
	sel        int // selected Forma or Ligacao ID, 0 = none
	ligarDe    *dom.Forma
	arrastou   bool
	movendo    []*dom.Forma  // shapes being dragged
	grupo      map[int]bool  // shapes picked with the selection rectangle
	emArea     bool          // drawing the selection rectangle
	areaIni    fyne.Position // its corners
	areaFim    fyne.Position
	onSel      func(novo bool) // selection changed; novo = item just created
	nota       string          // advisor's red-pen note in the margin
	notaEm     *dom.Forma      // the shape it circles
	notaGen    int             // so an expired timer can't erase a newer note
	erro       string          // file error to show on the card, once
	scroll     *container.Scroll
	orientador *orientador
	zm         *zoom
}

// erroAbrir explains in Portuguese why a file did not open, and what to do.
func erroAbrir(nome string, err error) error {
	var sintaxe *xml.SyntaxError
	motivo := "o arquivo não é um diagrama do brModelo em XML"
	switch {
	case errors.As(err, &sintaxe):
		motivo = fmt.Sprintf("o arquivo está incompleto ou corrompido (linha %d)", sintaxe.Line)
	case strings.HasPrefix(err.Error(), "tipo de diagrama"):
		motivo = "só os modelos conceitual e lógico são suportados"
	}
	return fmt.Errorf("Não foi possível abrir %s: %s. Abra outro arquivo ou salve-o de novo pelo brModelo.", nome, motivo)
}

// avisar writes a file error on the card in red instead of opening a dialog.
func (b *board) avisar(err error) {
	b.erro = err.Error()
	if b.onSel != nil {
		b.onSel(false)
	}
}

func newBoard(w fyne.Window, s *aplicacao.Sessao) *board {
	b := &board{s: s, win: w, modo: modoSel, zm: &zoom{z: 1}}
	b.ExtendBaseWidget(b)
	return b
}

// anotar circles f in red and writes s beside it; passageira notes fade after a few seconds.
func (b *board) anotar(s string, f *dom.Forma, passageira bool) {
	b.notaGen++
	b.nota, b.notaEm = s, f
	if gen := b.notaGen; passageira {
		time.AfterFunc(4*time.Second, func() {
			fyne.Do(func() {
				if b.notaGen == gen {
					b.nota, b.notaEm = "", nil
					b.Refresh()
				}
			})
		})
	}
}

// The domain works in dom.Ponto; Fyne in fyne.Position.
func pt(p fyne.Position) dom.Ponto  { return dom.Ponto{X: p.X, Y: p.Y} }
func pos(p dom.Ponto) fyne.Position { return fyne.NewPos(p.X, p.Y) }

func borda(c, p fyne.Position, hw, hh float32, losango bool) fyne.Position {
	return pos(dom.Borda(pt(c), pt(p), hw, hh, losango))
}

func (b *board) pontas(l *dom.Ligacao) (fyne.Position, fyne.Position) {
	pa, pb := b.s.D.Pontas(l)
	return pos(pa), pos(pb)
}

func (b *board) formaEm(p fyne.Position) *dom.Forma     { return b.s.D.FormaEm(pt(p)) }
func (b *board) ligacaoEm(p fyne.Position) *dom.Ligacao { return b.s.D.LigacaoEm(pt(p)) }

// selecionar changes the selection (dropping any group) and tells the properties panel.
func (b *board) selecionar(id int, novo bool) {
	if id != b.sel || novo || b.grupo != nil {
		b.sel, b.grupo = id, nil
		if b.onSel != nil {
			b.onSel(novo)
		}
	}
}

// selecionarArea picks every shape fully inside the rectangle p1-p2.
func (b *board) selecionarArea(p1, p2 fyne.Position) {
	x0, y0, x1, y1 := min(p1.X, p2.X), min(p1.Y, p2.Y), max(p1.X, p2.X), max(p1.Y, p2.Y)
	g := map[int]bool{}
	for _, f := range b.s.D.Formas {
		if f.X >= x0 && f.Y >= y0 && f.X+f.W <= x1 && f.Y+f.H <= y1 {
			g[f.ID] = true
		}
	}
	if len(g) == 1 {
		for id := range g {
			b.selecionar(id, false)
		}
		return
	}
	b.sel, b.grupo = 0, nil
	if len(g) > 1 {
		b.grupo = g
	}
	if b.onSel != nil {
		b.onSel(false)
	}
}

func (b *board) Tapped(ev *fyne.PointEvent) {
	p := b.zm.modelo(ev.Position)
	f := b.formaEm(p)
	switch b.modo {
	case modoSel:
		if f != nil {
			b.selecionar(f.ID, false)
		} else if l := b.ligacaoEm(p); l != nil {
			b.selecionar(l.ID, false)
		} else {
			b.selecionar(0, false)
		}
	case modoLigar:
		switch {
		case f == nil:
			b.ligarDe = nil
			b.anotar("", nil, false)
		case b.ligarDe == nil:
			b.ligarDe = f
			b.anotar("ligar a quê? clique no destino", f, false)
		default:
			b.anotar("", nil, false)
			b.s.Marcar()
			if r, err := b.s.Ligar(b.ligarDe, f); err != nil {
				b.anotar(err.Error(), f, true)
			} else if r != nil { // two entities: a relationship was created between them
				b.selecionar(r.ID, true)
			}
			b.ligarDe = nil
		}
	default: // add a shape
		b.s.Marcar()
		t := dom.Tipo(b.modo)
		var n *dom.Forma
		if t == dom.Atributo && f != nil && f.Tipo != dom.Atributo {
			n, _ = b.s.AdicionarAtributo(f, "", "", false, &dom.Ponto{X: p.X, Y: p.Y}) // dropped on a shape: attached to it
		} else {
			n = b.s.CriarForma(t, pt(p), "")
		}
		b.selecionar(n.ID, true)
	}
	b.Refresh()
}

// Dragged moves the shape under the pointer, the whole group if that shape is
// in it, or (on empty space, in Selecionar mode) draws a selection rectangle.
func (b *board) Dragged(ev *fyne.DragEvent) {
	pos, delta := b.zm.modelo(ev.Position), b.zm.modelo(fyne.NewPos(ev.Dragged.DX, ev.Dragged.DY))
	if !b.arrastou {
		b.arrastou = true
		ini := pos.Subtract(delta)
		f := b.formaEm(ini)
		switch {
		case f != nil && b.grupo[f.ID]:
			for _, o := range b.s.D.Formas {
				if b.grupo[o.ID] {
					b.movendo = append(b.movendo, o)
				}
			}
		case f != nil:
			b.selecionar(f.ID, false)
			b.movendo = []*dom.Forma{f}
		case b.modo == modoSel:
			b.emArea, b.areaIni = true, ini
		}
		if b.movendo != nil {
			b.s.Marcar() // a whole drag is one undo step
		}
	}
	if b.emArea {
		b.areaFim = pos
		b.Refresh()
		return
	}
	for _, f := range b.movendo {
		f.X += delta.X
		f.Y += delta.Y
	}
	if b.movendo != nil {
		b.Refresh()
	}
}

func (b *board) DragEnd() {
	if b.emArea {
		b.selecionarArea(b.areaIni, b.areaFim)
	}
	b.arrastou, b.movendo, b.emArea = false, nil, false
	b.Refresh()
}

func (b *board) excluir() {
	if b.sel != 0 || b.grupo != nil {
		b.s.Marcar()
	}
	for id := range b.grupo {
		b.s.Remover(id)
	}
	if b.sel != 0 || b.grupo != nil {
		b.s.Remover(b.sel)
		b.selecionar(0, false)
		b.Refresh()
	}
}

func (b *board) cor(id int) color.Color {
	if id == b.sel || b.grupo[id] || (b.ligarDe != nil && b.ligarDe.ID == id) {
		return caneta
	}
	return tinta
}

func (b *board) espessura(id int) float32 {
	if c := b.cor(id); c == caneta {
		return 2
	}
	return 1.25
}

// ---- rendering ----

// texto sets a label inside the figure (sans, like the Arial of a TCC figure).
func texto(s string, x, y float32, c color.Color, bold bool) *canvas.Text {
	return textoCom(s, x, y, tamFigura, fonteFig(bold), c)
}

// rotuloCard puts a cardinality next to point p, on the side the line leaves
// towards (dx, dy), on a scrap of paper so no line or rule strikes through it.
func rotuloCard(s string, p fyne.Position, dx, dy float32, c color.Color) []fyne.CanvasObject {
	return rotuloCardEm(s, p, dx, dy, 6, c)
}

// rotuloCardEm is rotuloCard with the label d pixels along the line from p.
func rotuloCardEm(s string, p fyne.Position, dx, dy, d float32, c color.Color) []fyne.CanvasObject {
	n := float32(math.Hypot(float64(dx), float64(dy)))
	if n == 0 {
		n = 1
	}
	ux, uy := dx/n, dy/n
	x, y := p.X+ux*d, p.Y+uy*d
	if math.Abs(float64(ux)) >= math.Abs(float64(uy)) {
		if ux < 0 {
			x -= medir(s, tamCard, fonteSans)
		}
		y -= tamCard + 7
	} else {
		x += 5
		if uy < 0 {
			y -= tamCard + 5
		}
	}
	t := textoCom(s, x, y, tamCard, fonteSans, c)
	bg := canvas.NewRectangle(papel)
	bg.Move(fyne.NewPos(x-2, y))
	bg.Resize(t.MinSize().AddWidthHeight(4, 0))
	return []fyne.CanvasObject{bg, t}
}

const largura, altura = 3000, 2000

func fundo() fyne.CanvasObject {
	r := canvas.NewRectangle(papel)
	r.Resize(fyne.NewSize(largura, altura))
	return r
}

func linha(a, c fyne.Position, cor color.Color) *canvas.Line {
	l := canvas.NewLine(cor)
	l.StrokeWidth = 1.25
	l.Position1, l.Position2 = a, c
	return l
}

// legenda writes the ABNT figure caption above (x, y) and returns its height.
func legenda(objs *[]fyne.CanvasObject, num int, titulo string, x, y float32) {
	rot := fmt.Sprintf("Figura %d – ", num)
	*objs = append(*objs,
		textoCom(rot, x, y, tamLegenda, fonteSerifBold, tinta),
		textoCom(titulo, x+medir(rot, tamLegenda, fonteSerifBold), y, tamLegenda, fonteSerif, tinta))
}

func fonteDaFigura(objs *[]fyne.CanvasObject, s string, x, y float32) {
	*objs = append(*objs, textoCom(s, x, y, tamFonte, fonteSerif, grafite))
}

func autor() string { return fmt.Sprintf("Fonte: elaborado pelo autor (%d).", time.Now().Year()) }

func (b *board) caixa() (x0, y0, x1, y1 float32) {
	x0, y0, x1, y1, _ = b.s.D.Caixa()
	return
}

func (b *board) desenhar() []fyne.CanvasObject {
	objs := b.desenharModelo()
	escalar(objs, b.zm.z)
	return objs
}

// desenharModelo draws the figure in diagram coordinates (zoom is applied after).
func (b *board) desenharModelo() []fyne.CanvasObject {
	objs := []fyne.CanvasObject{fundo()}

	if len(b.s.D.Formas) == 0 { // empty state: the caption waits for its figure and says how to start
		legenda(&objs, 1, "Modelo conceitual", 72, 64)
		for i, s := range []string{
			"A figura ainda está em branco.",
			"Escolha 2 ENTIDADE no sumário (ou tecle E) e clique aqui para criar a primeira.",
			"Depois, 5 LIGAR entre duas entidades cria o relacionamento entre elas.",
		} {
			objs = append(objs, textoCom(s, 72, 104+float32(i)*24, 15, fonteSerifItalic, grafite))
		}
		fonteDaFigura(&objs, autor(), 72, 196)
		return objs
	}

	x0, y0, _, y1 := b.caixa()
	legenda(&objs, 1, "Modelo conceitual", x0, max(12, y0-44))
	fonteDaFigura(&objs, autor(), x0, y1+30)

	var cards []fyne.CanvasObject
	for _, l := range b.s.D.Ligacoes {
		a, c := b.pontas(l)
		ln := linha(a, c, b.cor(l.ID))
		ln.StrokeWidth = b.espessura(l.ID)
		objs = append(objs, ln)
		if l.Card >= 0 {
			if b.s.D.Forma(l.A).Tipo != dom.Entidade { // card sits next to the entidade
				a, c = c, a
			}
			cards = append(cards, b.cardLivre(dom.Cards[l.Card], a, c, b.cor(l.ID))...)
		}
	}

	objs = append(objs, cards...) // under the shapes: a label never paints over a shape's text

	for _, f := range b.s.D.Formas {
		c, w := b.cor(f.ID), b.espessura(f.ID)
		switch f.Tipo {
		case dom.Entidade:
			r := canvas.NewRectangle(papel)
			r.StrokeColor, r.StrokeWidth = c, w
			r.Move(fyne.NewPos(f.X, f.Y))
			r.Resize(fyne.NewSize(f.W, f.H))
			objs = append(objs, r)
		case dom.Relacionamento:
			cx, cy := f.X+f.W/2, f.Y+f.H/2
			t, r, bt, l := fyne.NewPos(cx, f.Y), fyne.NewPos(f.X+f.W, cy), fyne.NewPos(cx, f.Y+f.H), fyne.NewPos(f.X, cy)
			for _, s := range [][2]fyne.Position{{t, r}, {r, bt}, {bt, l}, {l, t}} {
				ln := linha(s[0], s[1], c)
				ln.StrokeWidth = w
				objs = append(objs, ln)
			}
		case dom.Atributo:
			fill := color.Color(papel)
			if f.Identificador {
				fill = c
			}
			o := canvas.NewCircle(fill)
			o.StrokeColor, o.StrokeWidth = c, w
			o.Move(fyne.NewPos(f.X, f.Y+f.H/2-5))
			o.Resize(fyne.NewSize(10, 10))
			objs = append(objs, o, texto(f.Texto, f.X+16, f.Y+f.H/2-tamFigura/2-2, c, false))
			continue
		}
		objs = append(objs, texto(f.Texto, f.X+(f.W-textW(f.Texto))/2, f.Y+f.H/2-tamFigura/2-2, c, true))
	}

	if b.nota != "" && b.notaEm != nil { // the advisor circles the shape and writes in the margin
		f := b.notaEm
		r := canvas.NewRectangle(color.Transparent)
		pad := b.folgaNota(f)
		r.StrokeColor, r.StrokeWidth, r.CornerRadius = caneta, 1.5, (f.H+2*pad)/2 // a pen loop around it
		r.Move(fyne.NewPos(f.X-pad, f.Y-pad))
		r.Resize(fyne.NewSize(f.W+2*pad, f.H+2*pad))
		objs = append(objs, r, textoCom(b.nota, f.X+f.W+16, f.Y-8, 15, fonteSerifItalic, caneta))
	}
	if b.emArea {
		r := canvas.NewRectangle(marca)
		r.StrokeColor, r.StrokeWidth = caneta, 1
		r.Move(fyne.NewPos(min(b.areaIni.X, b.areaFim.X), min(b.areaIni.Y, b.areaFim.Y)))
		r.Resize(fyne.NewSize(abs(b.areaFim.X-b.areaIni.X), abs(b.areaFim.Y-b.areaIni.Y)))
		objs = append(objs, r)
	}
	return objs
}

func abs(v float32) float32 { return max(v, -v) }

// sincronizar follows a change made outside the pointer (MCP, assistant,
// undo): drops references to shapes that no longer exist, redraws, refreshes the card.
func (b *board) sincronizar() {
	if b.ligarDe != nil && b.s.D.Forma(b.ligarDe.ID) != b.ligarDe {
		b.ligarDe = nil
	}
	if b.notaEm != nil && b.s.D.Forma(b.notaEm.ID) != b.notaEm {
		b.nota, b.notaEm = "", nil
	}
	b.grupo = nil
	if b.s.D.Forma(b.sel) == nil && !b.temLigacao(b.sel) {
		b.sel = 0
	}
	b.Refresh()
	if b.onSel != nil {
		b.onSel(false)
	}
}

func (b *board) desfazer() {
	if b.s.Desfazer() {
		b.sincronizar()
	}
}

func (b *board) refazer() {
	if b.s.Refazer() {
		b.sincronizar()
	}
}

func (b *board) temLigacao(id int) bool {
	for _, l := range b.s.D.Ligacoes {
		if l.ID == id {
			return true
		}
	}
	return false
}

// cardLivre places a cardinality next to the entity end a of a link, sliding it
// along the line until its box covers no shape or label.
func (b *board) cardLivre(s string, a, c fyne.Position, cor color.Color) []fyne.CanvasObject {
	var objs []fyne.CanvasObject
	for d := float32(6); d <= 78; d += 8 {
		objs = rotuloCardEm(s, a, c.X-a.X, c.Y-a.Y, d, cor)
		r := objs[0] // the paper backing is the label's box
		if !b.cobre(r.Position(), r.Size()) {
			break
		}
	}
	return objs
}

func (b *board) cobre(p fyne.Position, s fyne.Size) bool {
	for _, f := range b.s.D.Formas {
		if p.X < f.X+f.W && f.X < p.X+s.Width && p.Y < f.Y+f.H && f.Y < p.Y+s.Height {
			return true
		}
	}
	return false
}

// folgaNota: the pen loop hugs the shape tightly when a neighbour sits within its usual margin.
func (b *board) folgaNota(f *dom.Forma) float32 {
	const larga = 7
	for _, o := range b.s.D.Formas {
		if o != f && o.X < f.X+f.W+larga && f.X-larga < o.X+o.W && o.Y < f.Y+f.H+larga && f.Y-larga < o.Y+o.H {
			return 2
		}
	}
	return larga
}

type renderer struct {
	b      *board
	objs   []fyne.CanvasObject
	minAnt fyne.Size
}

// folga is the room past the figure's far corner, so there is always paper to drag onto.
const folga = 400

func (b *board) CreateRenderer() fyne.WidgetRenderer {
	return &renderer{b: b, objs: b.desenhar()}
}

func (r *renderer) Layout(fyne.Size) {}
func (r *renderer) MinSize() fyne.Size {
	if len(r.b.s.D.Formas) == 0 {
		return fyne.Size{}
	}
	_, _, x1, y1 := r.b.caixa()
	return fyne.NewSize((x1+folga)*r.b.zm.z, (y1+folga)*r.b.zm.z)
}
func (r *renderer) Objects() []fyne.CanvasObject { return r.objs }
func (r *renderer) Destroy()                     {}

// ponytail: rebuilds every canvas object on each refresh; fine for classroom-size diagrams, diff objects if it lags
func (r *renderer) Refresh() {
	r.objs = r.b.desenhar()
	canvas.Refresh(r.b)
	if ms := r.MinSize(); ms != r.minAnt && r.b.scroll != nil { // the figure grew or shrank: resize the scroll area
		r.minAnt = ms
		r.b.scroll.Refresh()
	}
}

// tiposSQL are the common column types offered for an attribute; any other can be typed.
var tiposSQL = []string{
	"INT", "BIGINT", "SMALLINT", "DECIMAL(10,2)", "FLOAT",
	"CHAR(1)", "VARCHAR(80)", "TEXT",
	"DATE", "TIME", "TIMESTAMP", "BOOLEAN",
}

// propriedades fills the catalog card with the selected item's fields.
func (b *board) propriedades(novo bool) fyne.CanvasObject {
	if e := b.erro; e != "" {
		b.erro = ""
		return ficha("Ficha – Atenção", notaCaneta(e))
	}
	if len(b.grupo) > 0 {
		return ficha(fmt.Sprintf("Ficha – Seleção de %d elementos", len(b.grupo)),
			nota("Arraste qualquer um deles\npara mover o conjunto.\nDel exclui todos."))
	}
	if f := b.s.D.Forma(b.sel); f != nil {
		nome := widget.NewEntry()
		nome.SetText(f.Texto)
		marcou := false // typing a name is one undo step, not one per letter
		nome.OnChanged = func(s string) {
			if !marcou {
				b.s.Marcar()
				marcou = true
			}
			b.s.Renomear(f, s)
			b.Refresh()
		}
		itens := []fyne.CanvasObject{campo("Nome", nome)}
		if f.Tipo == dom.Atributo {
			tipo := widget.NewSelectEntry(tiposSQL)
			tipo.SetText(f.TipoAtributo)
			tipo.SetPlaceHolder("ex.: INT")
			marcouTipo := false
			tipo.OnChanged = func(s string) {
				if !marcouTipo {
					b.s.Marcar()
					marcouTipo = true
				}
				f.TipoAtributo = s
			}
			itens = append(itens, campo("Tipo", tipo))
			id := widget.NewCheck("Identificador (chave)", func(v bool) {
				if v == f.Identificador {
					return
				}
				b.s.Marcar()
				f.Identificador = v
				b.Refresh()
			})
			id.SetChecked(f.Identificador)
			itens = append(itens, id)
		}
		if novo { // type the name right after creating the item
			b.win.Canvas().Focus(nome)
			nome.TypedShortcut(&fyne.ShortcutSelectAll{})
		}
		return ficha(fmt.Sprintf("Ficha – %s nº %d", f.Tipo, f.ID), itens...)
	}
	for _, l := range b.s.D.Ligacoes {
		if l.ID == b.sel {
			if l.Card < 0 {
				return ficha("Ficha – Ligação de atributo", nota("Sem cardinalidade: liga\no atributo ao seu dono."))
			}
			card := widget.NewSelect(dom.Cards, nil)
			card.SetSelectedIndex(l.Card)
			card.OnChanged = func(string) {
				if card.SelectedIndex() == l.Card {
					return
				}
				b.s.Marcar()
				l.Card = card.SelectedIndex()
				b.Refresh()
			}
			return ficha("Ficha – Ligação", campo("Card.", card))
		}
	}
	return ficha("Ficha do elemento", nota("Nada selecionado. Clique\nnuma forma da Figura 1\npara editá-la aqui."))
}

// abrirArquivo loads a brModelo XML into the board, or a logical one into its own window.
func abrirArquivo(w fyne.Window, b *board) {
	dl := dialog.NewFileOpen(func(r fyne.URIReadCloser, err error) {
		if err != nil || r == nil {
			return
		}
		defer r.Close()
		v, err := xmlbrm.Abrir(r)
		if err != nil {
			b.avisar(erroAbrir(r.URI().Name(), err))
			return
		}
		switch v := v.(type) {
		case *dom.Diagrama:
			b.s.Marcar() // opening a file can be undone too
			b.s.D = v
			b.selecionar(0, true)
			b.Refresh()
		case *dom.Logico:
			janelaLogico(v, "Fonte: "+r.URI().Name()+".", nil)
		}
	}, w)
	dl.SetFilter(storage.NewExtensionFileFilter([]string{".xml"}))
	dl.Show()
}

// Montar builds the main window around session s: the SUMÁRIO sheet on the
// left, Figura 1 on the right. statusMCP is shown at the foot of the SUMÁRIO.
// It also plugs the window into the session: real font metrics, the UI thread
// for outside changes, and redraws after them.
//
// cofre keeps the ORIENTADOR's API keys.
func Montar(w fyne.Window, s *aplicacao.Sessao, statusMCP string, cofre *segredos.Cofre) *board {
	fyne.CurrentApp().Settings().SetTheme(temaABNT{})
	b := newBoard(w, s)
	s.Medir = func(t string, negrito bool) float32 { return medir(t, tamFigura, fonteFig(negrito)) }
	s.Executar = fyne.DoAndWait
	s.AoGerarLogico = func(l *dom.Logico, erros []string) {
		janelaLogico(l, "Fonte: convertido da Figura 1 pelas regras do brModelo.", erros)
	}
	s.AoMudar = b.sincronizar

	ferramentas := []struct{ modo, rotulo, tecla string }{
		{modoSel, "SELECIONAR", "V"},
		{string(dom.Entidade), "ENTIDADE", "E"},
		{string(dom.Relacionamento), "RELACIONAMENTO", "R"},
		{string(dom.Atributo), "ATRIBUTO", "A"},
		{modoLigar, "LIGAR", "L"},
	}
	var itens []*itemSumario
	usar := func(modo string) {
		b.modo, b.ligarDe = modo, nil
		b.anotar("", nil, false)
		for i, f := range ferramentas {
			itens[i].setAtivo(f.modo == modo)
		}
		b.Refresh()
	}
	teclas := map[fyne.KeyName]string{}
	for i, f := range ferramentas {
		itens = append(itens, novoItem(strconv.Itoa(i+1), f.rotulo, f.tecla, func() { usar(f.modo) }))
		teclas[fyne.KeyName(f.tecla)] = f.modo
	}
	itens[0].setAtivo(true)

	gerar := func() { s.GerarLogico() }
	edicao := []fyne.CanvasObject{
		novoItem("6.1", "Desfazer", "Ctrl+Z", b.desfazer),
		novoItem("6.2", "Refazer", "Ctrl+Y", b.refazer),
		novoItem("6.3", "Excluir seleção", "Del", b.excluir),
	}
	comandos := []fyne.CanvasObject{
		novoItem("7.1", "Abrir", "Ctrl+O", func() { abrirArquivo(w, b) }),
		novoItem("7.2", "Salvar", "Ctrl+S", func() {
			salvarXML(w, "conceitual.xml", func(w io.Writer) error { return xmlbrm.SalvarConceitual(w, b.s.D) }, b.avisar)
		}),
		novoItem("7.3", "Gerar modelo lógico", "Ctrl+G", gerar),
	}
	for k, f := range map[fyne.KeyName]func(){
		fyne.KeyO: func() { abrirArquivo(w, b) },
		fyne.KeyS: func() {
			salvarXML(w, "conceitual.xml", func(w io.Writer) error { return xmlbrm.SalvarConceitual(w, b.s.D) }, b.avisar)
		},
		fyne.KeyG: gerar,
		fyne.KeyZ: b.desfazer,
		fyne.KeyY: b.refazer,
	} {
		w.Canvas().AddShortcut(&desktop.CustomShortcut{KeyName: k, Modifier: fyne.KeyModifierShortcutDefault}, func(fyne.Shortcut) { f() })
	}
	w.Canvas().AddShortcut(&desktop.CustomShortcut{KeyName: fyne.KeyZ, Modifier: fyne.KeyModifierShortcutDefault | fyne.KeyModifierShift},
		func(fyne.Shortcut) { b.refazer() })
	w.Canvas().SetOnTypedKey(func(k *fyne.KeyEvent) {
		switch {
		case k.Name == fyne.KeyDelete || k.Name == fyne.KeyBackspace:
			b.excluir()
		case k.Name == fyne.KeyEscape:
			b.ligarDe = nil
			b.anotar("", nil, false)
			b.selecionar(0, false)
			b.Refresh()
		case teclas[k.Name] != "":
			usar(teclas[k.Name])
		}
	})

	fichaBox := container.NewStack(b.propriedades(false))
	var rolagem *container.Scroll // the sheet's layout must rerun when the card changes height
	b.onSel = func(novo bool) {
		fichaBox.Objects = []fyne.CanvasObject{b.propriedades(novo)}
		rolagem.Refresh()
	}

	largura := canvas.NewRectangle(color.Transparent)
	largura.SetMinSize(fyne.NewSize(300, 0))
	sumario := container.NewVBox(capa("Modelagem conceitual e lógica", statusMCP), titulo("Sumário"))
	for _, i := range itens {
		sumario.Add(i)
	}
	sumario.Add(novoItem("6", "Edição", "", nil))
	for _, c := range edicao {
		sumario.Add(c)
	}
	sumario.Add(novoItem("7", "Arquivo", "", nil))
	for _, c := range comandos {
		sumario.Add(c)
	}
	comandoOrientador := novoItem("8", "Orientador", "Ctrl+K", func() {}) // shows/hides the chat sheet
	sumario.Add(comandoOrientador)

	// the card stays at the foot of the sheet; when both don't fit, the sheet scrolls instead of overlapping
	rolagem = container.NewVScroll(container.NewBorder(sumario, fichaBox, nil, nil))
	lado := folha(container.NewStack(largura, rolagem), 22)

	b.scroll = container.NewScroll(b)
	var controleZoom fyne.CanvasObject
	b.zm, controleZoom = novoZoom(func() { b.Refresh(); b.scroll.Refresh() })
	b.zm.atalhos(w)
	figura := folha(comControle(b.scroll, controleZoom), 0)

	ajudante, errAjudante := assistente.Novo(s)
	var folhaOrientador fyne.CanvasObject
	b.orientador, folhaOrientador = novoOrientador(b, ajudante, cofre, errAjudante)
	direita := container.New(layout.NewCustomPaddedLayout(0, 0, 18, 0), folhaOrientador)
	alternar := func() {
		if direita.Visible() {
			direita.Hide()
		} else {
			direita.Show()
		}
		w.Content().Refresh()
	}
	comandoOrientador.tocar = alternar
	w.Canvas().AddShortcut(&desktop.CustomShortcut{KeyName: fyne.KeyK, Modifier: fyne.KeyModifierShortcutDefault}, func(fyne.Shortcut) { alternar() })
	gutter := layout.NewCustomPaddedLayout(18, 22, 18, 22)
	w.SetContent(container.New(gutter,
		container.NewBorder(nil, nil, container.New(layout.NewCustomPaddedLayout(0, 0, 0, 18), lado), direita, figura)))
	return b
}
