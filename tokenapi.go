package main

// The token API behind the connect dialog. A token lets an agent write to moki,
// so the browser makes tokens only when moki knows who asks: past basic auth, or
// on this machine.

import (
	"net"
	"net/http"
	"net/url"
)

func (s *Server) tokenRoutes() {
	s.mux.HandleFunc("POST /api/tokens", s.apiMakeToken)
	s.mux.HandleFunc("GET /api/agent", s.apiAgent)
}

// apiAgent names the agent that connected last, so the first-run page can say it is
// connected. It is "" until an agent sends its first MCP request.
func (s *Server) apiAgent(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, r, struct {
		Name string `json:"name"`
	}{s.tokens.LastAgent()})
}

// apiMakeToken makes a token and returns the guide with it, set to the URL in the
// browser: canChange checked that the Origin is this moki.
func (s *Server) apiMakeToken(w http.ResponseWriter, r *http.Request) {
	if !s.canChange(w, r) {
		return
	}
	tok, err := s.tokens.Make()
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	activity.Browser("made the token " + tok.Name)
	writeJSON(w, r, struct {
		Name string `json:"name"`
		Text string `json:"text"`
	}{tok.Name, guideText(r.Header.Get("Origin"), &tok)})
}

// canChange checks a request that makes or revokes a token. It must come from a
// moki page: a page on another site cannot send it, even with the password cached.
func (s *Server) canChange(w http.ResponseWriter, r *http.Request) bool {
	if !sameOrigin(w, r) {
		return false
	}
	if !s.canManage(r) {
		http.Error(w, `make tokens with "moki token register" on the moki machine, or set MOKI_USERNAME and MOKI_PASSWORD`, http.StatusForbidden)
		return false
	}
	return true
}

// sameOrigin checks that a request that changes something comes from a moki page.
func sameOrigin(w http.ResponseWriter, r *http.Request) bool {
	origin, err := url.Parse(r.Header.Get("Origin"))
	if err != nil || origin.Host != r.Host {
		http.Error(w, "this can be changed only from moki itself", http.StatusForbidden)
		return false
	}
	return true
}

// canManage is true past basic auth, or for a request from this machine to a loopback
// name. The name check stops other sites that point their domain at 127.0.0.1.
func (s *Server) canManage(r *http.Request) bool {
	if s.cfg.Username != "" {
		return true // ServeHTTP checked the password
	}
	remote, _, err := net.SplitHostPort(r.RemoteAddr)
	return err == nil && isLoopback(remote) && isLoopback(hostName(r.Host))
}

func hostName(hostport string) string {
	if h, _, err := net.SplitHostPort(hostport); err == nil {
		return h
	}
	return hostport
}
