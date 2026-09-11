package siteapi

import (
	"sync"
	"time"
)

// janela is "at most max calls per dur", per account.
type janela struct {
	max int
	dur time.Duration
}

// maxChaves bounds the map. Every key is one account and one action, so this is
// thousands of players acting inside the same hour; past it, marks older than
// the longest window are dropped rather than letting memory grow.
const maxChaves = 20_000

// limites counts calls per key in memory. The panel runs as one instance, the
// same reason its login throttle is in-process (panel/ratelimit.go); a restart
// forgets the counts, and the site keeps its own on the other side.
//
// Sliding windows rather than the login's token bucket: the site has to tell a
// player how long to wait, and "your next try is in 42 minutes" is a question a
// list of timestamps answers exactly.
type limites struct {
	mu     sync.Mutex
	marcas map[string][]time.Time
}

func novosLimites() *limites {
	return &limites{marcas: make(map[string][]time.Time)}
}

// vivasLocked drops the marks that left the window and returns the rest.
func (l *limites) vivasLocked(chave string, j janela, agora time.Time) []time.Time {
	lista := l.marcas[chave]
	i := 0
	for i < len(lista) && agora.Sub(lista[i]) >= j.dur {
		i++
	}
	lista = lista[i:]
	if len(lista) == 0 {
		delete(l.marcas, chave)
	} else {
		l.marcas[chave] = lista
	}
	return lista
}

// espera reports how long until the window has room again; zero when it has.
func (l *limites) espera(chave string, j janela, agora time.Time) time.Duration {
	l.mu.Lock()
	defer l.mu.Unlock()
	return l.esperaLocked(chave, j, agora)
}

func (l *limites) esperaLocked(chave string, j janela, agora time.Time) time.Duration {
	lista := l.vivasLocked(chave, j, agora)
	if len(lista) < j.max {
		return 0
	}
	falta := lista[0].Add(j.dur).Sub(agora)
	if falta < time.Second {
		falta = time.Second
	}
	return falta
}

// marca records one call.
func (l *limites) marca(chave string, j janela, agora time.Time) {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.marcaLocked(chave, j, agora)
}

func (l *limites) marcaLocked(chave string, j janela, agora time.Time) {
	if len(l.marcas) >= maxChaves {
		l.limpaLocked(agora)
	}
	l.marcas[chave] = append(l.vivasLocked(chave, j, agora), agora)
}

// tenta checks and records in one step, so two calls arriving together cannot
// both see room that only one of them has.
func (l *limites) tenta(chave string, j janela, agora time.Time) (time.Duration, bool) {
	l.mu.Lock()
	defer l.mu.Unlock()
	if falta := l.esperaLocked(chave, j, agora); falta > 0 {
		return falta, false
	}
	l.marcaLocked(chave, j, agora)
	return 0, true
}

// limpaLocked drops every key whose newest mark is older than the longest
// window in use, which makes it indistinguishable from a key never seen.
func (l *limites) limpaLocked(agora time.Time) {
	for k, lista := range l.marcas {
		if len(lista) == 0 || agora.Sub(lista[len(lista)-1]) >= janelaMaisLonga {
			delete(l.marcas, k)
		}
	}
}

// janelaMaisLonga is the longest window any endpoint uses.
const janelaMaisLonga = time.Hour
