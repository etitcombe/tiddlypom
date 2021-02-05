package main

import (
	"encoding/gob"
	"flag"
	"fmt"
	"log"
	"os"

	app "github.com/etitcombe/tiddlypom"
	"github.com/etitcombe/tiddlypom/rand"
	"golang.org/x/crypto/bcrypt"
)

/*
How to generate a users.gob file for the web application. Currently only one
user is supported.

1. > ./admin -cmd=pepper
CPjaot8hYLXpm4xIaXHWsQKJWkelY3msP6AbR8wYmrE=
2. > ./admin -cmd=password -pepper=CPjaot8hYLXpm4xIaXHWsQKJWkelY3msP6AbR8wYmrE= -password=fancy-password
$2a$10$r1sE9VECMqhjaikC2z5/iOaSwCDGlVOe4PLwDjJzKLT7iY1QDkF3.
3. > ./admin -cmd=userfile -email=user@site.com -hashedPassword=$2a$10$r1sE9VECMqhjaikC2z5/iOaSwCDGlVOe4PLwDjJzKLT7iY1QDkF3.
[this produces users.gob which can now be copied to the folder where the web application runs from]
*/

func main() {
	var (
		cmd            string
		pepper         string
		password       string
		email          string
		hashedPassword string
	)
	flag.StringVar(&cmd, "cmd", "", "The command to execute: pepper, password, userfile. [Required]")
	flag.StringVar(&pepper, "pepper", "", "The pepper to use when hashing a password. [Required when cmd=password]")
	flag.StringVar(&password, "password", "", "The password to hash. [Required when cmd=password]")
	flag.StringVar(&email, "email", "", "The email to user for the user. [Required when cmd=userfile]")
	flag.StringVar(&hashedPassword, "hashedPassword", "", "The hashed password to use for the user. [Required when cmd=userfile]")
	flag.Parse()

	switch cmd {
	case "pepper":
		generatePepper()
	case "password":
		if pepper == "" || password == "" {
			flag.Usage()
			return
		}
		hashPassword(pepper, password)
	case "userfile":
		if email == "" || hashedPassword == "" {
			flag.Usage()
			return
		}
		saveUsers(email, hashedPassword)
	default:
		flag.Usage()
	}
}

func generatePepper() {
	t, err := rand.RememberToken()
	if err != nil {
		log.Fatal(err)
	}
	fmt.Println(t)
}

func hashPassword(pepper, password string) {
	hashedBytes, err := bcrypt.GenerateFromPassword([]byte(password+pepper), bcrypt.DefaultCost)
	if err != nil {
		log.Fatal(err)
	}
	fmt.Println(string(hashedBytes))
}

func saveUsers(email, hashedPassword string) {
	f, err := os.OpenFile("users.gob", os.O_CREATE|os.O_RDWR, 0644)
	if err != nil {
		log.Fatal(err)
	}
	defer f.Close()

	users := []app.User{
		{Email: email, PasswordHash: hashedPassword},
	}
	enc := gob.NewEncoder(f)
	err = enc.Encode(users)
	if err != nil {
		log.Fatal(err)
	}
}
