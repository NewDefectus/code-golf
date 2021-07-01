package routes

import (
	"database/sql"
	"encoding/json"
	"net/http"
	"os"
	"time"

	Config "github.com/code-golf/code-golf/config"
	"github.com/code-golf/code-golf/session"
	"github.com/code-golf/code-golf/zone"
	"golang.org/x/oauth2"
	"golang.org/x/oauth2/github"
)

var config = oauth2.Config{
	ClientID:     "7f6709819023e9215205",
	ClientSecret: os.Getenv("CLIENT_SECRET"),
	Endpoint:     github.Endpoint,
}

// /connect/github/dev exists because GitHub doesn't support multiple URLs.

// ConnectGitHubDev serves GET /connect/github/dev
func ConnectGitHubDev(w http.ResponseWriter, r *http.Request) {
	http.Redirect(w, r, "https://localhost/connect/github?"+r.URL.RawQuery, http.StatusSeeOther)
}

// ConnectGitHub serves GET /connect/github
func ConnectGitHub(w http.ResponseWriter, r *http.Request) {
	var user struct {
		ID    int
		Login string
	}

	cookie := http.Cookie{
		HttpOnly: true,
		Name:     "__Host-session",
		Path:     "/",
		SameSite: http.SameSiteLaxMode,
		Secure:   true,
	}

	var country, timeZone sql.NullString

	if tz, _ := time.LoadLocation(r.FormValue("time_zone")); tz != nil {
		country.String, country.Valid = zone.Country[tz.String()]

		timeZone = sql.NullString{
			String: tz.String(),
			Valid:  tz != time.Local && tz != time.UTC,
		}
	}

	// In dev mode, the username is selected by the "username" parameter
	if Config.Dev && config.ClientSecret == "" {
		user.Login = r.FormValue("username")
		if user.Login == "" {
			user.Login = "JRaspass"
		}

		if err := session.Database(r).QueryRow(
			`SELECT COALESCE((SELECT id FROM oauths WHERE type = 'github' AND username = $1), COUNT(*) + 1) FROM oauths`,
			user.Login,
		).Scan(&user.ID); err != nil {
			panic(err)
		}
	} else {
		if r.FormValue("code") == "" {
			w.WriteHeader(http.StatusBadRequest)
			return
		}

		token, err := config.Exchange(r.Context(), r.FormValue("code"))
		if err != nil {
			panic(err)
		}

		req, err := http.NewRequestWithContext(
			r.Context(), "GET", "https://api.github.com/user", nil)
		if err != nil {
			panic(err)
		}

		req.Header.Add("Authorization", "Bearer "+token.AccessToken)

		res, err := http.DefaultClient.Do(req)
		if err != nil {
			panic(err)
		}
		defer res.Body.Close()

		if err := json.NewDecoder(res.Body).Decode(&user); err != nil {
			panic(err)
		}
	}

	// Only replace NULL countries and time_zones, never user chosen ones.
	if err := session.Database(r).QueryRow(
		`SELECT login('github', $1, $2, $3, $4)`,
		user.ID, user.Login, country, timeZone,
	).Scan(&cookie.Value); err != nil {
		panic(err)
	}

	http.SetCookie(w, &cookie)

	uri := r.FormValue("redirect_uri")
	if uri == "" {
		uri = "/"
	}

	http.Redirect(w, r, uri, http.StatusSeeOther)
}
