package gui

// The ORIENTADOR sheet: a chat with a language model that edits the diagram.
// It reads like a terminal (your request after a prompt mark, one line per
// tool the model runs) set in the monograph's world: the model's replies are
// the advisor's notes, in red pen, and what it created is marked in red on
// the figure when it finishes.

import (
	"context"
	"fmt"
	"image/color"
	"strings"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/canvas"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/widget"

	"github.com/brmodelo/gobrmodelo/internal/apresentacao/assistente"
	"github.com/brmodelo/gobrmodelo/internal/infra/llm"
	"github.com/brmodelo/gobrmodelo/internal/infra/segredos"
)

const larguraOrientador = 360

type orientador struct {
	b     *board
	a     *assistente.Assistente
	cofre *segredos.Cofre

	prov     llm.Provedor
	cli      llm.Cliente
	modelo   string
	cancelar context.CancelFunc // non-nil while a request runs

	conexao *fyne.Container // the connection card's slot
	linhas  *fyne.Container // transcript
	rolagem *container.Scroll
	pedido  *widget.Entry
	enviar  *widget.Button
	pagina  *container.Scroll
}

// novoOrientador builds the sheet. a may be nil when the assistant failed to start.
func novoOrientador(b *board, a *assistente.Assistente, cofre *segredos.Cofre, erroInicio error) (*orientador, fyne.CanvasObject) {
	o := &orientador{b: b, a: a, cofre: cofre, prov: llm.Provedores[0]}
	o.conexao = container.NewStack()
	o.linhas = container.NewVBox()
	o.rolagem = container.NewVScroll(o.linhas)
	o.pedido = widget.NewEntry()
	o.pedido.SetPlaceHolder("Peça algo ao orientador…")
	o.pedido.OnSubmitted = func(string) { o.enviarPedido() }
	o.enviar = widget.NewButton("Enviar", o.enviarPedido)
	limpar := widget.NewButton("Nova conversa", func() {
		if o.a != nil {
			o.a.Limpar()
		}
		o.linhas.Objects = nil
		o.linhas.Refresh()
	})

	topo := container.NewVBox(titulo("Orientador"), o.conexao)
	base := container.NewVBox(campo("›", o.pedido), container.NewHBox(o.enviar, limpar))
	o.pagina = container.NewVScroll(container.NewBorder(topo, base, nil, nil, o.rolagem))
	largura := canvas.NewRectangle(color.Transparent)
	largura.SetMinSize(fyne.NewSize(larguraOrientador, 0))
	folhaO := folha(container.NewStack(largura, o.pagina), 22)

	switch {
	case erroInicio != nil:
		o.mostrarConexao(ficha("Ficha – Atenção", notaCaneta("O assistente não iniciou: "+erroInicio.Error())))
		o.enviar.Disable()
	default:
		o.telaConexao("")
		if o.cofre.Ler(o.prov.Nome) != "" || !o.prov.PrecisaChave {
			o.conectar(o.cofre.Ler(o.prov.Nome)) // a saved key: connect without asking
		}
	}
	return o, folhaO
}

func (o *orientador) mostrarConexao(c fyne.CanvasObject) {
	o.conexao.Objects = []fyne.CanvasObject{c}
	o.pagina.Refresh()
}

// telaConexao shows the provider picker and the key field (or the error erro).
func (o *orientador) telaConexao(erro string) {
	linhas := []fyne.CanvasObject{campo("LLM", o.seletorProvedor())}
	if o.prov.PrecisaChave {
		chave := widget.NewPasswordEntry()
		chave.SetPlaceHolder("cole a chave de API")
		chave.OnSubmitted = func(k string) { o.conectar(k) }
		linhas = append(linhas, campo("Chave", chave),
			container.NewHBox(widget.NewButton("Entrar", func() { o.conectar(chave.Text) })),
			nota("Crie a chave em "+o.prov.ChaveURL+".\nEla fica no chaveiro do sistema."))
	} else {
		linhas = append(linhas, container.NewHBox(widget.NewButton("Conectar", func() { o.conectar("") })),
			nota("Roda no seu computador, sem conta.\nInstale um modelo com suporte a\nferramentas (ex.: ollama pull qwen3)."))
	}
	if erro != "" {
		linhas = append(linhas, notaCaneta(erro))
	}
	o.mostrarConexao(ficha("Ficha – Conexão", linhas...))
	o.atualizarEnviar()
}

// conectar validates the key by listing the provider's models, in the background.
func (o *orientador) conectar(chave string) {
	chave = strings.TrimSpace(chave)
	if o.prov.PrecisaChave && chave == "" {
		o.telaConexao("Cole a chave de API para entrar.")
		return
	}
	o.mostrarConexao(ficha("Ficha – Conexão", nota("Conectando ao "+o.prov.Nome+"…")))
	prov, cli := o.prov, o.prov.Conectar(chave)
	go func() {
		modelos, err := cli.Modelos(context.Background())
		fyne.Do(func() {
			if prov.Nome != o.prov.Nome { // switched provider meanwhile
				return
			}
			if err == nil && len(modelos) == 0 {
				err = fmt.Errorf("%s não tem modelos disponíveis para esta conta", prov.Nome)
			}
			if err != nil {
				o.telaConexao(err.Error())
				return
			}
			persistente := true
			if prov.PrecisaChave {
				persistente = o.cofre.Guardar(prov.Nome, chave)
			}
			o.cli, o.modelo = cli, prov.Padrao(modelos)
			o.telaConectado(modelos, persistente)
		})
	}()
}

func (o *orientador) telaConectado(modelos []string, persistente bool) {
	modelo := widget.NewSelect(modelos, func(m string) { o.modelo = m })
	modelo.SetSelected(o.modelo)
	estado := "Conectado."
	switch {
	case !o.prov.PrecisaChave:
		estado = "Conectado ao Ollama local."
	case persistente:
		estado = "Conectado. Chave no chaveiro."
	default:
		estado = "Conectado. Sem chaveiro: a chave\nvale só até fechar o app."
	}
	linhas := []fyne.CanvasObject{campo("LLM", o.seletorProvedor()), campo("Modelo", modelo), nota(estado)}
	if o.prov.PrecisaChave {
		linhas = append(linhas, container.NewHBox(widget.NewButton("Sair e esquecer a chave", func() {
			o.cofre.Esquecer(o.prov.Nome)
			o.cli, o.modelo = nil, ""
			o.telaConexao("")
		})))
	}
	o.mostrarConexao(ficha("Ficha – Conexão", linhas...))
	o.atualizarEnviar()
	o.b.win.Canvas().Focus(o.pedido)
}

// seletorProvedor switches provider; a saved key (or none needed) connects right away.
func (o *orientador) seletorProvedor() *widget.Select {
	nomes := make([]string, len(llm.Provedores))
	for i, p := range llm.Provedores {
		nomes[i] = p.Nome
	}
	sel := widget.NewSelect(nomes, nil)
	sel.SetSelected(o.prov.Nome)
	sel.OnChanged = func(n string) {
		if n == o.prov.Nome {
			return
		}
		o.prov, _ = llm.ProvedorPorNome(n)
		o.cli, o.modelo = nil, ""
		if k := o.cofre.Ler(n); k != "" || !o.prov.PrecisaChave {
			o.conectar(k)
			return
		}
		o.telaConexao("")
	}
	return sel
}

func (o *orientador) atualizarEnviar() {
	switch {
	case o.cancelar != nil:
		o.enviar.SetText("Parar")
		o.enviar.Enable()
	case o.a == nil || o.cli == nil:
		o.enviar.SetText("Enviar")
		o.enviar.Disable()
	default:
		o.enviar.SetText("Enviar")
		o.enviar.Enable()
	}
}

// ---- transcript ----

func (o *orientador) adicionar(c fyne.CanvasObject) {
	o.linhas.Add(c)
	o.rolagem.ScrollToBottom()
}

func paragrafo(s string, estilo fyne.TextStyle, importancia widget.Importance) fyne.CanvasObject {
	l := widget.NewLabelWithStyle(s, fyne.TextAlignLeading, estilo)
	l.Wrapping = fyne.TextWrapWord
	l.Importance = importancia
	return l
}

// linhaPedido: your request, after the prompt mark.
func linhaPedido(s string) fyne.CanvasObject {
	return container.NewBorder(nil, nil, paragrafo("›", fyne.TextStyle{Bold: true}, widget.MediumImportance), nil,
		paragrafo(s, fyne.TextStyle{Bold: true}, widget.MediumImportance))
}

// linhaFerramenta: one tool call, as a terminal would log it.
func linhaFerramenta(e assistente.Evento) fyne.CanvasObject {
	cor := grafite
	if e.Falhou {
		cor = caneta
	}
	ponto := canvas.NewCircle(cor)
	if e.Falhou {
		ponto.FillColor, ponto.StrokeColor, ponto.StrokeWidth = color.Transparent, caneta, 1.25
	}
	ponto.Resize(fyne.NewSize(6, 6))
	marca := container.NewWithoutLayout(ponto)
	ponto.Move(fyne.NewPos(10, 8))
	espaco := canvas.NewRectangle(color.Transparent)
	espaco.SetMinSize(fyne.NewSize(22, 20))
	txt := e.Ferramenta
	if e.Resumo != "" {
		txt += " · " + e.Resumo
	}
	if e.Falhou {
		txt += " (falhou)"
	}
	t := canvas.NewText(txt, cor)
	t.TextSize, t.FontSource = 12, fonteSans
	return container.NewBorder(nil, nil, container.NewStack(espaco, marca), nil, t)
}

// linhaOrientador: the model's reply, the advisor's note in red pen.
func linhaOrientador(s string) fyne.CanvasObject {
	return paragrafo(s, fyne.TextStyle{Italic: true}, widget.DangerImportance)
}

func (o *orientador) enviarPedido() {
	if o.cancelar != nil { // the button reads "Parar"
		o.cancelar()
		return
	}
	texto := strings.TrimSpace(o.pedido.Text)
	if texto == "" || o.cli == nil || o.a == nil {
		return
	}
	o.pedido.SetText("")
	o.adicionar(linhaPedido(texto))
	pensando := paragrafo("o orientador está trabalhando…", fyne.TextStyle{Italic: true}, widget.LowImportance)
	o.adicionar(pensando)

	ctx, cancelar := context.WithCancel(context.Background())
	o.cancelar = cancelar
	o.b.s.IniciarGrupo() // the whole request is one Ctrl+Z, however many tools it runs
	o.atualizarEnviar()
	cli, modelo := o.cli, o.modelo
	criados := map[int]bool{}

	go func() {
		err := o.a.Enviar(ctx, cli, modelo, texto, func(e assistente.Evento) {
			fyne.Do(func() {
				o.linhas.Remove(pensando)
				if e.Ferramenta != "" {
					o.adicionar(linhaFerramenta(e))
					if !e.Falhou && e.ID != 0 && o.b.s.D.Forma(e.ID) != nil {
						criados[e.ID] = true
					}
				} else {
					o.adicionar(linhaOrientador(e.Texto))
				}
				o.adicionar(pensando)
			})
		})
		fyne.Do(func() {
			o.linhas.Remove(pensando)
			switch {
			case ctx.Err() != nil:
				o.adicionar(linhaOrientador("Pedido interrompido."))
			case err != nil:
				o.adicionar(linhaOrientador(err.Error()))
			}
			o.cancelar = nil
			cancelar()
			o.b.s.FimGrupo()
			o.atualizarEnviar()
			o.marcar(criados)
		})
	}()
}

// marcar circles in red pen what the request created that still exists.
func (o *orientador) marcar(ids map[int]bool) {
	g := map[int]bool{}
	for id := range ids {
		if o.b.s.D.Forma(id) != nil {
			g[id] = true
		}
	}
	if len(g) == 0 {
		return
	}
	o.b.sel, o.b.grupo = 0, g
	if len(g) == 1 {
		for id := range g {
			o.b.sel, o.b.grupo = id, nil
		}
	}
	o.b.Refresh()
	if o.b.onSel != nil {
		o.b.onSel(false)
	}
}
