package dominio

import "math"

// Ponto is a position on the diagram, in pixels.
type Ponto struct{ X, Y float32 }

// Centro is where links attach: the middle of the shape, or of an attribute's circle.
func Centro(f *Forma) Ponto {
	if f.Tipo == Atributo {
		return Ponto{f.X + 5, f.Y + f.H/2}
	}
	return Ponto{f.X + f.W/2, f.Y + f.H/2}
}

// Borda is where the segment from c towards p leaves a shape of half-size hw x hh
// (a rectangle, or a diamond when losango).
func Borda(c, p Ponto, hw, hh float32, losango bool) Ponto {
	dx, dy := float64(p.X-c.X), float64(p.Y-c.Y)
	if dx == 0 && dy == 0 {
		return c
	}
	s := math.Min(float64(hw)/math.Abs(dx), float64(hh)/math.Abs(dy))
	if losango {
		s = 1 / (math.Abs(dx)/float64(hw) + math.Abs(dy)/float64(hh))
	}
	return Ponto{c.X + float32(dx*s), c.Y + float32(dy*s)}
}

// Dentro tells whether p is inside the drawn outline of f.
func Dentro(f *Forma, p Ponto) bool {
	c := Centro(f)
	dx, dy := math.Abs(float64(p.X-c.X)), math.Abs(float64(p.Y-c.Y))
	switch f.Tipo {
	case Atributo:
		return dx*dx+dy*dy <= 25
	case Relacionamento:
		return dx/float64(f.W/2)+dy/float64(f.H/2) <= 1
	}
	return dx <= float64(f.W/2) && dy <= float64(f.H/2)
}

// Sair is where the segment de->ate leaves shape f (bisection; de should be inside f).
func Sair(f *Forma, de, ate Ponto) Ponto {
	if !Dentro(f, de) {
		return de
	}
	lo, hi := float32(0), float32(1)
	for range 20 {
		mid := (lo + hi) / 2
		if Dentro(f, Ponto{de.X + (ate.X-de.X)*mid, de.Y + (ate.Y-de.Y)*mid}) {
			lo = mid
		} else {
			hi = mid
		}
	}
	return Ponto{de.X + (ate.X-de.X)*lo, de.Y + (ate.Y-de.Y)*lo}
}

// Pontas are a link's end points clipped to the shapes' outlines. Links joining
// the same two shapes (a self-relationship) are spread apart so both stay visible.
func (d *Diagrama) Pontas(l *Ligacao) (pa, pb Ponto) {
	fa, fb := d.Forma(l.A), d.Forma(l.B)
	ca, cb := Centro(fa), Centro(fb)
	k, n := 0, 0
	for _, o := range d.Ligacoes {
		if (o.A == l.A && o.B == l.B) || (o.A == l.B && o.B == l.A) {
			if o == l {
				k = n
			}
			n++
		}
	}
	if n > 1 {
		dx, dy := cb.X-ca.X, cb.Y-ca.Y
		dist := float32(math.Hypot(float64(dx), float64(dy)))
		off := (float32(k) - float32(n-1)/2) * 24 / max(dist, 1)
		ca, cb = Ponto{ca.X - dy*off, ca.Y + dx*off}, Ponto{cb.X - dy*off, cb.Y + dx*off}
	}
	return Sair(fa, ca, cb), Sair(fb, cb, ca)
}

// DistSeg is the distance from p to the segment a-c.
func DistSeg(p, a, c Ponto) float64 {
	dx, dy := float64(c.X-a.X), float64(c.Y-a.Y)
	t := 0.0
	if l2 := dx*dx + dy*dy; l2 > 0 {
		t = math.Max(0, math.Min(1, (float64(p.X-a.X)*dx+float64(p.Y-a.Y)*dy)/l2))
	}
	return math.Hypot(float64(p.X)-float64(a.X)-t*dx, float64(p.Y)-float64(a.Y)-t*dy)
}

// FormaEm is the topmost shape whose box contains p.
func (d *Diagrama) FormaEm(p Ponto) *Forma {
	for i := len(d.Formas) - 1; i >= 0; i-- {
		f := d.Formas[i]
		if p.X >= f.X && p.X <= f.X+f.W && p.Y >= f.Y && p.Y <= f.Y+f.H {
			return f
		}
	}
	return nil
}

// LigacaoEm is a link passing within 5px of p.
func (d *Diagrama) LigacaoEm(p Ponto) *Ligacao {
	for _, l := range d.Ligacoes {
		a, c := d.Pontas(l)
		if DistSeg(p, a, c) < 5 {
			return l
		}
	}
	return nil
}

// Ocupado tells whether a's box overlaps another shape or a link it isn't part of.
func (d *Diagrama) Ocupado(a *Forma) bool {
	for _, o := range d.Formas {
		if o != a && a.X < o.X+o.W && o.X < a.X+a.W && a.Y < o.Y+o.H+4 && o.Y < a.Y+a.H+4 {
			return true
		}
	}
	for _, l := range d.Ligacoes {
		if l.A == a.ID || l.B == a.ID {
			continue
		}
		pa, pb := d.Pontas(l)
		for i := range 51 { // ponytail: sampled segment-vs-rect test, plenty for screen pixels
			x, y := pa.X+(pb.X-pa.X)*float32(i)/50, pa.Y+(pb.Y-pa.Y)*float32(i)/50
			if x > a.X-3 && x < a.X+a.W+3 && y > a.Y-4 && y < a.Y+a.H+4 {
				return true
			}
		}
	}
	return false
}

// Anexar places attribute a on the side of f nearest p (like the Java app),
// slides it until the spot is free, and links it to f.
func (d *Diagrama) Anexar(a, f *Forma, p Ponto) {
	dl, dr, dt, db := p.X-f.X, f.X+f.W-p.X, p.Y-f.Y, f.Y+f.H-p.Y
	var passo float32 = 18
	switch min(dl, dr, dt, db) {
	case dt:
		a.X, a.Y, passo = p.X-5, f.Y-30, -18
	case db:
		a.X, a.Y = p.X-5, f.Y+f.H+16
	case dl:
		a.X, a.Y = f.X-a.W-20, p.Y-a.H/2
	default:
		a.X, a.Y = f.X+f.W+20, p.Y-a.H/2
	}
	for range 20 {
		if !d.Ocupado(a) {
			break
		}
		a.Y += passo
	}
	d.Ligar(f, a)
}

// PosicionarEntre centers relationship r in the gap between e1 and e2, or
// beside e1 when it is a self-relationship (e1 == e2).
func PosicionarEntre(r, e1, e2 *Forma) {
	if e1 == e2 {
		r.X, r.Y = e1.X+e1.W+60, e1.Y+(e1.H-r.H)/2
		return
	}
	gap := func(p1, s1, p2, s2 float32) float32 {
		if p1 <= p2 {
			return (p1 + s1 + p2) / 2
		}
		return (p2 + s2 + p1) / 2
	}
	r.X = gap(e1.X, e1.W, e2.X, e2.W) - r.W/2
	r.Y = gap(e1.Y, e1.H, e2.Y, e2.H) - r.H/2
}

// Desviar moves a freshly placed shape up or down, in growing steps, until it
// sits on free paper (it stays put when nothing nearby is free).
func (d *Diagrama) Desviar(f *Forma) {
	y0 := f.Y
	for _, dy := range []float32{0, 70, -70, 140, -140, 210, -210} {
		f.Y = y0 + dy
		if !d.Ocupado(f) {
			return
		}
	}
	f.Y = y0
}

// Caixa is the bounding box of every shape (ok is false for an empty diagram).
func (d *Diagrama) Caixa() (x0, y0, x1, y1 float32, ok bool) {
	if len(d.Formas) == 0 {
		return 0, 0, 0, 0, false
	}
	x0, y0, x1, y1 = math.MaxFloat32, math.MaxFloat32, -math.MaxFloat32, -math.MaxFloat32
	for _, f := range d.Formas {
		x0, y0 = min(x0, f.X), min(y0, f.Y)
		x1, y1 = max(x1, f.X+f.W), max(y1, f.Y+f.H)
	}
	return x0, y0, x1, y1, true
}

// LivreParaEntidade finds a spot for a new entity when no position is given:
// left to right in rows 260px apart, within the first 760px so it lands on
// the visible part of the figure sheet, skipping anything already there.
func (d *Diagrama) LivreParaEntidade(w, h float32) Ponto {
	const passoX, passoY, larguraLinha = 320, 260, 780 // 200px between entities: room for a relationship
	for y := float32(120); ; y += passoY {
		for x := float32(120); x+w <= larguraLinha; x += passoX {
			teste := &Forma{X: x - 80, Y: y - 80, W: w + 160, H: h + 160} // keep room for attributes around it
			if !d.sobrepoe(teste) {
				return Ponto{x, y}
			}
		}
	}
}

func (d *Diagrama) sobrepoe(a *Forma) bool {
	for _, o := range d.Formas {
		if a.X < o.X+o.W && o.X < a.X+a.W && a.Y < o.Y+o.H && o.Y < a.Y+a.H {
			return true
		}
	}
	return false
}
