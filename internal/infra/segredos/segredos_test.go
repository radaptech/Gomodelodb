package segredos

import (
	"errors"
	"testing"

	"github.com/zalando/go-keyring"
)

func TestCofre(t *testing.T) {
	keyring.MockInit() // in-memory keyring; never touches the real one
	c := Novo()
	if c.Ler("Claude") != "" {
		t.Fatal("empty at start")
	}
	if !c.Guardar("Claude", "sk-ant-1") {
		t.Fatal("should persist to the keyring")
	}
	if k, _ := keyring.Get(servico, "Claude"); k != "sk-ant-1" {
		t.Fatal("not in keyring")
	}
	if Novo().Ler("Claude") != "sk-ant-1" {
		t.Fatal("a new run must read it back")
	}
	c.Esquecer("Claude")
	if Novo().Ler("Claude") != "" {
		t.Fatal("forgotten key came back")
	}

	// no keyring available: keys stay in memory, the cofre says so
	keyring.MockInitWithError(errors.New("no secret service"))
	c = Novo()
	if c.Guardar("GPT", "sk-1") || c.Persistente() {
		t.Fatal("must report memory-only storage")
	}
	if c.Ler("GPT") != "sk-1" {
		t.Fatal("memory fallback")
	}
}
