package main

import (
	"bytes"
	"crypto/md5"
	"fmt"
	"html/template"
	"log"
	"net/http"
	"runtime/debug"
	"sync"
	"time"

	app "github.com/etitcombe/tiddlypom"
)

type contextKey string

const (
	rememberCookieName string = "tiddlywiki-remember"

	adminCheckKey contextKey = "admin-check"

	etagCacheKey string = "etag"
)

type server struct {
	infoLog  *log.Logger
	errorLog *log.Logger

	router http.Handler

	tiddlyStore app.TiddlyStore
	userStore   app.UserStore

	templateCache map[string]*template.Template

	rwMutex sync.RWMutex
	cache   map[string]interface{}
}

type viewModel struct {
	Blurb template.HTML
	Title string
	Yield interface{}
}

func newServer(infoLog, errorLog *log.Logger, ls app.TiddlyStore, us app.UserStore) *server {
	srv := &server{
		infoLog:  infoLog,
		errorLog: errorLog,
	}
	srv.rwMutex = sync.RWMutex{}
	srv.tiddlyStore = ls
	srv.userStore = us
	srv.parseTemplates()
	srv.registerRoutes()
	srv.cache = make(map[string]interface{})
	srv.updateEtag()
	return srv
}

func (s *server) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	s.router.ServeHTTP(w, r)
}

func (s *server) clientError(w http.ResponseWriter, status int, message string) {
	errorMessage := http.StatusText(status)
	if message != "" {
		errorMessage += ": " + message
	}
	http.Error(w, errorMessage, status)
}

func (s *server) serverError(w http.ResponseWriter, r *http.Request, err error) {
	trace := fmt.Sprintf("%s\n%s", err.Error(), debug.Stack())
	s.errorLog.Output(2, trace)

	errorMessage := http.StatusText(http.StatusInternalServerError)
	if s.isAdmin(r) {
		errorMessage += "\n" + trace
	}
	http.Error(w, errorMessage, http.StatusInternalServerError)
}

func (s *server) isAdmin(r *http.Request) bool {
	if temp := r.Context().Value(adminCheckKey); temp != nil {
		if val, ok := temp.(bool); ok {
			return val
		}
		s.errorLog.Printf("isAdmin context.value is not a bool: %v", temp)
	}
	return false
}

func (s *server) parseTemplates() {
	cache := map[string]*template.Template{}
	cache["login"] = template.Must(template.New("login").ParseFiles("./templates/login.gohtml"))
	s.templateCache = cache
}

func (s *server) render(w http.ResponseWriter, r *http.Request, name, title string, scripts []string, data interface{}) {
	isAdminFunc := func() bool {
		return s.isAdmin(r)
	}

	ts, ok := s.templateCache[name]
	if !ok {
		s.serverError(w, r, fmt.Errorf("template %s does not exist", name))
		return
	}

	ts.Funcs(template.FuncMap{"isAdmin": isAdminFunc})

	viewModel := viewModel{
		Blurb: "<!-- Heaven is your fucking life. -->",
		Title: title,
		Yield: data,
	}

	buf := bytes.Buffer{}

	err := ts.ExecuteTemplate(&buf, "layout", viewModel)
	if err != nil {
		s.serverError(w, r, err)
		return
	}

	buf.WriteTo(w)
}

/*func (s *server) readCache(key string) (interface{}, bool) {
	s.rwMutex.RLock()
	defer s.rwMutex.RUnlock()
	if value, ok := s.cache[key]; ok {
		return value, true
	}
	return nil, false
}

func (s *server) removeCache(key string) {
	s.rwMutex.Lock()
	defer s.rwMutex.Unlock()
	delete(s.cache, key)
}*/

func (s *server) setCache(key string, value interface{}) {
	s.rwMutex.Lock()
	defer s.rwMutex.Unlock()
	s.cache[key] = value
}

func (s *server) updateEtag() {
	v := fmt.Sprintf("%d", time.Now().Unix())
	h := fmt.Sprintf("%x", md5.Sum([]byte(v)))
	s.setCache(etagCacheKey, h)
}
