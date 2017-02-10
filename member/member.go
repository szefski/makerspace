package member

import (
	"crypto/rand"
	"database/sql"
	"encoding/hex"
	"golang.org/x/crypto/scrypt"
	"log"
	"time"
)

func Rand256() string {
	n := make([]byte, 32)
	_, err := rand.Read(n)
	if err != nil {
		log.Panic(err)
	}
	return hex.EncodeToString(n)
}

func key(password, salt string) string {
	s, err := hex.DecodeString(salt)
	if err != nil {
		log.Panicf("Invalid salt: %s", err)
	}
	key, err := scrypt.Key([]byte(password), s, 16384, 8, 1, 32)
	if err != nil {
		log.Panic(err)
	}
	return hex.EncodeToString(key)
}

type Member struct {
	Username      string
	Name          string
	Email         string
	Registered    time.Time
	Active		  bool
	password_key  string
	password_salt string
	db			  *sql.DB
}

// New creates a new user, but will panic if the username already exists
func New(username, name, email, password string, db *sql.DB) *Member {
	salt := Rand256()
	m := &Member{
		Username: username,
		Name: name,
		Email: email,
		Registered: time.Now(),
		password_key: key(password, salt),
		password_salt: salt,
		db: db}
	_, err := db.Exec("INSERT INTO member (username, name, password_key, password_salt, email, registered) VALUES ($1, $2, $3, $4, $5, $6)", username, name, m.password_key, salt, email, m.Registered)
	if err != nil {
		log.Panic(err)
	}
	return m
}

func Get(username string, db *sql.DB) *Member {
	m := &Member{}
	if err := db.QueryRow("SELECT username, name, password_key, password_salt, email, registered FROM member WHERE username = $1").Scan(m.Username, m.Name, m.password_key, m.password_salt, m.Registered); err != nil {
		if err == sql.ErrNoRows {
			return nil
		}
		log.Panic(err)
	}
	// Check if member is active
	var n int
	if err := db.QueryRow("SELECT COUNT(*) FROM billing WHERE username = $1 AND name = $2 AND (end_date > now() OR end_date IS NULL)", m.Username, "Membership dues").Scan(&n); err != nil {
		log.Panic(err)
	}
	if n == 1 {
		m.Active = true
	}
	return m
}

func (m *Member) Authenticate(password string) bool {
	if m.password_key == key(password, m.password_salt) {
		return true
	}
	return false
}