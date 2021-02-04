package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"net/url"
	"strings"
	"time"

	app "github.com/etitcombe/tiddlypom"
)

func (s *server) handleDelete() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		// https://tiddlywiki.com/static/WebServer%2520API%253A%2520Delete%2520Tiddler.html

		if r.Method != http.MethodDelete {
			s.clientError(w, http.StatusMethodNotAllowed, http.StatusText(http.StatusMethodNotAllowed))
			return
		}
		title := strings.TrimPrefix(r.URL.Path, "/bags/default/tiddlers/")
		if err := s.tiddlyStore.Delete(r.Context(), title); err != nil {
			s.serverError(w, r, err)
			return
		}
		w.WriteHeader(http.StatusNoContent)
	}
}

func (s *server) handleHome() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		// https://tiddlywiki.com/static/WebServer%2520API%253A%2520Get%2520Wiki.html

		if r.Method != http.MethodGet {
			s.clientError(w, http.StatusMethodNotAllowed, http.StatusText(http.StatusMethodNotAllowed))
			return
		}
		if r.URL.Path != "/" {
			s.clientError(w, http.StatusNotFound, http.StatusText(http.StatusNotFound))
			return
		}
		http.ServeFile(w, r, "index.html")
	}
}

func (s *server) handleList() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		// https://tiddlywiki.com/static/WebServer%2520API%253A%2520Get%2520All%2520Tiddlers.html

		tiddlers, err := s.tiddlyStore.GetList(r.Context())
		if err != nil {
			s.serverError(w, r, err)
			return
		}

		sep := ""
		var buf bytes.Buffer
		buf.WriteString("[")
		for _, t := range tiddlers {
			buf.WriteString(sep)
			sep = ","
			buf.WriteString(t.Meta)
		}
		buf.WriteString("]")

		// s.infoLog.Println(buf.String())

		w.Header().Set("Content-Type", "application/json")
		w.Write(buf.Bytes())
	}
}

func (s *server) handleLogin() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodPost {
			err := r.ParseForm()
			if err != nil {
				s.clientError(w, http.StatusBadRequest, err.Error())
				return
			}
			email := r.PostFormValue("email")
			if len(email) == 0 {
				s.clientError(w, http.StatusBadRequest, "Email address is required.")
				return
			}
			password := r.PostFormValue("password")
			if len(password) == 0 {
				s.clientError(w, http.StatusBadRequest, "Password is required.")
				return
			}

			u, err := s.userStore.Authenticate(email, password)
			if err != nil {
				s.clientError(w, http.StatusUnauthorized, "")
				return
			}

			token, err := s.userStore.CreateRememberToken(u)
			if err != nil {
				s.serverError(w, r, err)
				return
			}

			c := http.Cookie{
				HttpOnly: true,
				Name:     rememberCookieName,
				Value:    token,
				Path:     "/",
				Expires:  time.Now().AddDate(1, 0, 0),
				MaxAge:   365 * 24 * 60 * 60,
			}
			http.SetCookie(w, &c)
			http.Redirect(w, r, "/", http.StatusFound)
			return
		}

		// GET
		s.render(w, r, "login", "Login", nil, nil)
	}
}

func (s *server) handleStatus() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		// https://tiddlywiki.com/static/WebServer%2520API%253A%2520Get%2520Server%2520Status.html

		if r.Method != "GET" {
			s.clientError(w, http.StatusMethodNotAllowed, http.StatusText(http.StatusMethodNotAllowed))
			return
		}

		// TODO: get name from currently logged-in user.

		w.Header().Set("Content-Type", "application/json")
		body := `{
			"username": "Eric",
			"anonymous": false,
			"read_only": false,
			"space": {
			  "recipe": "default"
			},
			"tiddlywiki_version": "5.1.23"
		  }`
		w.Write([]byte(body))
	}
}

func (s *server) handleTiddler() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		title := strings.TrimPrefix(r.URL.Path, "/recipes/default/tiddlers/")

		if r.Method == http.MethodPut {
			// https://tiddlywiki.com/static/WebServer%2520API%253A%2520Put%2520Tiddler.html

			data, err := ioutil.ReadAll(r.Body)
			if err != nil {
				s.clientError(w, http.StatusBadRequest, "cannot read data: "+err.Error())
				return
			}

			var js map[string]interface{}
			err = json.Unmarshal(data, &js)
			if err != nil {
				s.serverError(w, r, err)
				return
			}

			js["bag"] = "bag"

			var t app.Tiddler
			if text, ok := js["text"].(string); ok {
				t.Text = text
			}
			delete(js, "text")

			rev := 1
			old, err := s.tiddlyStore.Get(r.Context(), title)
			if err == nil {
				rev = old.Rev + 1
			}
			t.Rev = rev

			js["revision"] = rev

			meta, err := json.Marshal(js)
			if err != nil {
				s.serverError(w, r, err)
				return
			}
			t.Meta = string(meta)
			t.IsSystem = isSystemTiddler(js["title"].(string))

			if err := s.tiddlyStore.Upsert(r.Context(), title, t); err != nil {
				s.serverError(w, r, err)
				return
			}

			w.Header().Set("Content-Type", "text/plain")
			// etag := fmt.Sprintf("\"default/%s/%d:%x\"", url.QueryEscape(title), rev, md5.Sum(data))
			etag := fmt.Sprintf("\"default/%s/%d:\"", url.QueryEscape(title), rev)
			w.Header().Set("Etag", etag)
			return
		}

		// GET
		// https://tiddlywiki.com/static/WebServer%2520API%253A%2520Get%2520Tiddler.html
		t, err := s.tiddlyStore.Get(r.Context(), title)
		if err != nil {
			s.serverError(w, r, err)
			return
		}

		var js map[string]interface{}
		err = json.Unmarshal([]byte(t.Meta), &js)
		if err != nil {
			s.serverError(w, r, err)
			return
		}
		js["text"] = string(t.Text)
		data, err := json.Marshal(js)
		if err != nil {
			s.serverError(w, r, err)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		w.Write(data)
	}
}

func isSystemTiddler(title string) bool {
	/*
		These tiddlers are listed when you click on System under More in the sidebar:
		   $:/boot/boot.css
		   $:/boot/boot.js
		   $:/boot/bootprefix.js
		   $:/core
		   $:/HistoryList
		   $:/isEncrypted
		   $:/library/sjcl.js
		   $:/plugins/tiddlywiki/tiddlyweb
		   $:/state/tab-1749438307
		   $:/state/tab/moresidebar-1850697562
		   $:/state/tab/sidebar--595412856
		   $:/status/IsAnonymous
		   $:/status/IsLoggedIn
		   $:/status/IsReadOnly
		   $:/status/RequireReloadDueToPluginChange
		   $:/status/UserName
		   $:/StoryList
		   $:/temp/info-plugin
		   $:/themes/tiddlywiki/snowwhite
		   $:/themes/tiddlywiki/vanilla
		But if we don't return what's under /themes/ any changes we make to the
		appearance will get lost on the next refresh.
	*/
	return strings.HasPrefix(title, "$:/boot/") ||
		strings.HasPrefix(title, "$:/core") ||
		strings.HasPrefix(title, "$:/HistoryList") ||
		strings.HasPrefix(title, "$:/isEncrypted") ||
		strings.HasPrefix(title, "$:/library/") ||
		strings.HasPrefix(title, "$:/plugins/") ||
		strings.HasPrefix(title, "$:/state/") ||
		strings.HasPrefix(title, "$:/status/") ||
		strings.HasPrefix(title, "$:/StoryList") ||
		strings.HasPrefix(title, "$:/temp/")
}

/*
The more it snows (tiddlypom)
The more it goes  (tiddlypom)
The more it goes  (tiddlypom)
On snowing

And nobody knows  (tiddlypom)
How cold my toes (tiddlypom)
How cold my toes (tiddlypom)
Are growing
*/
