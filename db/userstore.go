package db

import (
	"encoding/gob"
	"errors"
	"os"
	"strings"
	"sync"

	app "github.com/etitcombe/tiddlypom"
	"github.com/etitcombe/tiddlypom/rand"
	"golang.org/x/crypto/bcrypt"
)

var errNotFound = errors.New("not found")

// UserStoreFile implements the UserStore interface against the file system.
type UserStoreFile struct {
	UserPwPepper string
	lock         sync.Mutex
}

// NewUserStoreFile creates and returns a new instance of a UserStoreFile.
func NewUserStoreFile(pepper string) (*UserStoreFile, error) {
	return &UserStoreFile{UserPwPepper: pepper}, nil
}

// Authenticate authenticates a user based on email and password
func (s *UserStoreFile) Authenticate(email, password string) (*app.User, error) {
	foundUser, err := s.ByEmail(email)
	if err != nil {
		return nil, err
	}

	err = bcrypt.CompareHashAndPassword([]byte(foundUser.PasswordHash), []byte(password+s.UserPwPepper))
	switch err {
	case nil:
		return foundUser, nil
	// case bcrypt.ErrMismatchedHashAndPassword:
	// 	return nil, app.ErrInvalidPassword
	default:
		return nil, err
	}
}

// Close closes the underlying connection.
func (s *UserStoreFile) Close() {
}

// Create creates a new user.
func (s *UserStoreFile) Create(user *app.User) error {
	return nil
}

// ByEmail retrieves a user by their email address.
func (s *UserStoreFile) ByEmail(email string) (*app.User, error) {
	users, err := s.retrieveUsers()
	if err != nil {
		return nil, err
	}
	for _, u := range users {
		if strings.EqualFold(u.Email, email) {
			return &u, nil
		}
	}
	return nil, errNotFound
}

// ByRememberToken retrieves a user by their remember token.
func (s *UserStoreFile) ByRememberToken(token string) (*app.User, error) {
	userTokens, err := s.retrieveUserTokens()
	if err != nil {
		return nil, err
	}
	for _, t := range userTokens {
		if strings.EqualFold(t.RememberToken, token) {
			return s.ByEmail(t.Email)
		}
	}
	return nil, errNotFound
}

// CreateRememberToken creates a new remember token for a user.
func (s *UserStoreFile) CreateRememberToken(user *app.User) (string, error) {
	token, err := rand.RememberToken()
	if err != nil {
		return "", err
	}
	userToken := app.UserToken{
		Email:         user.Email,
		RememberToken: token,
	}

	userTokens, err := s.retrieveUserTokens()
	if err != nil {
		return "", err
	}
	userTokens = append(userTokens, userToken)
	err = s.saveUserTokens(userTokens)
	if err != nil {
		return "", err
	}
	return token, nil
}

// ClearRememberToken clears the remember token in the store.
func (s *UserStoreFile) ClearRememberToken(token string) error {
	userTokens, err := s.retrieveUserTokens()
	if err != nil {
		return err
	}
	tokenIndex := -1
	for i, t := range userTokens {
		if strings.EqualFold(t.RememberToken, token) {
			tokenIndex = i
			break
		}
	}
	if tokenIndex > -1 {
		// https://yourbasic.org/golang/delete-element-slice/
		userTokens[tokenIndex] = userTokens[len(userTokens)-1]
		userTokens[len(userTokens)-1] = app.UserToken{}
		userTokens = userTokens[:len(userTokens)-1]
	}
	return s.saveUserTokens(userTokens)
}

func (s *UserStoreFile) retrieveUsers() ([]app.User, error) {
	s.lock.Lock()
	defer s.lock.Unlock()

	f, err := os.Open("users.gob")
	if err != nil {
		return nil, err
	}
	defer f.Close()

	users := []app.User{}
	dec := gob.NewDecoder(f)
	err = dec.Decode(&users)
	if err != nil {
		return nil, err
	}
	return users, nil
}

func (s *UserStoreFile) retrieveUserTokens() ([]app.UserToken, error) {
	s.lock.Lock()
	defer s.lock.Unlock()

	userTokens := []app.UserToken{}

	f, err := os.Open("usertokens.gob")
	if err != nil {
		if os.IsNotExist(err) {
			return userTokens, nil
		}
		return nil, err
	}
	defer f.Close()

	dec := gob.NewDecoder(f)
	err = dec.Decode(&userTokens)
	if err != nil {
		return nil, err
	}
	return userTokens, nil
}

func (s *UserStoreFile) saveUsers(users []app.User) error {
	s.lock.Lock()
	defer s.lock.Unlock()

	f, err := os.OpenFile("users.gob", os.O_CREATE|os.O_RDWR, 0666)
	if err != nil {
		return err
	}
	defer f.Close()

	enc := gob.NewEncoder(f)
	return enc.Encode(users)
}

func (s *UserStoreFile) saveUserTokens(userTokens []app.UserToken) error {
	s.lock.Lock()
	defer s.lock.Unlock()

	f, err := os.OpenFile("usertokens.gob", os.O_CREATE|os.O_RDWR, 0666)
	if err != nil {
		return err
	}
	defer f.Close()

	enc := gob.NewEncoder(f)
	return enc.Encode(userTokens)
}
