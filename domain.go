package app

import "context"

// Tiddler represents a tiddlywiki tiddler.
type Tiddler struct {
	Rev      int
	Meta     string
	Text     string
	IsSystem bool
}

// TiddlyStore represents the actions that can be taken about tiddlers.
type TiddlyStore interface {
	Delete(ctx context.Context, title string) error
	Get(ctx context.Context, title string) (Tiddler, error)
	GetList(ctx context.Context) ([]Tiddler, error)
	Upsert(ctx context.Context, title string, t Tiddler) error
}

// UserStore represents the actions that can be taken about users.
type UserStore interface {
	Close()
	Authenticate(email, password string) (*User, error)
	Create(user *User) error
	ByEmail(email string) (*User, error)
	ByRememberToken(token string) (*User, error)
	CreateRememberToken(user *User) (string, error)
	ClearRememberToken(token string) error
}

// User represents a user in our system.
type User struct {
	Email        string
	PasswordHash string
}

// UserToken is a remember token created by a user.
type UserToken struct {
	Email         string
	RememberToken string
}
