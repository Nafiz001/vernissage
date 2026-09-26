package api

import (
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"fmt"
	"strconv"
	"strings"
	"time"
)

// A live-room ticket carries who a visitor is from a page served on one
// domain to a WebSocket on another. When the site is on Vercel and this
// server on Render, the browser opens the socket without the site's
// cookies; the page asks for a ticket through the same-origin API first and
// hands it over in the URL. Tickets are signed and last a minute.
type tickets struct{ key []byte }

const ticketTTL = time.Minute

func newTickets(secret string) tickets {
	key := []byte(secret)
	if len(key) == 0 {
		key = make([]byte, 32)
		rand.Read(key)
	}
	return tickets{key: key}
}

func (t tickets) sign(payload string) string {
	m := hmac.New(sha256.New, t.key)
	m.Write([]byte(payload))
	return base64.RawURLEncoding.EncodeToString(m.Sum(nil))
}

func (t tickets) issue(visitor string, userID int64) string {
	payload := fmt.Sprintf("%s|%d|%d", visitor, userID, time.Now().Add(ticketTTL).Unix())
	return base64.RawURLEncoding.EncodeToString([]byte(payload)) + "." + t.sign(payload)
}

func (t tickets) check(ticket string) (visitor string, userID int64, ok bool) {
	body, sig, found := strings.Cut(ticket, ".")
	if !found {
		return "", 0, false
	}
	raw, err := base64.RawURLEncoding.DecodeString(body)
	if err != nil || !hmac.Equal([]byte(sig), []byte(t.sign(string(raw)))) {
		return "", 0, false
	}
	parts := strings.Split(string(raw), "|")
	if len(parts) != 3 {
		return "", 0, false
	}
	uid, err1 := strconv.ParseInt(parts[1], 10, 64)
	exp, err2 := strconv.ParseInt(parts[2], 10, 64)
	if err1 != nil || err2 != nil || time.Now().Unix() > exp || parts[0] == "" {
		return "", 0, false
	}
	return parts[0], uid, true
}
