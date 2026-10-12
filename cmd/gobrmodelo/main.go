// Command gobrmodelo is brModelo in Go: the conceptual/logical modeling window,
// plus an MCP server on localhost so Claude can edit the open diagram live.
//
// The MCP endpoint is http://127.0.0.1:7421/mcp; set BRMODELO_MCP_PORTA to use
// another port, or 0 to turn it off.
package main

import (
	"fmt"
	"os"
	"strconv"
	"strings"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/app"

	"github.com/brmodelo/gobrmodelo/internal/aplicacao"
	"github.com/brmodelo/gobrmodelo/internal/apresentacao/gui"
	"github.com/brmodelo/gobrmodelo/internal/apresentacao/mcp"
	"github.com/brmodelo/gobrmodelo/internal/infra/segredos"
)

func main() {
	w := app.NewWithID("br.com.sis4.gobrmodelo").NewWindow("brModelo — Modelo conceitual")
	s := aplicacao.Nova()

	porta := mcp.PortaPadrao
	if v, ok := os.LookupEnv("BRMODELO_MCP_PORTA"); ok {
		if n, err := strconv.Atoi(v); err == nil {
			porta = n
		}
	}
	status := "MCP desligado"
	if porta > 0 {
		url, parar, err := mcp.Servir(s, porta)
		if err != nil {
			// the window still opens; only the assistant link is missing
			status = fmt.Sprintf("MCP indisponível: porta %d ocupada", porta)
			fmt.Fprintln(os.Stderr, "brmodelo: MCP:", err)
		} else {
			status = "MCP " + strings.TrimPrefix(url, "http://")
			defer parar()
		}
	}

	gui.Montar(w, s, status, segredos.Novo())
	w.Resize(fyne.NewSize(1600, 860))
	w.ShowAndRun()
}
