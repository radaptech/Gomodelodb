package gui

import (
	"fmt"
	"image/color"
	"io"
	"math"
	"slices"
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

	dom "github.com/brmodelo/gobrmodelo/internal/dominio"
	"github.com/brmodelo/gobrmodelo/internal/infra/sqlgen"
	"github.com/brmodelo/gobrmodelo/internal/infra/xmlbrm"
)

// larguraFigura is the figure sheet's width in a 1200px window:
// 1200 − 18 − 22 (desk) − 344 (sumário sheet) − 18 (gutter) − 14 (scroll bar).
const larguraFigura = 784

// Tables are set like ABNT/IBGE tables: name above, a heavy rule on top, a
// light rule under the column heads, a heavy rule at the bottom, no sides.
const (
	tabTitulo = 20 // table name line
	tabCab    = 22 // column-head row
	tabLinha  = 22 // field row
	tabPad    = 10 // horizontal cell padding
	tabCols   = 3  // Campo | Tipo | Chave
)

// quadroLogico shows a logical diagram; tables can be dragged around.
type quadroLogico struct {
	widget.BaseWidget
	l        *dom.Logico
	fonte    string   // the figure's "Fonte:" line
	notas    []string // converter warnings, written in red under the figure
	arrastar *dom.Tabela
	arrastou bool
	scroll   *container.Scroll
	zm       *zoom
	// perguntarSQL opens the migration question on the card
	perguntarSQL func()
}

func (q *quadroLogico) tabelaEm(p fyne.Position) *dom.Tabela {
	for i := len(q.l.Tabelas) - 1; i >= 0; i-- {
		t := q.l.Tabelas[i]
		if p.X >= t.X && p.X <= t.X+t.W && p.Y >= t.Y && p.Y <= t.Y+t.H {
			return t
		}
	}
	return nil
}

func (q *quadroLogico) Dragged(ev *fyne.DragEvent) {
	pos, delta := q.zm.modelo(ev.Position), q.zm.modelo(fyne.NewPos(ev.Dragged.DX, ev.Dragged.DY))
	if !q.arrastou {
		q.arrastou = true
		q.arrastar = q.tabelaEm(pos.Subtract(delta))
	}
	if q.arrastar != nil {
		q.arrastar.X += delta.X
		q.arrastar.Y += delta.Y
		q.Refresh()
	}
}

func (q *quadroLogico) DragEnd() {
	q.arrastou, q.arrastar = false, nil
	q.scroll.Refresh() // the figure may have grown
}

func chave(c *dom.Campo) string {
	switch {
	case c.Key && c.Fkey:
		return "PK, FK"
	case c.Key:
		return "PK"
	case c.Fkey:
		return "FK"
	}
	return ""
}

// colunas returns the three column widths of t.
func colunas(t *dom.Tabela) [tabCols]float32 {
	w := [tabCols]float32{
		medir("Campo", tamCard, fonteSansBold),
		medir("Tipo", tamCard, fonteSansBold),
		medir("Chave", tamCard, fonteSansBold),
	}
	for _, c := range t.Campos {
		w[0] = max(w[0], medir(c.Nome, tamFigura, fonteSans))
		w[1] = max(w[1], medir(c.Tipo, tamFigura, fonteSans))
		w[2] = max(w[2], medir(chave(c), tamFigura, fonteSansBold))
	}
	for i := range w {
		w[i] += 2 * tabPad
	}
	return w
}

// autosize fits every table to its text, like Tabela.Autosize.
func autosize(l *dom.Logico) {
	for _, t := range l.Tabelas {
		c := colunas(t)
		t.W = max(c[0]+c[1]+c[2], medir(t.Nome, tamFigura+1, fonteSansBold)+8)
		t.H = tabTitulo + tabCab + tabLinha*float32(max(len(t.Campos), 1)) + 4
	}
}

// Margins of the figure sheet: the caption sits on the top margin.
const (
	margemEsq = 60
	margemTop = 112 // caption at 64 plus room for cardinalities above the first row
)

func caixaTabelas(l *dom.Logico) (x0, y0, x1, y1 float32) {
	x0, y0 = l.Tabelas[0].X, l.Tabelas[0].Y
	x1, y1 = x0, y0
	for _, t := range l.Tabelas {
		x0, y0 = min(x0, t.X), min(y0, t.Y)
		x1, y1 = max(x1, t.X+t.W), max(y1, t.Y+t.H)
	}
	return
}

// centralizar spaces the tables out (Logico.Organizar, the Java organizer),
// pins the figure to the top margin, centers it across a sheet w wide, and
// wraps tables that would cross the right margin onto a new row below.
func centralizar(l *dom.Logico, w float32) {
	if len(l.Tabelas) == 0 {
		return
	}
	autosize(l)
	l.Organizar()
	x0, y0, x1, _ := caixaTabelas(l)
	dx := max(margemEsq, (w-(x1-x0))/2) - x0
	for _, t := range l.Tabelas {
		t.X, t.Y = t.X+dx, t.Y+margemTop-y0
	}
	// ponytail: one wrap pass into a single extra row; a real layout engine if diagrams outgrow it
	var fora []*dom.Tabela
	for _, t := range l.Tabelas {
		if t.X+t.W > w-margemEsq {
			fora = append(fora, t)
		}
	}
	if len(fora) == 0 {
		return
	}
	_, _, _, y1 := caixaTabelas(l)
	x := float32(margemEsq)
	for _, t := range fora {
		t.X, t.Y = x, y1+120
		x += t.W + 160
	}
}

func regua(x0, x1, y, peso float32) *canvas.Line {
	l := canvas.NewLine(tinta)
	l.StrokeWidth = peso
	l.Position1, l.Position2 = fyne.NewPos(x0, y), fyne.NewPos(x1, y)
	return l
}

func desenharTabela(t *dom.Tabela) []fyne.CanvasObject {
	fundo := canvas.NewRectangle(papel) // hides the links passing behind
	fundo.Move(fyne.NewPos(t.X, t.Y))
	fundo.Resize(fyne.NewSize(t.W, t.H))
	objs := []fyne.CanvasObject{fundo, textoCom(t.Nome, t.X, t.Y, tamFigura+1, fonteSansBold, tinta)}

	c := colunas(t)
	xs := [tabCols]float32{t.X, t.X + c[0], t.X + c[0] + c[1]}
	y := t.Y + tabTitulo
	objs = append(objs, regua(t.X, t.X+t.W, y, 1.75))
	for i, h := range []string{"Campo", "Tipo", "Chave"} {
		objs = append(objs, textoCom(h, xs[i]+tabPad, y+4, tamCard, fonteSansBold, grafite))
	}
	y += tabCab
	objs = append(objs, regua(t.X, t.X+t.W, y, 0.75))
	if len(t.Campos) == 0 {
		objs = append(objs, textoCom("sem campos", xs[0]+tabPad, y+3, tamFigura, fonteSerifItalic, grafite))
		y += tabLinha
	}
	for _, cp := range t.Campos {
		cor := color.Color(tinta)
		objs = append(objs,
			textoCom(cp.Nome, xs[0]+tabPad, y+3, tamFigura, fonteSans, cor),
			textoCom(cp.Tipo, xs[1]+tabPad, y+3, tamFigura, fonteSans, grafite))
		if k := chave(cp); k != "" {
			if cp.Fkey && !cp.Key {
				cor = caneta // foreign keys are what the conversion added: the advisor marks them
			}
			objs = append(objs, textoCom(k, xs[2]+tabPad, y+3, tamFigura, fonteSansBold, cor))
		}
		y += tabLinha
	}
	return append(objs, regua(t.X, t.X+t.W, y+2, 1.75))
}

func (q *quadroLogico) desenhar() []fyne.CanvasObject {
	objs := q.desenharModelo()
	escalar(objs, q.zm.z)
	return objs
}

// desenharModelo draws Figura 2 in diagram coordinates (zoom is applied after).
func (q *quadroLogico) desenharModelo() []fyne.CanvasObject {
	objs := []fyne.CanvasObject{fundo()}
	autosize(q.l)
	if len(q.l.Tabelas) == 0 {
		legenda(&objs, 2, "Modelo lógico", 72, 64)
		objs = append(objs, textoCom("Nenhuma tabela: a Figura 1 não tem entidades para converter.", 72, 104, 15, fonteSerifItalic, grafite))
		fonteDaFigura(&objs, q.fonte, 72, 140)
		return objs
	}

	x0, y0, y1 := q.l.Tabelas[0].X, q.l.Tabelas[0].Y, float32(0)
	for _, t := range q.l.Tabelas {
		x0, y0, y1 = min(x0, t.X), min(y0, t.Y), max(y1, t.Y+t.H)
	}
	legenda(&objs, 2, "Modelo lógico", x0, max(12, y0-48))

	var cards []fyne.CanvasObject
	for _, ln := range q.l.Linhas {
		ca := fyne.NewPos(ln.A.X+ln.A.W/2, ln.A.Y+ln.A.H/2)
		cb := fyne.NewPos(ln.B.X+ln.B.W/2, ln.B.Y+ln.B.H/2)
		pa := borda(ca, cb, ln.A.W/2+6, ln.A.H/2+6, false)
		pb := borda(cb, ca, ln.B.W/2+6, ln.B.H/2+6, false)
		objs = append(objs, linha(pa, pb, tinta))
		// labels step out past the table box so its rules never cross them
		fora := func(p, para fyne.Position) fyne.Position {
			dx, dy := para.X-p.X, para.Y-p.Y
			n := float32(math.Hypot(float64(dx), float64(dy)))
			if n == 0 {
				return p
			}
			return p.Add(fyne.NewPos(dx/n*(tamCard+4), dy/n*(tamCard+4)))
		}
		cards = append(cards, rotuloCard(dom.Cards[ln.CardA], fora(pa, pb), pb.X-pa.X, pb.Y-pa.Y, tinta)...)
		cards = append(cards, rotuloCard(dom.Cards[ln.CardB], fora(pb, pa), pa.X-pb.X, pa.Y-pb.Y, tinta)...)
	}
	for _, t := range q.l.Tabelas {
		objs = append(objs, desenharTabela(t)...)
	}
	objs = append(objs, cards...)

	fonteDaFigura(&objs, q.fonte, x0, y1+30)
	for i, n := range q.notas {
		objs = append(objs, textoCom("Nota: "+n, x0, y1+56+float32(i)*22, 14, fonteSerifItalic, caneta))
	}
	return objs
}

type renderLogico struct {
	q    *quadroLogico
	objs []fyne.CanvasObject
}

func (q *quadroLogico) CreateRenderer() fyne.WidgetRenderer {
	return &renderLogico{q: q, objs: q.desenhar()}
}

func (r *renderLogico) Layout(fyne.Size) {}
func (r *renderLogico) MinSize() fyne.Size {
	if len(r.q.l.Tabelas) == 0 {
		return fyne.Size{}
	}
	_, _, x1, y1 := caixaTabelas(r.q.l)
	return fyne.NewSize((x1+folga)*r.q.zm.z, (y1+folga)*r.q.zm.z)
}
func (r *renderLogico) Objects() []fyne.CanvasObject { return r.objs }
func (r *renderLogico) Destroy()                     {}
func (r *renderLogico) Refresh() {
	r.objs = r.q.desenhar()
	canvas.Refresh(r.q)
}

// salvarXML asks where to save; a failure goes to falhou (the red note on the card).
func salvarXML(w fyne.Window, nome string, save func(io.Writer) error, falhou func(error)) {
	salvarArquivo(w, nome, ".xml", save, falhou, nil)
}

// salvarArquivo asks where to save a file of extension ext; salvou (optional)
// gets the chosen file name on success.
func salvarArquivo(w fyne.Window, nome, ext string, save func(io.Writer) error, falhou func(error), salvou func(string)) {
	dl := dialog.NewFileSave(func(wr fyne.URIWriteCloser, err error) {
		if err != nil || wr == nil {
			return
		}
		defer wr.Close()
		if err := save(wr); err != nil {
			falhou(fmt.Errorf("Não foi possível salvar %s. Verifique se a pasta permite gravar e tente de novo.", wr.URI().Name()))
			return
		}
		if salvou != nil {
			salvou(wr.URI().Name())
		}
	}, w)
	dl.SetFilter(storage.NewExtensionFileFilter([]string{ext}))
	dl.SetFileName(nome)
	dl.Show()
}

// montarLogico builds the logical window's content: a short SUMÁRIO with the
// model summary on its card, and Figura 2.
func montarLogico(w fyne.Window, l *dom.Logico, fonte string, notas []string) *quadroLogico {
	q := &quadroLogico{l: l, fonte: fonte, notas: notas, zm: &zoom{z: 1}}
	q.ExtendBaseWidget(q)
	fichaBox := container.NewStack()
	var rolagem *container.Scroll
	mostrar := func(o fyne.CanvasObject) {
		fichaBox.Objects = []fyne.CanvasObject{o}
		rolagem.Refresh()
	}
	falhou := func(err error) { mostrar(ficha("Ficha – Atenção", notaCaneta(err.Error()))) }
	salvar := func() {
		salvarXML(w, "logico.xml", func(wr io.Writer) error { return xmlbrm.SalvarLogico(wr, l) }, falhou)
	}
	var resumo fyne.CanvasObject

	// perguntarSQL asks for the target database on the card, then saves the migration.
	q.perguntarSQL = func() {
		escolha := widget.NewRadioGroup(sqlgen.Dialetos, nil)
		gerar := widget.NewButton("Gerar e salvar .sql", nil)
		gerar.Disable() // nothing is generated until a database is chosen
		escolha.OnChanged = func(s string) {
			if s != "" {
				gerar.Enable()
			}
		}
		gerar.OnTapped = func() {
			d := sqlgen.Dialeto(slices.Index(sqlgen.Dialetos, escolha.Selected))
			agora := time.Now()
			salvarArquivo(w, sqlgen.NomeMigration(agora), ".sql", func(wr io.Writer) error {
				_, err := io.WriteString(wr, sqlgen.Gerar(l, d, agora))
				return err
			}, falhou, func(nome string) {
				mostrar(ficha("Ficha – Migration salva", nota(strings.Join(quebrar(
					fmt.Sprintf("%s gravado para %s: %d tabelas com chaves primárias e estrangeiras.", nome, d, len(l.Tabelas)), 30), "\n"))))
			})
		}
		mostrar(ficha("Ficha – Migration SQL",
			nota("Para qual banco de dados?"),
			escolha,
			container.NewHBox(gerar, widget.NewButton("Voltar", func() { mostrar(resumo) }))))
	}
	organizar := func() {
		autosize(l)
		l.Organizar()
		q.Refresh()
	}
	for k, f := range map[fyne.KeyName]func(){fyne.KeyS: salvar, fyne.KeyR: organizar, fyne.KeyM: q.perguntarSQL} {
		w.Canvas().AddShortcut(&desktop.CustomShortcut{KeyName: k, Modifier: fyne.KeyModifierShortcutDefault}, func(fyne.Shortcut) { f() })
	}

	fks := 0
	for _, t := range l.Tabelas {
		fks += len(t.FKs)
	}
	fk := "Nenhuma chave estrangeira."
	switch {
	case fks == 1:
		fk = "1 chave estrangeira, em vermelho."
	case fks > 1:
		fk = fmt.Sprintf("%d chaves estrangeiras, em vermelho.", fks)
	}
	resumo = ficha(fmt.Sprintf("Ficha – %d tabelas · %d ligações", len(l.Tabelas), len(l.Linhas)),
		nota(fk+"\nArraste uma tabela para\nreposicioná-la."))
	fichaBox.Add(resumo)

	largura := canvas.NewRectangle(color.Transparent)
	largura.SetMinSize(fyne.NewSize(300, 0))
	sumario := container.NewVBox(
		capa("Modelo lógico"),
		titulo("Sumário"),
		novoItem("1", "Salvar", "Ctrl+S", salvar),
		novoItem("2", "Organizar tabelas", "Ctrl+R", organizar),
		novoItem("3", "Gerar SQL", "Ctrl+M", q.perguntarSQL),
	)
	rolagem = container.NewVScroll(container.NewBorder(sumario, fichaBox, nil, nil))
	lado := folha(container.NewStack(largura, rolagem), 22)
	q.scroll = container.NewScroll(q)
	var controleZoom fyne.CanvasObject
	q.zm, controleZoom = novoZoom(func() { q.Refresh(); q.scroll.Refresh() })
	q.zm.atalhos(w)
	w.SetContent(container.New(layout.NewCustomPaddedLayout(18, 22, 18, 22),
		container.NewBorder(nil, nil, container.New(layout.NewCustomPaddedLayout(0, 0, 0, 18), lado), nil,
			folha(comControle(q.scroll, controleZoom), 0))))
	return q
}

func janelaLogico(l *dom.Logico, fonte string, notas []string) fyne.Window {
	w := fyne.CurrentApp().NewWindow("brModelo — Modelo lógico")
	centralizar(l, larguraFigura) // ponytail: initial figure-sheet width, not tracked on resize
	montarLogico(w, l, fonte, notas)
	w.Resize(fyne.NewSize(1200, 760))
	w.Show()
	return w
}
