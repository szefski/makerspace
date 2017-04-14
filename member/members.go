package member

import (
	"crypto/rand"
	"database/sql"
	"encoding/hex"
	"fmt"
	"github.com/stripe/stripe-go"
	"github.com/vvanpo/makerspace/talk"
	"golang.org/x/crypto/scrypt"
	"log"
	"regexp"
	"time"
)

type Config struct {
	Reserved_usernames        []string
	Password_reset_window     string
	Email_verification_window string
	Smtp                      struct {
		Address  string
		Port     int
		Username string
		Password string
	}
	Billing struct {
		Private_key string
		Public_key  string
	}
}

type Members struct {
	Config
	*sql.DB
	*talk.Talk_api
	Plans map[string]*stripe.Plan
}

func New(config Config, db *sql.DB, talk *talk.Talk_api) *Members {
	stripe.Key = config.Billing.Private_key
	ms := &Members{config, db, talk, make(map[string]*stripe.Plan)}
	ms.load_plans()
	return ms
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

var username_chars_rexp = regexp.MustCompile(`[^\w.-]`)
var username_first_char_rexp = regexp.MustCompile(`^[\W]`)
var username_last_char_rexp = regexp.MustCompile(`[^A-Za-z0-9]$`)
var username_double_special_rexp = regexp.MustCompile(`[-_.]{2,}`)
var username_confusing_suffix_rexp = regexp.MustCompile(`\.(js|json|css|htm|html|xml|jpg|jpeg|png|gif|bmp|ico|tif|tiff|woff)$`)

func (ms *Members) Check_username_availability(username string) (available bool, err string) {
	if username == "" {
		return false, "Username cannot be blank"
	} else if len(username) < 3 {
		return false, "Username must be at least 3 characters"
	} else if len(username) > 20 {
		return false, "Username must be no more than 20 characters"
	} else if username_chars_rexp.MatchString(username) {
		return false, "Username must only include numbers, letters, underscores, hyphens, and periods"
	} else if username_first_char_rexp.MatchString(username) {
		return false, "Username must begin with an underscore or alphanumeric character"
	} else if username_last_char_rexp.MatchString(username) {
		return false, "Username must end with an alphanumeric character"
	} else if username_double_special_rexp.MatchString(username) {
		return false, "Username cannot contain consecutive special characters (underscore, period, or hyphen)"
	} else if username_confusing_suffix_rexp.MatchString(username) {
		return false, "Username must not end in a confusing filetype suffix"
	}
	for _, u := range ms.Config.Reserved_usernames {
		if username == u {
			return false, "Username reserved"
		}
	}
	var count int
	if err := ms.QueryRow(
		"SELECT COUNT(*) "+
			"FROM member "+
			"WHERE username = $1", username).Scan(&count); err != nil {
		log.Panic(err)
	}
	if count == 1 {
		return false, "Username already in use"
	}
	return true, ""
}

var email_rexp = regexp.MustCompile("^[a-zA-Z0-9.!#$%&’*+/=?^_`{|}~-]+@[a-zA-Z0-9-]+(?:\\.[a-zA-Z0-9-]+)*$")

func (ms *Members) Check_email_availability(email string) (available bool, err string) {
	if email == "" {
		return false, "E-mail cannot be blank"
	}
	if !email_rexp.MatchString(email) {
		return false, "Invalid E-mail address"
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

var name_rexp = regexp.MustCompile(`^([\pL\pN\pM\pP]+ ?)+$`)

func validate_name(name string) (bool, error) {
	if !name_rexp.MatchString(name) {
		return false, fmt.Errorf("Invalid characters in name")
	} else if len(name) > 100 {
		return false, fmt.Errorf("Name must be no more than 100 characters")
	}
	return true, nil
}

// New creates a new user, returns nil and a set of errors on invalid input.
//	Only checks for e-mail availability, does not send off a verification e-mail
//	or otherwise store the e-mail address.  The new member is created with an
//	uninitialized password, which must be set via the reset form.
func (ms *Members) New_member(username, email, name string) (m *Member, err map[string]string) {
	err = make(map[string]string)
	m = &Member{
		Username: username,
		Name:     name,
		Members:  ms}
	if ok, e := validate_name(name); !ok {
		err["name_error"] = e.Error()
		m = nil
	}
	if available, e := ms.Check_username_availability(username); !available {
		err["username_error"] = e
		m = nil
	}
	if available, e := ms.Check_email_availability(email); !available {
		err["email_error"] = e
		m = nil
	}
	if m == nil {
		return
	}
	if e := m.QueryRow(
		"INSERT INTO member ("+
			"	username,"+
			"	name"+
			") "+
			"VALUES ($1, $2) "+
			"RETURNING id, registered",
		username, name).Scan(&m.Id, &m.Registered); e != nil {
		log.Panic(e)
	}
	return m, nil
}

func parse_duration(w string) (time.Duration, error) {
	var weeks int
	if w == "1 week" {
		w = fmt.Sprintf("%dh", 7*24)
	} else if n, err := fmt.Sscanf(w, "%d weeks", &weeks); n == 1 && err == nil {
		w = fmt.Sprintf("%dh", 7*24*weeks)
	}
	return time.ParseDuration(w)
}

func (ms *Members) Get_member_from_reset_token(token string) (*Member, error) {
	var member_id int
	var t time.Time
	if err := ms.QueryRow(
		"SELECT member, time "+
			"FROM reset_password_token "+
			"WHERE token = $1", token).Scan(&member_id, &t); err != nil {
		if err == sql.ErrNoRows {
			return nil, fmt.Errorf("Reset token does not exist")
		}
		log.Panic(err)
	}
	duration, err := parse_duration(ms.Config.Password_reset_window)
	if err != nil {
		log.Panic(err)
	}
	expires := t.Add(duration)
	if time.Now().After(expires) {
		if _, err := ms.Exec("DELETE FROM reset_password_token "+
			"WHERE token = $1", token); err != nil {
			log.Panic(err)
		}
		return nil, fmt.Errorf("Reset token is expired")
	}
	return ms.Get_member_by_id(member_id), nil
}

func (ms *Members) Get_member_from_verification_token(token string) (*Member, string, error) {
	var (
		member_id int
		email     string
		t         time.Time
	)
	if err := ms.QueryRow(
		"SELECT member, email, time "+
			"FROM email_verification_token "+
			"WHERE token = $1", token).Scan(&member_id, &email, &t); err != nil {
		if err == sql.ErrNoRows {
			return nil, "", fmt.Errorf("Verification token does not exist")
		}
		log.Panic(err)
	}
	duration, err := parse_duration(
		ms.Config.Email_verification_window)
	if err != nil {
		log.Panic(err)
	}
	if time.Now().After(t.Add(duration)) {
		if _, err := ms.Exec("DELETE FROM email_verification_token "+
			"WHERE token = $1", token); err != nil {
			log.Panic(err)
		}
		return nil, "", fmt.Errorf("Verification token is expired")
	}
	return ms.Get_member_by_id(member_id), email, nil
}

func (ms *Members) get_members(query string) []*Member {
	members := make([]*Member, 0)
	rows, err := ms.Query(query)
	defer rows.Close()
	if err != nil {
		if err != sql.ErrNoRows {
			log.Panic(err)
		}
		return members
	}
	for rows.Next() {
		var member_id int
		if err = rows.Scan(&member_id); err != nil {
			log.Panic(err)
		}
		members = append(members, ms.Get_member_by_id(member_id))
	}
	return members
}

// Grabs all e-mail-verified members
func (ms *Members) Get_all_members() []*Member {
	return ms.get_members(
		"SELECT id " +
			"FROM member " +
			"WHERE email IS NOT NULL " +
			"ORDER BY username ASC")
}

func (ms *Members) Get_all_active_members() []*Member {
	return ms.get_members(
		"SELECT m.id " +
			"FROM member m " +
			"JOIN session_http s " +
			"ON s.member = m.id " +
			"GROUP BY m.id " +
			"ORDER BY max(s.last_seen) DESC")
}

func (ms *Members) Get_new_members(limit int) []*Member {
	return ms.get_members(
		"SELECT id " +
			"FROM member " +
			"ORDER BY registered DESC " +
			"LIMIT " + fmt.Sprint(limit))
}
