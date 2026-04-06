package main

import (
	"embed"
	"html/template"
	"log"
	"net/http"
	"net/url"
	"time"

	"github.com/mathiazom/rezervo-unpoly/internal/api"
	"github.com/mathiazom/rezervo-unpoly/internal/auth"
	"github.com/mathiazom/rezervo-unpoly/internal/booking"
	"github.com/mathiazom/rezervo-unpoly/internal/config"
)

//go:embed templates
var templateFS embed.FS

//go:embed static
var staticFS embed.FS

func main() {
	cfg, err := config.Load()
	if err != nil {
		log.Fatal(err)
	}

	loc, err := time.LoadLocation("Europe/Oslo")
	if err != nil {
		log.Printf("warn: kunne ikke laste Europe/Oslo tidssone, bruker UTC: %v", err)
		loc = time.UTC
	}

	funcMap := template.FuncMap{
		"formatTime": func(t time.Time) string {
			return t.In(loc).Format("15:04")
		},
		"urlquery": url.QueryEscape,
	}

	tmpl := template.Must(
		template.New("").Funcs(funcMap).ParseFS(templateFS, "templates/*.html"),
	)

	authHandler := &auth.Handler{
		Cfg:  cfg,
		Tmpl: tmpl,
	}

	bookingHandler := &booking.Handler{
		Auth: authHandler,
		API:  api.NewClient(cfg.APIURL),
		Tmpl: tmpl,
		Loc:  loc,
	}

	mux := http.NewServeMux()

	mux.Handle("/static/", http.FileServer(http.FS(staticFS)))

	mux.HandleFunc("GET /login", authHandler.RenderLogin)
	mux.HandleFunc("GET /auth/start", authHandler.HandleAuthStart)
	mux.HandleFunc("GET /callback", authHandler.HandleCallback)
	mux.HandleFunc("POST /logout", authHandler.HandleLogout)

	mux.HandleFunc("GET /{$}", bookingHandler.HandleIndex)
	mux.HandleFunc("GET /bookings", bookingHandler.HandleBookings)
	mux.HandleFunc("GET /classes/{chain}/{classId}", bookingHandler.HandleClassDetail)
	mux.HandleFunc("GET /classes/{chain}/{classId}/slots", bookingHandler.HandleClassSlots)
	mux.HandleFunc("GET /cancel-modal/{chain}/{classId}", bookingHandler.HandleCancelModal)
	mux.HandleFunc("POST /cancel/{chain}/{classId}", bookingHandler.HandleCancel)

	log.Printf("Server starter på :%s", cfg.Port)
	log.Fatal(http.ListenAndServe(":"+cfg.Port, mux))
}
