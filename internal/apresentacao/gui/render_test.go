package gui

import (
	"context"
	"encoding/json"
	"time"

	"encoding/xml"
	"github.com/brmodelo/gobrmodelo/internal/infra/llm"
	"github.com/brmodelo/gobrmodelo/internal/infra/segredos"
	"github.com/zalando/go-keyring"
	"image/png"
	"os"
	"testing"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/test"

	"github.com/brmodelo/gobrmodelo/internal/aplicacao"
	dom "github.com/brmodelo/gobrmodelo/internal/dominio"
)

func snap(t *testing.T, c fyne.Canvas, nome string) {
	f, err := os.Create(os.Getenv("SNAP_DIR") + "/" + nome + ".png")
	if err != nil {
		t.Skip("SNAP_DIR not set")
	}
	defer f.Close()
	png.Encode(f, c.Capture())
}

func TestRender(t *testing.T) {
	test.NewApp()
	w := test.NewWindow(nil)
	keyring.MockInit() // never touch the real keyring
	b := Montar(w, aplicacao.Nova(), "MCP 127.0.0.1:7421/mcp", segredos.Novo())
	w.Resize(fyne.NewSize(1600, 860))
	snap(t, w.Canvas(), "vazio")
	off := fyne.CurrentApp().Driver().AbsolutePositionForObject(b) // taps below are board coordinates

	tap := func(modo string, x, y float32) {
		b.modo = modo
		test.TapCanvas(w.Canvas(), fyne.NewPos(x, y).Add(off))
	}
	tap(string(dom.Entidade), 100, 100)
	tap(string(dom.Entidade), 500, 100)
	tap(string(dom.Atributo), 100, 250)
	tap(modoLigar, 110, 110) // entidade -> entidade: relationship created between them
	tap(modoLigar, 510, 110)
	tap(modoLigar, 510, 140) // entidade -> itself: self-relationship
	tap(modoLigar, 520, 140)
	tap(string(dom.Atributo), 215, 130) // on the entity's right half: attached to it
	tap(string(dom.Atributo), 215, 130)
	tap(string(dom.Atributo), 215, 130)
	tap(string(dom.Atributo), 150, 102) // near the top edge
	b.modo = modoSel
	tap(modoSel, 150, 130) // select the first entity: panel shows its name
	snap(t, w.Canvas(), "conceitual")

	// like the glfw driver: Position is the current pointer, Dragged the delta since press
	b.Dragged(&fyne.DragEvent{PointEvent: fyne.PointEvent{Position: fyne.NewPos(520, 270)}, Dragged: fyne.Delta{DY: 150}})
	b.DragEnd()
	snap(t, w.Canvas(), "arrastado")

	// selection rectangle around the left entity and its attributes, then drag the group
	drag := func(x0, y0, x1, y1 float32) {
		b.Dragged(&fyne.DragEvent{PointEvent: fyne.PointEvent{Position: fyne.NewPos(x1, y1)}, Dragged: fyne.Delta{DX: x1 - x0, DY: y1 - y0}})
	}
	b.modo = modoSel
	drag(50, 40, 330, 230)
	snap(t, w.Canvas(), "area")
	b.DragEnd()
	if len(b.grupo) != 5 {
		t.Fatalf("grupo = %d, want entity + 4 attributes", len(b.grupo))
	}
	ent := b.s.D.Formas[0]
	x := ent.X
	drag(150, 130, 150, 330)
	b.DragEnd()
	if ent.X != x || ent.Y != 300 {
		t.Fatalf("entity not moved with the group: %+v", ent)
	}
	snap(t, w.Canvas(), "grupo")

	// error state: linking two attributes gets a red-pen note in the margin
	var attrs []*dom.Forma
	for _, f := range b.s.D.Formas {
		if f.Tipo == dom.Atributo {
			attrs = append(attrs, f)
		}
	}
	tap(modoLigar, attrs[1].X+5, attrs[1].Y+attrs[1].H/2)
	tap(modoLigar, attrs[2].X+5, attrs[2].Y+attrs[2].H/2)
	if b.nota == "" || b.notaEm != attrs[2] {
		t.Fatalf("expected a margin note on the target attribute, got %q", b.nota)
	}
	snap(t, w.Canvas(), "erro")

	attrs[2].TipoAtributo = "VARCHAR(80)"
	tap(modoSel, attrs[2].X+20, attrs[2].Y+attrs[2].H/2) // the card shows Nome, Tipo and Identificador
	snap(t, w.Canvas(), "atributo")

	b.avisar(erroAbrir("modelo.xml", &xml.SyntaxError{Line: 3, Msg: "unexpected EOF"}))
	snap(t, w.Canvas(), "erro_arquivo")

	// a change from another goroutine (what the MCP server does) runs on the UI
	// thread through the session and shows up in the window
	feito := make(chan error)
	go func() {
		feito <- b.s.Fazer(func() error {
			e := b.s.CriarEntidade("Cliente", nil)
			_, err := b.s.AdicionarAtributo(e, "cpf", "VARCHAR(11)", true, nil)
			return err
		})
	}()
	if err := <-feito; err != nil {
		t.Fatal(err)
	}
	if f, err := b.s.Achar("cpf"); err != nil || !f.Identificador {
		t.Fatal("MCP-style change did not reach the diagram", err)
	}
	snap(t, w.Canvas(), "mcp")

	// the ORIENTADOR sheet: a request goes to the model, its tools edit the
	// diagram, the transcript logs each call and what it created is marked red
	o := b.orientador
	o.cli, o.modelo = &modeloFalso{turnos: []llm.Resposta{
		{Texto: "Vou criar Autor e ligá-lo a Cliente.", Chamadas: []llm.Chamada{
			{ID: "1", Nome: "criar_entidade", Argumentos: json.RawMessage(`{"nome":"Autor"}`)},
			{ID: "2", Nome: "adicionar_atributo", Argumentos: json.RawMessage(`{"dono":"Autor","nome":"codigo","tipo":"INT","identificador":true}`)},
			{ID: "3", Nome: "criar_relacionamento", Argumentos: json.RawMessage(`{"entidade_a":"Autor","entidade_b":"Cliente","nome":"atende"}`)},
			{ID: "4", Nome: "adicionar_atributo", Argumentos: json.RawMessage(`{"dono":"Editora","nome":"x"}`)},
		}},
		{Texto: "Pronto. Não existe Editora no diagrama, então pulei esse atributo."},
	}}, "fake"
	o.telaConectado([]string{"fake"}, true)
	o.pedido.SetText("crie Autor com código e ligue a Cliente")
	o.enviarPedido()
	for i := 0; o.cancelar != nil && i < 200; i++ {
		time.Sleep(10 * time.Millisecond)
	}
	if o.cancelar != nil {
		t.Fatal("request did not finish")
	}
	if _, err := b.s.Achar("Autor"); err != nil {
		t.Fatal("the panel's request did not reach the diagram:", err)
	}
	if len(b.grupo) != 3 { // Autor, codigo, atende
		t.Fatalf("created elements not marked: %v", b.grupo)
	}
	snap(t, w.Canvas(), "orientador")

	// Ctrl+Z undoes the whole assistant request in one step; Ctrl+Y brings it back
	formas := len(b.s.D.Formas)
	b.desfazer()
	if _, err := b.s.Achar("Autor"); err == nil || len(b.s.D.Formas) != formas-3 {
		t.Fatalf("one undo should remove the request's 3 elements: %d -> %d", formas, len(b.s.D.Formas))
	}
	b.refazer()
	if _, err := b.s.Achar("Autor"); err != nil {
		t.Fatal("redo")
	}

	// a drag is one undo step, however many motion events it has
	autor, _ := b.s.Achar("Autor")
	x0 := autor.X
	b.Dragged(&fyne.DragEvent{PointEvent: fyne.PointEvent{Position: fyne.NewPos(autor.X+10, autor.Y+10)}, Dragged: fyne.Delta{}})
	for range 5 {
		b.Dragged(&fyne.DragEvent{PointEvent: fyne.PointEvent{Position: fyne.NewPos(autor.X+10, autor.Y+10)}, Dragged: fyne.Delta{DX: 10}})
	}
	b.DragEnd()
	autor, _ = b.s.Achar("Autor")
	if autor.X != x0+50 {
		t.Fatalf("drag moved %v", autor.X-x0)
	}
	b.desfazer()
	if autor, _ = b.s.Achar("Autor"); autor.X != x0 {
		t.Fatalf("one undo should return the whole drag: %v", autor.X-x0)
	}

	// zoom: 150 %, a click lands where the diagram is, not where the screen is
	b.zm.definir(1.5)
	b.modo = string(dom.Entidade)
	test.TapCanvas(w.Canvas(), fyne.NewPos(440*1.5, 470*1.5).Add(off)) // inside the visible sheet
	nova := b.s.D.Formas[len(b.s.D.Formas)-1]
	if nova.Tipo != dom.Entidade || abs(nova.X-440) > 1 || abs(nova.Y-470) > 1 {
		t.Fatalf("click at zoom 150%% created at %v,%v", nova.X, nova.Y)
	}
	b.modo = modoSel
	snap(t, w.Canvas(), "zoom")
	b.desfazer()
	b.zm.definir(1)

	attrs[2].Identificador = true // a key, so the logical model has FKs to show
	l, _ := dom.Converter(b.s.D)
	lw := test.NewWindow(nil)
	centralizar(l, larguraFigura)
	q := montarLogico(lw, l, "Fonte: convertido da Figura 1 pelas regras do brModelo.", nil)
	lw.Resize(fyne.NewSize(1200, 760))
	snap(t, lw.Canvas(), "logico")

	q.perguntarSQL() // the card asks which database before generating anything
	snap(t, lw.Canvas(), "sql")
	if len(b.s.D.Ligacoes) != 12 { // 8 by clicks + cpf from "outside" + 3 by the ORIENTADOR
		t.Fatalf("atributo should be linked: %d ligacoes", len(b.s.D.Ligacoes))
	}
}

type modeloFalso struct{ turnos []llm.Resposta }

func (m *modeloFalso) Responder(context.Context, string, string, []llm.Mensagem, []llm.Ferramenta) (llm.Resposta, error) {
	t := m.turnos[0]
	m.turnos = m.turnos[1:]
	return t, nil
}

func (m *modeloFalso) Modelos(context.Context) ([]string, error) { return []string{"fake"}, nil }
