// Package segredos keeps API keys in the system keyring (Secret Service /
// GNOME Keyring on Linux, Keychain on macOS, Credential Manager on Windows).
// When no keyring is available, keys live only in memory for this run.
package segredos

import (
	"errors"
	"sync"

	"github.com/zalando/go-keyring"
)

const servico = "gobrmodelo"

// Cofre stores one key per provider name.
type Cofre struct {
	mu       sync.Mutex
	memoria  map[string]string
	semCofre bool // the keyring failed once: stop trying, keep keys in memory
}

func Novo() *Cofre { return &Cofre{memoria: map[string]string{}} }

// Ler returns the saved key for provedor, or "".
func (c *Cofre) Ler(provedor string) string {
	c.mu.Lock()
	defer c.mu.Unlock()
	if k, ok := c.memoria[provedor]; ok {
		return k
	}
	if c.semCofre {
		return ""
	}
	k, err := keyring.Get(servico, provedor)
	if err != nil && !errors.Is(err, keyring.ErrNotFound) {
		c.semCofre = true
	}
	return k
}

// Guardar saves the key. persistente reports whether it reached the keyring
// (false: it lasts only until the app closes).
func (c *Cofre) Guardar(provedor, chave string) (persistente bool) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.memoria[provedor] = chave
	if c.semCofre {
		return false
	}
	if keyring.Set(servico, provedor, chave) != nil {
		c.semCofre = true
		return false
	}
	return true
}

// Esquecer removes the key everywhere.
func (c *Cofre) Esquecer(provedor string) {
	c.mu.Lock()
	defer c.mu.Unlock()
	delete(c.memoria, provedor)
	if !c.semCofre {
		keyring.Delete(servico, provedor)
	}
}

// Persistente tells whether keys survive a restart.
func (c *Cofre) Persistente() bool {
	c.mu.Lock()
	defer c.mu.Unlock()
	return !c.semCofre
}
