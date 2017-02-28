package member

import (
	"crypto/rand"
	"database/sql"
	"encoding/hex"
	"github.com/vvanpo/makerspace/billing"
	"github.com/vvanpo/makerspace/talk"
	"golang.org/x/crypto/scrypt"
	"log"
	"regexp"
)

type Members struct {
	*sql.DB
	*talk.Talk_api
	*billing.Billing
}

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

//	username_rexp := regexp.MustCompile("^[\\pL\\pN\\pM\\pP]+$")
var username_rexp = regexp.MustCompile(``)
var name_rexp = regexp.MustCompile(`^(?:[\pL\pN\pM\pP]+ ?)+$`)

func (ms *Members) Check_username_availability(username string) (available bool, err string) {
	if username == "" {
		return false, "Username cannot be blank"
	}
	var count int
	if err := ms.QueryRow(
		"SELECT COUNT(*) "+
			"FROM member "+
			"WHERE username = $1", username).Scan(&count); err != nil {
		log.Panic(err)
	}
	if count == 1 {
		return false, "Username not available"
	}
	return ms.Check_username(username)
}

func (ms *Members) Check_email_availability(email string) (available bool, err string) {
	if email == "" {
		return false, "E-mail cannot be blank"
	}
	var count int
	if err := ms.QueryRow(
		"SELECT COUNT(*) "+
			"FROM member "+
			"WHERE email = $1", email).Scan(&count); err != nil {
		log.Panic(err)
	}
	if count == 0 {
		return true, ""
	}
	return false, "E-mail already in use"
}

// New creates a new user, but will panic if the username already exists.
//	Will create members with invalid usernames, so call Check_username() first.
//	Returns nil if the name is invalid.
func (ms *Members) New_member(username, name, email, password string) *Member {
	//TODO: Ideally, all members are created through the join page when the talk
	//	server is running, as it queries discourse's check_username.json api to
	//	ensure usernames are compliant with discourse.
	if !name_rexp.MatchString(name) || len(name) > 100 {
		return nil
	}
	salt := Rand256()
	m := &Member{
		Username:      username,
		Name:          name,
		Email:         email,
		password_key:  key(password, salt),
		password_salt: salt,
		Members:       ms}
	if err := m.QueryRow(
		"INSERT INTO member ("+
			"	username,"+
			"	name,"+
			"	password_key,"+
			"	password_salt,"+
			"	email"+
			") "+
			"VALUES ($1, $2, $3, $4, $5) "+
			"RETURNING id, registered",
		username, name, m.password_key, salt, email).Scan(&m.Id, &m.Registered); err != nil {
		log.Panic(err)
	}
	return m
}

func (ms *Members) Get_all_members() []*Member {
	members := make([]*Member, 0)
	rows, err := ms.Query(
		"SELECT " +
			"	m.id, " +
			"	m.username, " +
			"	m.name, " +
			"	m.password_key, " +
			"	m.password_salt, " +
			"	m.email, " +
			"	m.agreed_to_terms, " +
			"	m.registered, " +
			"	s.username IS NOT NULL, " +
			"	a.username IS NOT NULL " +
			"FROM member m " +
			"NATURAL LEFT JOIN administrator a " +
			"NATURAL LEFT JOIN student s")
	defer rows.Close()
	if err != nil {
		if err != sql.ErrNoRows {
			log.Panic(err)
		}
		return members
	}
	for rows.Next() {
		var password_key, password_salt sql.NullString
		m := &Member{Members: ms}
		if err = rows.Scan(&m.Id, &m.Username, &m.Name, &password_key,
			&password_salt, &m.Email, &m.Agreed_to_terms, &m.Registered,
			&m.Student, &m.Admin); err != nil {
			log.Panic(err)
		}
		m.password_key = password_key.String
		m.password_salt = password_salt.String
		members = append(members, m)
	}
	return members
}

/*
func (ms *Members) Get_all_active_members() []*Member {
	members := make([]*Member, 0)
	//TODO: BUG: should by on f.category = 'membership'
	rows, err := ms.db.Query("SELECT m.id, m.username, m.name, m.password_key, m.password_salt, m.email, m.agreed_to_terms, m.registered, s.username IS NOT NULL, a.username IS NOT NULL FROM member m NATURAL LEFT JOIN administrator a NATURAL LEFT JOIN student s JOIN (SELECT COALESCE(i.paid_by, i.username) AS paid_by FROM invoice i LEFT JOIN fee f ON (i.fee = f.id) WHERE COALESCE(i.recurring, f.recurring) = '1 month' AND (i.end_date > now() OR i.end_date IS NULL)) inv ON inv.paid_by = m.username")
	defer rows.Close()
	if err != nil {
		if err != sql.ErrNoRows {
			log.Panic(err)
		}
		return members
	}
	for rows.Next() {
		m := &Member{Members: ms}
		if err = rows.Scan(&m.Id, &m.Username, &m.Name, &m.password_key, &m.password_salt, &m.Email, &m.Agreed_to_terms, &m.Registered, &m.Student, &m.Admin); err != nil {
			log.Panic(err)
		}
	}
	return members
}*/