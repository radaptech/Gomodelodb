package aplicacao

import dom "github.com/brmodelo/gobrmodelo/internal/dominio"

// Resumo is the diagram described by meaning rather than by geometry: what an
// assistant needs to reason about the model and to refer to its parts by ID.
type Resumo struct {
	Entidades       []EntidadeResumo       `json:"entidades"`
	Relacionamentos []RelacionamentoResumo `json:"relacionamentos"`
	AtributosSoltos []AtributoResumo       `json:"atributos_soltos,omitempty"`
}

type AtributoResumo struct {
	ID            int    `json:"id"`
	Nome          string `json:"nome"`
	Tipo          string `json:"tipo,omitempty"`
	Identificador bool   `json:"identificador,omitempty"`
}

type EntidadeResumo struct {
	ID        int              `json:"id"`
	Nome      string           `json:"nome"`
	Atributos []AtributoResumo `json:"atributos"`
}

type Participacao struct {
	Entidade      int    `json:"entidade_id"`
	Nome          string `json:"entidade"`
	Cardinalidade string `json:"cardinalidade"`
}

type RelacionamentoResumo struct {
	ID            int              `json:"id"`
	Nome          string           `json:"nome"`
	Participantes []Participacao   `json:"participantes"`
	Atributos     []AtributoResumo `json:"atributos,omitempty"`
}

func (s *Sessao) Resumo() Resumo {
	d := s.D
	attrs := map[int][]AtributoResumo{} // owner ID -> attributes
	donos := map[int]bool{}             // attribute IDs that have an owner
	parts := map[int][]Participacao{}   // relationship ID -> entities
	for _, l := range d.Ligacoes {
		a, b := d.Forma(l.A), d.Forma(l.B)
		switch {
		case a.Tipo == dom.Atributo:
			attrs[b.ID] = append(attrs[b.ID], AtributoResumo{a.ID, a.Texto, a.TipoAtributo, a.Identificador})
			donos[a.ID] = true
		case l.Card >= 0:
			e, r := a, b
			if e.Tipo != dom.Entidade {
				e, r = r, e
			}
			parts[r.ID] = append(parts[r.ID], Participacao{e.ID, e.Texto, dom.Cards[l.Card]})
		}
	}
	res := Resumo{Entidades: []EntidadeResumo{}, Relacionamentos: []RelacionamentoResumo{}}
	for _, f := range d.Formas {
		switch f.Tipo {
		case dom.Entidade:
			res.Entidades = append(res.Entidades, EntidadeResumo{f.ID, f.Texto, append([]AtributoResumo{}, attrs[f.ID]...)})
		case dom.Relacionamento:
			res.Relacionamentos = append(res.Relacionamentos, RelacionamentoResumo{f.ID, f.Texto, parts[f.ID], attrs[f.ID]})
		case dom.Atributo:
			if !donos[f.ID] {
				res.AtributosSoltos = append(res.AtributosSoltos, AtributoResumo{f.ID, f.Texto, f.TipoAtributo, f.Identificador})
			}
		}
	}
	return res
}
