package main

import (
	"context"
	"flag"
	"fmt"
	"io/ioutil"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/etitcombe/tiddlypom/config"
	"github.com/etitcombe/tiddlypom/db"
)

func main() {
	var port int
	var debug bool
	flag.IntVar(&port, "port", 9090, "the port to start the web server on")
	flag.BoolVar(&debug, "debug", false, "set to true when you need more logging info")
	flag.Parse()

	infoWriter := ioutil.Discard
	if debug {
		infoWriter = os.Stdout
	}
	infoLog := log.New(infoWriter, "INFO  ", log.Ldate|log.Ltime|log.Lmsgprefix)
	errorLog := log.New(os.Stderr, "ERROR ", log.Ldate|log.Ltime|log.Lshortfile|log.Lmsgprefix)

	config, err := config.LoadConfig()
	if err != nil {
		errorLog.Fatal(err)
	}

	tiddlyStore, err := db.NewTiddlyStore("./database/tiddly.db")
	if err != nil {
		errorLog.Fatal(err)
	}
	defer tiddlyStore.Close()

	if err := tiddlyStore.Open(); err != nil {
		errorLog.Fatal(err)
	}

	userStore, err := db.NewUserStoreFile(config.Pepper)
	if err != nil {
		errorLog.Fatal(err)
	}

	server := newServer(infoLog, errorLog, tiddlyStore, userStore)

	srv := &http.Server{
		Addr:         fmt.Sprintf(":%d", port),
		ErrorLog:     errorLog,
		Handler:      server,
		WriteTimeout: time.Second * 15,
		ReadTimeout:  time.Second * 15,
		IdleTimeout:  time.Second * 60,
	}

	idleConnsClosed := make(chan struct{})
	go func() {
		sigint := make(chan os.Signal, 1)
		signal.Notify(sigint, os.Interrupt, os.Kill, syscall.SIGINT, syscall.SIGTERM)
		s := <-sigint

		ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
		defer cancel()

		infoLog.Println("shutting down:", s)
		if err := srv.Shutdown(ctx); err != nil {
			// Error from closing listeners, or context timeout:
			errorLog.Printf("HTTP server Shutdown: %v", err)
		}
		close(idleConnsClosed)
	}()

	infoLog.Printf("tiddly listening on %d\n", port)
	if err := srv.ListenAndServe(); err != http.ErrServerClosed {
		// Error starting or closing listener:
		errorLog.Fatalf("HTTP server ListenAndServe: %v", err)
	}

	<-idleConnsClosed
}
