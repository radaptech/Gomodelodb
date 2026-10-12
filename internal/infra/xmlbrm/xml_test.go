package xmlbrm

import (
	"bytes"
	"strings"
	"testing"

	dom "github.com/brmodelo/gobrmodelo/internal/dominio"
)

// Trimmed from Template.brMt (written by the Java app).
const javaXML = `<?xml version="1.0" encoding="UTF-8" standalone="no"?><DIAGRAMA FIRST_SEL="2" ID="1" TIPO="tpConceitual" UniversalUnicID="E5EB"><Atributo ID="4"><Bounds Height="14" Left="223" Top="42" Width="61"/><Texto>Código</Texto><Identificador Valor="true"/><TipoAtributo Valor="VARCHAR(11)"/></Atributo><Entidade ID="10"><Bounds Height="58" Left="121" Top="94" Width="120"/><Texto>Pessoa</Texto></Entidade><Relacionamento ID="11"><Bounds Height="60" Left="300" Top="94" Width="100"/><Texto>mora</Texto></Relacionamento><Uniao ID="12"/><Ligacao ID="5"><Ligacoes PontaA="4" PontaB="10"/></Ligacao><Ligacao ID="13"><Ligacoes PontaA="10" PontaB="11"/><Cardinalidade ID="20"><Card Valor="2"/></Cardinalidade></Ligacao><Ligacao ID="14"><Ligacoes PontaA="12" PontaB="10"/></Ligacao></DIAGRAMA>`

func TestLoadSaveRoundTrip(t *testing.T) {
	d, err := LerConceitual(strings.NewReader(javaXML))
	if err != nil {
		t.Fatal(err)
	}
	if len(d.Formas) != 3 || len(d.Ligacoes) != 2 { // Uniao and its link skipped
		t.Fatalf("got %d formas %d ligacoes", len(d.Formas), len(d.Ligacoes))
	}
	if a := d.Forma(4); a.Texto != "Código" || !a.Identificador || a.X != 223 || a.W != 61 {
		t.Fatalf("atributo: %+v", a)
	}
	if d.Ligacoes[1].Card != 2 {
		t.Fatalf("card: %d", d.Ligacoes[1].Card)
	}
	if f := d.Add(dom.Entidade, 0, 0, "X"); f.ID != 21 {
		t.Fatalf("new id %d must not collide (cardinalidade had 20)", f.ID)
	}

	var buf bytes.Buffer
	if err := SalvarConceitual(&buf, d); err != nil {
		t.Fatal(err)
	}
	d2, err := LerConceitual(&buf)
	if err != nil {
		t.Fatal(err)
	}
	if len(d2.Formas) != 4 || len(d2.Ligacoes) != 2 || d2.Ligacoes[1].Card != 2 || d2.Forma(4).Texto != "Código" || d2.Forma(4).TipoAtributo != "VARCHAR(11)" {
		t.Fatalf("round trip lost data: %s", buf.String())
	}
}

// Real logical XML written by the Java app (Template.brMt), trimmed.
const javaLogico = `<?xml version="1.0" encoding="UTF-8" standalone="no"?><DIAGRAMA ID="1" TIPO="tpLogico"><Tabela ID="3"><Bounds Height="106" Left="371" Top="173" Width="150"/><Texto>Dependente</Texto><Campos><Campo><Texto>ID_RESPONSAVEL</Texto><Tipo>INT</Tipo><Key Valor="false"/><Fkey Valor="true"/></Campo></Campos><Constraints><Constraint><Tipo Valor="2"/><DdlOnDelete/><LigacaoParaOrigem ID="4"/><ConstraintOrigem CamposDestino="[0]" CamposOrigem="[0]" ID="2,0">IR</ConstraintOrigem></Constraint></Constraints></Tabela><LogicoLinha ID="4"><Ligacoes PontaA="2" PontaB="3"/><LogicoCardinalidade ID="5"><Card Valor="1"/></LogicoCardinalidade><LogicoCardinalidade ID="6"><Card Valor="3"/></LogicoCardinalidade></LogicoLinha><Tabela ID="2"><Bounds Height="106" Left="55" Top="44" Width="150"/><Texto>Funcionário</Texto><Campos><Campo><Texto>ID</Texto><Tipo>INT</Tipo><Key Valor="true"/><Fkey Valor="false"/></Campo></Campos><Constraints><Constraint><Tipo Valor="0"/><LigacaoParaOrigem ID="-1"/><ConstraintOrigem CamposDestino="[-1]" CamposOrigem="[0]" ID="-1,-1"/></Constraint></Constraints></Tabela></DIAGRAMA>`

func TestLoadJavaLogico(t *testing.T) {
	l, err := LerLogico(bytes.NewBufferString(javaLogico))
	if err != nil {
		t.Fatal(err)
	}
	dep := l.Tabela(3)
	if dep == nil || len(dep.FKs) != 1 || dep.FKs[0].Ref.Nome != "Funcionário" || dep.FKs[0].Linha.ID != 4 || dep.FKs[0].Pares[0] != [2]int{0, 0} {
		t.Fatalf("%+v", dep)
	}
	if ln := l.Linhas[0]; ln.CardA != 1 || ln.CardB != 3 || ln.A.Nome != "Funcionário" {
		t.Fatalf("%+v", ln)
	}
	if f := l.AddTabela("x", 0, 0); f.ID != 7 {
		t.Fatalf("id %d collides", f.ID)
	}
}
