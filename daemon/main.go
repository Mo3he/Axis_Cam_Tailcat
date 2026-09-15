// Command Tailcat is the ACAP entry binary. It runs a tailcat server that
// forwards selected camera ports over Tailscale's data plane, and serves the
// settings/status API consumed by the ACAP web UI.
//
// There is no C supervisor and no axparameter use: the camera's manifest
// reverseProxy points straight at this process, and configuration lives in
// localdata. That keeps settings working on devices without param.cgi.
package main

import (
	"encoding/json"
	"log"
	"net/http"
	"os"
	"os/signal"
	"path"
	"strings"
	"syscall"
	"time"
)

// tailcatVersion is reported to the UI. Overridden at build time with
// -ldflags "-X main.tailcatVersion=...".
var tailcatVersion = "0.6.0"

const (
	apiAddr      = "127.0.0.1:2208"
	appDir       = "/usr/local/packages/Tailcat"
	shutdownGrit = 10 * time.Second
)

func main() {
	log.SetFlags(0)

	stateDir := os.Getenv("TAILCAT_STATE_DIR")
	if stateDir == "" {
		stateDir = path.Join(appDir, "localdata")
	}

	store, err := NewStore(stateDir)
	if err != nil {
		log.Fatalf("config: %v", err)
	}
	mgr := NewManager(store, stateDir)

	if store.Get().AutoStart {
		if err := mgr.Start(); err != nil {
			log.Printf("auto-start failed: %v", err)
		}
	}

	srv := &http.Server{
		Addr:              apiAddr,
		Handler:           newAPI(store, mgr),
		ReadHeaderTimeout: 10 * time.Second,
	}
	go func() {
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatalf("settings API: %v", err)
		}
	}()

	stop := make(chan os.Signal, 1)
	signal.Notify(stop, syscall.SIGTERM, syscall.SIGINT)
	<-stop

	srv.Close()
	mgr.Stop()
}

// newAPI routes on the last path segment. The camera's reverseProxy maps
// /local/Tailcat/api/... onto this server, and whether it forwards the "api"
// prefix is not guaranteed, so matching the trailing segment keeps both
// behaviours working.
func newAPI(store *Store, mgr *Manager) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Cache-Control", "no-store")

		switch lastSegment(r.URL.Path) {
		case "settings":
			handleSettings(w, r, store, mgr)
		case "status":
			writeJSON(w, http.StatusOK, mgr.Status())
		case "start":
			if !requirePost(w, r) {
				return
			}
			if err := mgr.Start(); err != nil {
				writeError(w, http.StatusInternalServerError, err.Error())
				return
			}
			writeJSON(w, http.StatusOK, mgr.Status())
		case "stop":
			if !requirePost(w, r) {
				return
			}
			mgr.Stop()
			writeJSON(w, http.StatusOK, mgr.Status())
		case "forget-key":
			if !requirePost(w, r) {
				return
			}
			if err := mgr.ForgetKey(); err != nil {
				writeError(w, http.StatusInternalServerError, err.Error())
				return
			}
			writeJSON(w, http.StatusOK, mgr.Status())
		default:
			writeError(w, http.StatusNotFound, "unknown endpoint")
		}
	})
}

func handleSettings(w http.ResponseWriter, r *http.Request, store *Store, mgr *Manager) {
	switch r.Method {
	case http.MethodGet:
		writeJSON(w, http.StatusOK, store.Get())
	case http.MethodPost, http.MethodPut:
		cfg := store.Get()
		if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, 64<<10)).Decode(&cfg); err != nil {
			writeError(w, http.StatusBadRequest, "invalid JSON: "+err.Error())
			return
		}
		if err := store.Set(cfg); err != nil {
			writeError(w, http.StatusBadRequest, err.Error())
			return
		}
		// Apply immediately so the UI never shows settings that differ from
		// what the live tunnel is actually serving.
		if mgr.Status().Running {
			if err := mgr.Restart(); err != nil {
				writeError(w, http.StatusInternalServerError, "saved, but restart failed: "+err.Error())
				return
			}
		}
		writeJSON(w, http.StatusOK, store.Get())
	default:
		writeError(w, http.StatusMethodNotAllowed, "use GET or POST")
	}
}

func requirePost(w http.ResponseWriter, r *http.Request) bool {
	if r.Method != http.MethodPost {
		writeError(w, http.StatusMethodNotAllowed, "use POST")
		return false
	}
	return true
}

func lastSegment(p string) string {
	p = strings.TrimSuffix(p, "/")
	if i := strings.LastIndexByte(p, '/'); i >= 0 {
		return p[i+1:]
	}
	return p
}

func writeJSON(w http.ResponseWriter, code int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(code)
	json.NewEncoder(w).Encode(v)
}

func writeError(w http.ResponseWriter, code int, msg string) {
	writeJSON(w, code, map[string]string{"error": msg})
}
