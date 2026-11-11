package synowake

import (
	"crypto/hmac"
	"crypto/rand"
	"encoding/base64"
	"errors"
	"net"
	"net/http"
	"net/url"
	"strings"
)

var errWakeLink = errors.New("Wecklink ungültig oder nicht verfügbar.")

func (a *App) manageWakeLink(r *http.Request, s Store) (any, error) {
	var input struct {
		ID        string `json:"id"`
		Operation string `json:"operation"`
	}
	if err := decodeBody(r, &input); err != nil {
		return nil, err
	}
	var token string
	edit := func(store *Store) error {
		for _, device := range store.Devices {
			if device.ID != input.ID {
				continue
			}
			token = store.WakeLinks[device.ID]
			switch input.Operation {
			case "get":
			case "create", "rotate":
				if token == "" || input.Operation == "rotate" {
					var random [16]byte
					if _, err := rand.Read(random[:]); err != nil {
						return err
					}
					token = base64.RawURLEncoding.EncodeToString(random[:])
					if store.WakeLinks == nil {
						store.WakeLinks = make(map[string]string)
					}
					store.WakeLinks[device.ID] = token
				}
			case "revoke":
				delete(store.WakeLinks, device.ID)
				token = ""
			default:
				return errors.New("Unbekannte Wecklink-Aktion.")
			}
			return nil
		}
		return errors.New("Gerät nicht gefunden.")
	}
	var err error
	if input.Operation == "get" {
		err = edit(&s)
	} else {
		err = a.update(edit)
	}
	return map[string]string{"token": token}, err
}

func (a *App) wakeLinkHTTP(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet && r.Method != http.MethodPost {
		w.Header().Set("Allow", "GET, POST")
		writeJSON(w, http.StatusMethodNotAllowed, nil, errors.New("HTTP-Methode nicht zulässig."))
		return
	}
	query, err := url.ParseQuery(r.URL.RawQuery)
	token := query.Get("w")
	if err != nil || len(query) != 1 || len(query["w"]) != 1 || len(token) != 22 ||
		r.Header.Get("Sec-Fetch-Site") == "cross-site" || r.Header.Get("Sec-Purpose") != "" || r.Header.Get("Purpose") != "" {
		writeJSON(w, http.StatusForbidden, nil, errWakeLink)
		return
	}
	// Use the CGI peer address, never a client-supplied forwarding header.
	addr := metadataFor(r).ClientAddress
	if host, _, err := net.SplitHostPort(addr); err == nil {
		addr = host
	}
	ip := lanIPv4(strings.Trim(addr, "[]"))
	networks, networkErr := localNetworks()
	if a.Demo {
		local, _ := parseLocalNetwork("Demo-LAN", "192.168.1.2/24")
		networks, networkErr = []LocalNetwork{local}, nil
	}
	if ip == nil || networkErr != nil || !isOnLinkHost(ip, networks) {
		writeJSON(w, http.StatusForbidden, nil, errWakeLink)
		return
	}
	s, err := a.readStore()
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, nil, errWakeLink)
		return
	}
	var id string
	for deviceID, secret := range s.WakeLinks {
		if hmac.Equal([]byte(secret), []byte(token)) {
			id = deviceID
			break
		}
	}
	if id == "" {
		writeJSON(w, http.StatusForbidden, nil, errWakeLink)
		return
	}
	_, err = a.wakeDeviceClaim(id, false, "url", "", func(current *Store) error {
		// Recheck under the store lock so an old token cannot start a new wake
		// after it is rotated, revoked, or removed together with the device.
		if !hmac.Equal([]byte(current.WakeLinks[id]), []byte(token)) {
			return errWakeLink
		}
		return nil
	})
	switch {
	case err == nil:
		writeJSON(w, http.StatusOK, map[string]bool{"sent": true}, nil)
	case errors.Is(err, errWakeLink):
		writeJSON(w, http.StatusForbidden, nil, errWakeLink)
	case errors.Is(err, ErrStopped):
		writeJSON(w, http.StatusConflict, nil, err)
	case errors.Is(err, errWakeCooldown):
		writeJSON(w, http.StatusTooManyRequests, nil, err)
	default:
		writeJSON(w, http.StatusInternalServerError, nil, errors.New("Aufwecken per Wecklink fehlgeschlagen. Protokoll prüfen."))
	}
}
