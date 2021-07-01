package routes

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/SeerUK/assert"
)

func TestConnectGitHubNoCode(t *testing.T) {
	w := httptest.NewRecorder()

	ConnectGitHub(w, httptest.NewRequest("", "/connect/github", nil))

	assert.Equal(t, w.Code, http.StatusBadRequest)
}
