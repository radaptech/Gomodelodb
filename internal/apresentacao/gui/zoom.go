package gui

// Zoom for the figure sheets. Figures are drawn in diagram coordinates and
// then scaled as a whole; pointer positions are divided back, so the diagram
// itself (and the saved file) never changes with the zoom.
//
// There is no Ctrl+wheel: a figure that implements fyne.Scrollable gets its
// own clip region in Fyne's painter, which overrides the sheet's scroll clip
// and lets the drawing spill past the paper.

import (
	"fmt"
	"image/color"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/canvas"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/driver/desktop"
	"fyne.io/fyne/v2/layout"
	"fyne.io/fyne/v2/widget"
)

const (
	zoomMin   = 0.25
	zoomMax   = 3
	zoomPasso = 1.25
)

type zoom struct {
	z       float32
	rotulo  *widget.Label
	aplicar func() // redraw the figure after a change
}

// novoZoom returns the zoom state and its "− 100 % +" control.
func novoZoom(aplicar func()) (*zoom, fyne.CanvasObject) {
	zm := &zoom{z: 1, rotulo: widget.NewLabel("100 %"), aplicar: aplicar}
	menos := widget.NewButton("−", func() { zm.definir(zm.z / zoomPasso) })
	mais := widget.NewButton("+", func() { zm.definir(zm.z * zoomPasso) })
	for _, b := range []*widget.Button{menos, mais} {
		b.Importance = widget.LowImportance
	}
	return zm, container.NewHBox(menos, zm.rotulo, mais)
}

func (zm *zoom) definir(z float32) {
	z = min(zoomMax, max(zoomMin, z))
	if abs(z-1) < 0.02 {
		z = 1 // land exactly on 100 % after a round trip of steps
	}
	if z == zm.z {
		return
	}
	zm.z = z
	zm.rotulo.SetText(fmt.Sprintf("%.0f %%", z*100))
	zm.aplicar()
}

// atalhos binds Ctrl+= / Ctrl+- / Ctrl+0 on window w.
func (zm *zoom) atalhos(w fyne.Window) {
	for k, f := range map[fyne.KeyName]func(){
		fyne.KeyEqual: func() { zm.definir(zm.z * zoomPasso) },
		fyne.KeyMinus: func() { zm.definir(zm.z / zoomPasso) },
		fyne.Key0:     func() { zm.definir(1) },
	} {
		w.Canvas().AddShortcut(&desktop.CustomShortcut{KeyName: k, Modifier: fyne.KeyModifierShortcutDefault}, func(fyne.Shortcut) { f() })
	}
}

// modelo converts a pointer position on the figure to diagram coordinates.
func (zm *zoom) modelo(p fyne.Position) fyne.Position { return fyne.NewPos(p.X/zm.z, p.Y/zm.z) }

// escalar scales a drawn figure by z around the origin. The first object is
// the paper background and keeps its size: scaled, it spills past the sheet.
func escalar(objs []fyne.CanvasObject, z float32) {
	if z == 1 || len(objs) < 2 {
		return
	}
	mul := func(p fyne.Position) fyne.Position { return fyne.NewPos(p.X*z, p.Y*z) }
	for _, o := range objs[1:] {
		switch x := o.(type) {
		case *canvas.Text:
			x.Move(mul(x.Position()))
			x.TextSize *= z
		case *canvas.Line:
			x.Position1, x.Position2 = mul(x.Position1), mul(x.Position2)
			x.StrokeWidth = max(0.75, x.StrokeWidth*z)
		case *canvas.Rectangle:
			x.Move(mul(x.Position()))
			x.Resize(fyne.NewSize(x.Size().Width*z, x.Size().Height*z))
			x.StrokeWidth, x.CornerRadius = x.StrokeWidth*z, x.CornerRadius*z
		case *canvas.Circle:
			x.Move(mul(x.Position()))
			x.Resize(fyne.NewSize(x.Size().Width*z, x.Size().Height*z))
			x.StrokeWidth = max(0.75, x.StrokeWidth*z)
		}
	}
}

// comControle lays the zoom control over the bottom-right corner of a figure sheet.
func comControle(figura, controle fyne.CanvasObject) fyne.CanvasObject {
	fundo := canvas.NewRectangle(color.NRGBA{0xff, 0xff, 0xff, 0xe6}) // paper, so the figure doesn't show through
	canto := container.NewStack(fundo, container.NewPadded(controle))
	return container.NewStack(figura,
		container.NewBorder(nil, container.NewHBox(layout.NewSpacer(), container.NewPadded(canto)), nil, nil))
}
