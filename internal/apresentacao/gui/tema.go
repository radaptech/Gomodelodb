package gui

// Visual world: an ABNT monograph. White A4 sheets on a grey desk, Times-metric
// serif for the document, Arial-metric sans inside figures, black ink, and the
// advisor's red pen for selection, notes and errors. Light only: it is paper.

import (
	_ "embed"
	"image/color"
	"math"
	"strconv"
	"strings"
	"time"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/canvas"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/driver/desktop"
	"fyne.io/fyne/v2/layout"
	"fyne.io/fyne/v2/theme"
	"fyne.io/fyne/v2/widget"
)

var (
	//go:embed fonts/LiberationSerif-Regular.ttf
	serifRegular []byte
	//go:embed fonts/LiberationSerif-Bold.ttf
	serifBold []byte
	//go:embed fonts/LiberationSerif-Italic.ttf
	serifItalic []byte
	//go:embed fonts/LiberationSans-Regular.ttf
	sansRegular []byte
	//go:embed fonts/LiberationSans-Bold.ttf
	sansBold []byte

	fonteSerif       = fyne.NewStaticResource("LiberationSerif-Regular.ttf", serifRegular)
	fonteSerifBold   = fyne.NewStaticResource("LiberationSerif-Bold.ttf", serifBold)
	fonteSerifItalic = fyne.NewStaticResource("LiberationSerif-Italic.ttf", serifItalic)
	fonteSans        = fyne.NewStaticResource("LiberationSans-Regular.ttf", sansRegular)
	fonteSansBold    = fyne.NewStaticResource("LiberationSans-Bold.ttf", sansBold)
)

var (
	papel   = color.NRGBA{0xff, 0xff, 0xff, 0xff} // the sheet
	mesa    = color.NRGBA{0xd4, 0xd6, 0xd9, 0xff} // desk the sheets lie on
	tinta   = color.NRGBA{0x17, 0x17, 0x17, 0xff} // ink
	grafite = color.NRGBA{0x55, 0x55, 0x55, 0xff} // secondary text, 7.4:1 on paper
	filete  = color.NRGBA{0x17, 0x17, 0x17, 0x33} // hairline rules
	caneta  = color.NRGBA{0xb3, 0x14, 0x1d, 0xff} // advisor's red pen, 6.6:1 on paper
	marca   = color.NRGBA{0xb3, 0x14, 0x1d, 0x14} // red-pen wash for hover/selection areas
)

type temaABNT struct{}

func (temaABNT) Color(n fyne.ThemeColorName, _ fyne.ThemeVariant) color.Color {
	switch n {
	case theme.ColorNameBackground:
		return mesa
	case theme.ColorNameForeground, theme.ColorNameForegroundOnPrimary:
		return tinta
	case theme.ColorNamePrimary, theme.ColorNameFocus, theme.ColorNameHyperlink, theme.ColorNameError:
		return caneta
	case theme.ColorNameSelection:
		return color.NRGBA{0xb3, 0x14, 0x1d, 0x38}
	case theme.ColorNameInputBackground, theme.ColorNameMenuBackground, theme.ColorNameOverlayBackground, theme.ColorNameHeaderBackground:
		return papel
	case theme.ColorNameInputBorder:
		return color.NRGBA{0x17, 0x17, 0x17, 0x8c}
	case theme.ColorNameButton:
		return color.NRGBA{0xee, 0xef, 0xf0, 0xff}
	case theme.ColorNameHover:
		return marca
	case theme.ColorNamePressed:
		return color.NRGBA{0xb3, 0x14, 0x1d, 0x26}
	case theme.ColorNamePlaceHolder, theme.ColorNameDisabled:
		return grafite
	case theme.ColorNameSeparator:
		return filete
	case theme.ColorNameScrollBar:
		return color.NRGBA{0x17, 0x17, 0x17, 0x26} // a hairline, not a stripe on the page
	case theme.ColorNameScrollBarBackground:
		return color.NRGBA{0, 0, 0, 0}
	case theme.ColorNameShadow:
		return color.NRGBA{0, 0, 0, 0x2e}
	}
	return theme.DefaultTheme().Color(n, theme.VariantLight)
}

func (temaABNT) Font(s fyne.TextStyle) fyne.Resource {
	switch {
	case s.Monospace:
		return theme.DefaultTheme().Font(s)
	case s.Bold:
		return fonteSerifBold
	case s.Italic:
		return fonteSerifItalic
	}
	return fonteSerif
}

func (temaABNT) Icon(n fyne.ThemeIconName) fyne.Resource { return theme.DefaultTheme().Icon(n) }

func (temaABNT) Size(n fyne.ThemeSizeName) float32 {
	switch n {
	case theme.SizeNameText:
		return 15
	case theme.SizeNameCaptionText:
		return 12
	case theme.SizeNameHeadingText:
		return 20
	case theme.SizeNameInputRadius, theme.SizeNameSelectionRadius, theme.SizeNameScrollBarRadius:
		return 0 // paper has square corners
	case theme.SizeNameInputBorder:
		return 1
	case theme.SizeNameScrollBar:
		return 3
	case theme.SizeNameScrollBarSmall:
		return 2
	}
	return theme.DefaultTheme().Size(n)
}

// ---- text helpers ----

// Sizes on the sheet (points, as the monograph would set them).
const (
	tamFigura  = 12 // labels inside figures
	tamCard    = 11 // cardinalities
	tamLegenda = 13 // "Figura 1 – ..."
	tamFonte   = 11 // "Fonte: ..."
)

func fonteFig(bold bool) fyne.Resource {
	if bold {
		return fonteSansBold
	}
	return fonteSans
}

func medir(s string, tam float32, fonte fyne.Resource) float32 {
	sz, _ := fyne.CurrentApp().Driver().RenderedTextSize(s, tam, fyne.TextStyle{}, fonte)
	return sz.Width
}

// textW measures a figure label (sans bold, like an entity name).
func textW(s string) float32 { return medir(s, tamFigura, fonteSansBold) }

func textoCom(s string, x, y, tam float32, fonte fyne.Resource, c color.Color) *canvas.Text {
	t := canvas.NewText(s, c)
	t.TextSize, t.FontSource = tam, fonte
	t.Move(fyne.NewPos(x, y))
	return t
}

// ---- the sheet ----

// folha lays content on a white sheet with a soft drop shadow over the desk.
func folha(content fyne.CanvasObject, margem float32) fyne.CanvasObject {
	sombras := []*canvas.Rectangle{
		canvas.NewRectangle(color.NRGBA{0, 0, 0, 0x0c}),
		canvas.NewRectangle(color.NRGBA{0, 0, 0, 0x10}),
		canvas.NewRectangle(color.NRGBA{0, 0, 0, 0x16}),
	}
	pagina := canvas.NewRectangle(papel)
	objs := []fyne.CanvasObject{sombras[0], sombras[1], sombras[2], pagina, content}
	return &fyne.Container{Layout: folhaLayout{margem}, Objects: objs}
}

type folhaLayout struct{ margem float32 }

func (l folhaLayout) Layout(o []fyne.CanvasObject, s fyne.Size) {
	// three offset, widening rects fake a blurred shadow below the sheet
	for i, d := range []float32{6, 3, 1} {
		o[i].Move(fyne.NewPos(-d/2, d))
		o[i].Resize(s.AddWidthHeight(d, d/2))
	}
	o[3].Move(fyne.NewPos(0, 0))
	o[3].Resize(s)
	o[4].Move(fyne.NewPos(l.margem, l.margem))
	o[4].Resize(s.SubtractWidthHeight(2*l.margem, 2*l.margem))
}

func (l folhaLayout) MinSize(o []fyne.CanvasObject) fyne.Size {
	return o[4].MinSize().AddWidthHeight(2*l.margem, 2*l.margem)
}

// ---- table-of-contents entry ----

// itemSumario is one SUMÁRIO line: "3 RELACIONAMENTO ........ R". It is the
// tool picker and the command list; the dotted leader runs to its shortcut.
// Primary sections are bold caps (ABNT NBR 6027), secondary ones regular. The
// entry in use gets the advisor's red tick and a red number; weight never
// changes, so bold keeps meaning "section level". tocar == nil makes it a plain
// heading line that still shares the number column and rhythm.
type itemSumario struct {
	widget.BaseWidget
	num, rotulo, atalho string
	primario            bool
	ativo, hover        bool
	tocar               func()
}

func novoItem(num, rotulo, atalho string, tocar func()) *itemSumario {
	i := &itemSumario{num: num, rotulo: rotulo, atalho: atalho, tocar: tocar, primario: !strings.Contains(num, ".")}
	if i.primario {
		i.rotulo = strings.ToUpper(rotulo)
	}
	i.ExtendBaseWidget(i)
	return i
}

func (i *itemSumario) Tapped(*fyne.PointEvent) {
	if i.tocar != nil {
		i.tocar()
	}
}
func (i *itemSumario) MouseIn(*desktop.MouseEvent)    { i.setHover(true) }
func (i *itemSumario) MouseMoved(*desktop.MouseEvent) {}
func (i *itemSumario) MouseOut()                      { i.setHover(false) }
func (i *itemSumario) setHover(v bool) {
	if i.tocar != nil {
		i.hover = v
		i.Refresh()
	}
}
func (i *itemSumario) Cursor() desktop.Cursor {
	if i.tocar == nil {
		return desktop.DefaultCursor
	}
	return desktop.PointerCursor
}

func (i *itemSumario) setAtivo(v bool) {
	if i.ativo != v {
		i.ativo = v
		i.Refresh()
	}
}

const (
	tamSumario = 15
	numCol     = 44 // the number column: labels of 1 and 6.1 start at the same x
	tickX      = 2  // the red tick sits in the left margin of the entry
)

func larguraAtalho() float32 { return medir("Ctrl+O", tamSumario, fonteSerif) }

func (i *itemSumario) CreateRenderer() fyne.WidgetRenderer {
	r := &rendItem{i: i, fundo: canvas.NewRectangle(color.Transparent), tick: canvas.NewRectangle(caneta)}
	r.num = canvas.NewText("", tinta)
	r.rot = canvas.NewText("", tinta)
	r.pontos = canvas.NewText("", grafite)
	r.atalho = canvas.NewText("", grafite)
	for _, t := range []*canvas.Text{r.num, r.rot, r.pontos, r.atalho} {
		t.TextSize = tamSumario
	}
	r.Refresh()
	return r
}

type rendItem struct {
	i                        *itemSumario
	fundo, tick              *canvas.Rectangle
	num, rot, pontos, atalho *canvas.Text
}

func (r *rendItem) Objects() []fyne.CanvasObject {
	return []fyne.CanvasObject{r.fundo, r.tick, r.num, r.rot, r.pontos, r.atalho}
}
func (r *rendItem) Destroy() {}

func (r *rendItem) MinSize() fyne.Size {
	return fyne.NewSize(numCol+r.rot.MinSize().Width+24+larguraAtalho()+12, 30)
}

func (r *rendItem) Layout(s fyne.Size) {
	r.fundo.Resize(s)
	y := (s.Height - r.rot.MinSize().Height) / 2
	r.tick.Move(fyne.NewPos(tickX, s.Height/2-1))
	r.tick.Resize(fyne.NewSize(10, 2))
	r.num.Move(fyne.NewPos(16, y))
	r.rot.Move(fyne.NewPos(numCol, y))
	aw := r.atalho.MinSize().Width
	r.atalho.Move(fyne.NewPos(s.Width-6-aw, y)) // right-aligned in the shared column
	r.pontos.Text = ""
	if r.i.atalho == "" {
		return
	}
	// dot leader on a fixed grid, so the dots of every entry line up in columns
	passo := medir(" .", tamSumario, fonteSerif)
	ini := float32(math.Ceil(float64((numCol+r.rot.MinSize().Width+6)/passo))) * passo
	fim := s.Width - 6 - aw - 8 // starts on the grid, stops just before this entry's shortcut
	if n := int((fim - ini) / passo); n > 0 {
		r.pontos.Text = strings.Repeat(" .", n)
	}
	r.pontos.Move(fyne.NewPos(ini, y))
}

func (r *rendItem) Refresh() {
	i := r.i
	r.num.Text, r.rot.Text, r.atalho.Text = i.num, i.rotulo, i.atalho
	r.rot.TextStyle = fyne.TextStyle{Bold: i.primario}
	r.num.TextStyle = r.rot.TextStyle
	r.num.Color, r.rot.Color = tinta, tinta
	r.tick.Hidden = !i.ativo
	if i.ativo {
		r.num.Color = caneta // the advisor ticks the section you are in
	}
	r.fundo.FillColor = color.Transparent
	if i.hover {
		r.fundo.FillColor = marca
		r.rot.Color = caneta
	}
	r.Layout(i.Size())
	for _, o := range r.Objects() {
		o.Refresh()
	}
}

// titulo is an ABNT heading: centered, bold, caps.
func titulo(s string) fyne.CanvasObject {
	return widget.NewLabelWithStyle(strings.ToUpper(s), fyne.TextAlignCenter, fyne.TextStyle{Bold: true})
}

// capa is the cover block on the left sheet: title, subtitle, and a foot line
// with the year (and, in the main window, the MCP status).
func capa(sub string, rodape ...string) fyne.CanvasObject {
	t := canvas.NewText("BRMODELO", tinta)
	t.TextSize, t.TextStyle, t.Alignment = 24, fyne.TextStyle{Bold: true}, fyne.TextAlignCenter
	st := canvas.NewText(sub, tinta)
	st.TextSize, st.TextStyle, st.Alignment = 15, fyne.TextStyle{Italic: true}, fyne.TextAlignCenter
	pe := strconv.Itoa(time.Now().Year())
	for _, r := range rodape {
		pe += " · " + r
	}
	ano := canvas.NewText(pe, grafite)
	ano.TextSize, ano.Alignment = 13, fyne.TextAlignCenter
	return container.NewVBox(espaco(4), t, espaco(2), st, ano, espaco(16))
}

func espaco(h float32) fyne.CanvasObject {
	r := canvas.NewRectangle(color.Transparent)
	r.SetMinSize(fyne.NewSize(0, h))
	return r
}

// ---- catalog card ----

// ficha is the properties box, ruled like a library catalog card: a heavy rule
// on top, its title as the first row, a hairline under every row (a solid ink
// line under input fields, which carry no box of their own) and a heavy rule
// closing it.
func ficha(tit string, linhas ...fyne.CanvasObject) fyne.CanvasObject {
	cab := canvas.NewText(tit, tinta)
	cab.TextSize, cab.TextStyle = 15, fyne.TextStyle{Bold: true}
	objs := []fyne.CanvasObject{cab}
	objs = append(objs, linhas...)
	if _, campoNoFim := objs[len(objs)-1].(*container.ThemeOverride); campoNoFim {
		objs = append(objs, espaco(0)) // so a final field still gets its own rule above the closing one
	}
	regras := make([]fyne.CanvasObject, len(objs)+1)
	for k := range regras {
		l := canvas.NewLine(filete)
		l.StrokeWidth = 0.75
		if k == 0 || k == len(regras)-1 {
			l.StrokeColor, l.StrokeWidth = tinta, 1.75
		} else if _, campo := objs[k-1].(*container.ThemeOverride); campo {
			l.StrokeColor, l.StrokeWidth = tinta, 1
		}
		regras[k] = l
	}
	return &fyne.Container{Layout: fichaLayout{n: len(objs)}, Objects: append(regras, objs...)}
}

// campo strips an input's own box so the card's ruled line is its only frame,
// and sets a small grey label ("Nome", "Tipo") in front of it.
func campo(rotulo string, o fyne.CanvasObject) fyne.CanvasObject {
	r := canvas.NewText(rotulo, grafite)
	r.TextSize = 13
	col := canvas.NewRectangle(color.Transparent)
	col.SetMinSize(fyne.NewSize(44, 0))
	rot := container.NewStack(col, container.NewVBox(layout.NewSpacer(), r, layout.NewSpacer()))
	pad := canvas.NewRectangle(color.Transparent)
	pad.SetMinSize(fyne.NewSize(theme.InnerPadding(), 0)) // the row is pulled left by this much
	return container.NewThemeOverride(container.NewBorder(nil, nil, container.NewHBox(pad, rot), nil, o), temaCampo{})
}

type temaCampo struct{ temaABNT }

func (temaCampo) Color(n fyne.ThemeColorName, v fyne.ThemeVariant) color.Color {
	if n == theme.ColorNameInputBackground || n == theme.ColorNameButton {
		return papel
	}
	return temaABNT{}.Color(n, v)
}

func (temaCampo) Size(n fyne.ThemeSizeName) float32 {
	if n == theme.SizeNameInputBorder {
		return 0
	}
	return temaABNT{}.Size(n)
}

// nota is an italic remark on the card, set flush with the card's text
// (a Label would indent it by its inner padding); \n breaks lines.
func nota(s string) fyne.CanvasObject { return linhasItalicas(strings.Split(s, "\n"), tinta) }

// notaCaneta writes a remark in the advisor's red, wrapped at about 32 characters.
func notaCaneta(s string) fyne.CanvasObject { return linhasItalicas(quebrar(s, 32), caneta) }

func linhasItalicas(linhas []string, c color.Color) fyne.CanvasObject {
	box := &fyne.Container{Layout: layout.NewCustomPaddedVBoxLayout(2)}
	for _, l := range linhas {
		t := canvas.NewText(l, c)
		t.TextSize, t.TextStyle = 15, fyne.TextStyle{Italic: true}
		box.Add(t)
	}
	return box
}

// quebrar word-wraps s to lines of at most n characters.
func quebrar(s string, n int) []string {
	var linhas []string
	atual := ""
	for _, p := range strings.Fields(s) {
		if atual != "" && len([]rune(atual))+1+len([]rune(p)) > n {
			linhas, atual = append(linhas, atual), p
			continue
		}
		atual = strings.TrimSpace(atual + " " + p)
	}
	return append(linhas, atual)
}

type fichaLayout struct{ n int }

const (
	fichaPad = 6
	fichaFim = 6
)

func (l fichaLayout) Layout(o []fyne.CanvasObject, s fyne.Size) {
	regras, linhas := o[:l.n+1], o[l.n+1:]
	y := float32(0)
	for k, c := range linhas {
		regua := regras[k].(*canvas.Line)
		regua.Position1, regua.Position2 = fyne.NewPos(0, y), fyne.NewPos(s.Width, y)
		y += fichaPad
		h := c.MinSize().Height
		x := float32(0)
		if _, ok := c.(*container.ThemeOverride); ok {
			x = -theme.InnerPadding() // let the field's text align with the card's text
		}
		c.Move(fyne.NewPos(x, y))
		c.Resize(fyne.NewSize(s.Width-x, h))
		y += h + fichaPad
	}
	y += fichaFim // keep the closing rule clear of the last field's own rule
	ult := regras[l.n].(*canvas.Line)
	ult.Position1, ult.Position2 = fyne.NewPos(0, y), fyne.NewPos(s.Width, y)
}

func (l fichaLayout) MinSize(o []fyne.CanvasObject) fyne.Size {
	var w, h float32
	for _, c := range o[l.n+1:] {
		w = max(w, c.MinSize().Width)
		h += c.MinSize().Height + 2*fichaPad
	}
	return fyne.NewSize(w, h+fichaFim+2)
}
