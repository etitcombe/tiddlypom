package main

import (
	"context"
	"fmt"
	"log"
	"net/http"

	"github.com/etitcombe/logifymw"
)

func (s *server) registerRoutes() {
	mux := http.NewServeMux()
	addIcons(mux)
	mux.Handle("/", s.authenticate(s.requireAuthentication(s.handleHome())))
	mux.Handle("/bags/default/tiddlers/", s.authenticate(s.requireAuthentication(s.handleDelete())))
	mux.Handle("/bags/bag/tiddlers/", s.authenticate(s.requireAuthentication(s.handleDelete())))
	mux.Handle("/login/", s.handleLogin())
	mux.Handle("/logout/", s.handleLogout())
	mux.Handle("/recipes/default/tiddlers/", s.authenticate(s.requireAuthentication(s.handleTiddler())))
	mux.Handle("/recipes/default/tiddlers.json", s.authenticate(s.requireAuthentication(s.handleList())))
	mux.Handle("/status", s.authenticate(s.requireAuthentication(s.handleStatus())))

	s.router = s.recoverPanicMw(logifymw.LogIt2(s.infoLog, headersMw(mux)))
}

func addIcons(mux *http.ServeMux) {
	icons := []string{
		"/apple-touch-icon-120x120-precomposed.png",
		"/apple-touch-icon-120x120.png",
		"/apple-touch-icon-precomposed.png",
		"/apple-touch-icon.png",
		"/favicon.ico",
	}
	for _, icon := range icons {
		addFileHandler(mux, icon)
	}
}

func addFileHandler(mux *http.ServeMux, file string) {
	mux.HandleFunc(file, func(w http.ResponseWriter, r *http.Request) {
		http.ServeFile(w, r, "."+file)
	})
}

func (s *server) authenticate(h http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		c, err := r.Cookie(rememberCookieName)
		if err != nil {
			// When the cookie doesn't exist the err will be "http: named cookie not present"
			h.ServeHTTP(w, r)
			return
		}

		_, err = s.userStore.ByRememberToken(c.Value)
		if err != nil {
			log.Println("remember token not found", c.Value, err)
			h.ServeHTTP(w, r)
			return
		}

		ctx := r.Context()
		ctx = context.WithValue(ctx, adminCheckKey, true)
		r = r.WithContext(ctx)
		h.ServeHTTP(w, r)
	})
}

func headersMw(next http.Handler) http.Handler {
	// adding these headers causes the site to render incorrectly. which one causes the problem?
	var headers = map[string]string{
		// "Content-Security-Policy":   "default-src 'self'; script-src 'self'; img-src 'self';",
		// "Feature-Policy":            "camera 'none';fullscreen 'self';geolocation 'none';gyroscope 'none';magnetometer 'none';microphone 'none';midi 'none';payment 'none';sync-xhr 'none';",
		// "Referrer-Policy":           "no-referrer-when-downgrade",
		// "Strict-Transport-Security": "max-age=31536000; includeSubDomains",
		// "X-Content-Type-Options":    "nosniff",
		// "X-Frame-Options":           "SAMEORIGIN",
		// "X-XSS-Protection":          "1; mode=block",
	}
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		for k, v := range headers {
			w.Header().Set(k, v)
		}

		next.ServeHTTP(w, r)
	})
}

func (s *server) recoverPanicMw(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		defer func() {
			if err := recover(); err != nil {
				w.Header().Set("Connection", "close")
				s.serverError(w, r, fmt.Errorf("%s", err))
			}
		}()

		next.ServeHTTP(w, r)
	})
}

func (s *server) requireAuthentication(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !s.isAdmin(r) {
			http.Redirect(w, r, "/login/", http.StatusFound)
			// s.clientError(w, http.StatusUnauthorized, "Nope. You're not authorized.")
			return
		}
		next.ServeHTTP(w, r)
	})
}
