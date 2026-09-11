package panel

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

// The player site's API (package siteapi) lives on a listener of its own. None
// of its routes may answer on the panel's public one — not even behind the
// staff login, which would turn "private network only" into "anyone with a
// moderator password".
func TestSiteAPIRoutesDoNotExistOnThePublicListener(t *testing.T) {
	h := newTestPanel(t, newFakeAccounts(roleAdmin))
	for _, r := range [][2]string{
		{http.MethodGet, "/site/v1/jogo"},
		{http.MethodGet, "/site/v1/contas/1/estado"},
		{http.MethodGet, "/site/v1/contas/1/historico"},
		{http.MethodGet, "/site/v1/contas/1/entregas"},
		{http.MethodPost, "/site/v1/contas/1/entregar-agora"},
		{http.MethodPost, "/site/v1/contas/1/desatolar"},
		{http.MethodPost, "/site/v1/contas/1/senha"},
	} {
		req := httptest.NewRequest(r[0], r[1], nil)
		req.Header.Set("Authorization", "Bearer qualquer")
		rec := httptest.NewRecorder()
		h.ServeHTTP(rec, req)
		// 404, not a redirect to the login: a redirect would mean some staff
		// route matched the path and only the session stood in the way.
		if rec.Code != http.StatusNotFound {
			t.Errorf("%s %s on the public listener: status = %d, want 404", r[0], r[1], rec.Code)
		}
	}
}
