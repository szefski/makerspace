package site

import (
	"github.com/lib/pq"
	"github.com/vvanpo/makerspace/member"
)

func (s *Http_server) session_cookie(value string) http.Cookie {
	return http.Cookie{Name: "session",
		Value:  value,
		Path:   "/",
		Domain: s.config.Domain,
		/* Secure: true, */
		HttpOnly: true}
}

type member_info struct {
	*member.Member
	Talk map[string]interface{}
	Billing *billing.Profile
}

type session struct {
	token  string
	Member *member_info
	cookie http.Cookie
	server *Http_server
}

// new_session creates a new session, without setting the cookie.
func (s *Http_server) new_session(m *member.Member, expires bool) *session {
	token := member.Rand256()
	if _, err := s.db.Exec("INSERT INTO session_http (token, username) VALUES ($1, $2)", token, m.Username); err != nil {
		log.Panic(err)
	}
	cookie := s.session_cookie(token)
	// If not set to expire, set expiry date for a year from now.
	if !expires {
		cookie.Expires = time.Now().AddDate(1, 0, 0)
		if _, err := s.db.Exec("UPDATE session_http SET expires = $1 WHERE token = $2", cookie.Expires, token); err != nil {
			log.Panic(err)
		}
	}
	return &session{
		token: token,
		Member: &member_info{m},
		cookie: cookie,
		server: s}
}

// authenticate validates the session cookie, returning nil if invalid
func (h *Http_server) authenticate(r *http.Request) *session {
	cookie, err := r.Cookie("session")
	if err != nil {
		return nil
	}
	var username string
	var expires pq.NullTime
	if err := h.db.QueryRow("SELECT username FROM session_http WHERE token = $1", cookie.Value).Scan(&username); err != nil && err != sql.ErrNoRows {
		log.Panic(err)
	} else if err == sql.ErrNoRows || (expires.Valid && expires.Time.Before(time.Now())) {
		/// TODO: invalidate cookie in response, and clean up the above expr
		return nil
	}
	s := &session{token: cookie.Value,
				Member: &member_info{member.Get(username)},
				cookie: h.session_cookie(cookie.Value),
				server: h}
	s.update()
	/// TODO: decode talk user data
	return s
}

// destroy destroys the session, but does not remove the cookie.  The session
//	object must not be used after destroy(), as session methods assume a valid
//	object.
//	destroy panics if the session doesn't exist, or if it is called twice.
func (s *session) destroy() {
	if _, err := s.server.db.Exec("UPDATE session_http SET expires = 'epoch' WHERE token = $1", s.token); err != nil {
		log.Panic(err)
	}
}

// update creates a new token for the session and updates the expiry date, if it
//	exists.  It does not actually set the resulting cookie.  update() will panic
//	if the session doesn't exist.
func (s *session) update() {
	// We first find the expiry date to update it by a year and add it to the
	//	cookie if it exists
	var expires pq.NullTime
	if err := s.server.db.QueryRow("SELECT expires FROM session_http WHERE token = $1", s.token).Scan(&expires); err != nil {
		log.Panic(err)
	}
	token := member.Rand256()
	if expires.Valid {
		expires.Time = time.Now().AddDate(1, 0, 0)
		s.cookie.Expires = expires.Time
	}
	if _, err := s.server.db.Exec("UPDATE session_http SET token = $1, last_seen = now(), expires = $2 WHERE token = $3", token, expires, s.token); err != nil {
		log.Panic(err)
	}
	s.token = token
	s.cookie.Value = token
}