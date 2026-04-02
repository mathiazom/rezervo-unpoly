package booking

import (
	"errors"
	"fmt"
	"html/template"
	"net/http"
	"sort"
	"time"

	"github.com/mathiazom/rezervo-unpoly/internal/api"
	"github.com/mathiazom/rezervo-unpoly/internal/auth"
)

type Handler struct {
	Auth *auth.Handler
	API  *api.Client
	Tmpl *template.Template
	Loc  *time.Location
}

type SessionVM struct {
	Chain        string
	ClassID      string
	ActivityName string
	StartTime    time.Time
	EndTime      time.Time
	Studio       string
	Instructors  []string
	Status       string
	StatusClass  string
}

type DayGroup struct {
	Label    string
	Sessions []SessionVM
}

type BookingData struct {
	Days  []DayGroup
	Error string
}

func (h *Handler) HandleIndex(w http.ResponseWriter, r *http.Request) {
	http.Redirect(w, r, "/bookings", http.StatusFound)
}

func (h *Handler) HandleBookings(w http.ResponseWriter, r *http.Request) {
	token, ok := h.Auth.GetAccessToken(w, r)
	if !ok {
		auth.RedirectToLogin(w, r)
		return
	}

	data := h.buildBookingData(token)

	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	if err := h.Tmpl.ExecuteTemplate(w, "bookings.html", data); err != nil {
		http.Error(w, "Mal-feil", http.StatusInternalServerError)
	}
}

func (h *Handler) HandleCancelModal(w http.ResponseWriter, r *http.Request) {
	data := struct {
		Chain        string
		ClassID      string
		ActivityName string
	}{
		Chain:        r.PathValue("chain"),
		ClassID:      r.PathValue("classId"),
		ActivityName: r.URL.Query().Get("name"),
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	if err := h.Tmpl.ExecuteTemplate(w, "cancel_modal.html", data); err != nil {
		http.Error(w, "Mal-feil", http.StatusInternalServerError)
	}
}

func (h *Handler) HandleCancel(w http.ResponseWriter, r *http.Request) {
	token, ok := h.Auth.GetAccessToken(w, r)
	if !ok {
		auth.RedirectToLogin(w, r)
		return
	}

	chain := r.PathValue("chain")
	classID := r.PathValue("classId")

	var cancelErr string
	if err := h.API.CancelBooking(token, chain, classID); err != nil {
		if errors.Is(err, api.ErrUnauthorized) {
			auth.ClearCookie(w, auth.CookieAccess, "/", h.Auth.Cfg.Secure)
			auth.ClearCookie(w, auth.CookieRefresh, "/", h.Auth.Cfg.Secure)
			auth.RedirectToLogin(w, r)
			return
		}
		cancelErr = "Kunne ikke avbestille timen. Prøv igjen."
	}

	data := h.buildBookingData(token)
	if cancelErr != "" {
		data.Error = cancelErr
	}

	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	if err := h.Tmpl.ExecuteTemplate(w, "bookings.html", data); err != nil {
		http.Error(w, "Mal-feil", http.StatusInternalServerError)
	}
}

func (h *Handler) buildBookingData(token string) BookingData {
	sessions, err := h.API.GetUserSessions(token)
	if err != nil {
		if errors.Is(err, api.ErrUnauthorized) {
			return BookingData{Error: "Ikke autorisert"}
		}
		return BookingData{Error: "Kunne ikke hente timer"}
	}

	var vms []SessionVM
	for _, s := range sessions {
		if s.Status != "BOOKED" && s.Status != "WAITLIST" && s.Status != "PLANNED" {
			continue
		}
		vms = append(vms, toSessionVM(s, h.Loc))
	}

	return BookingData{Days: groupByDay(vms, h.Loc)}
}

func toSessionVM(s api.UserSession, loc *time.Location) SessionVM {
	status := "Ukjent"
	statusClass := "bg-zinc-500/20 text-zinc-400"

	switch s.Status {
	case "BOOKED":
		status = "Booket"
		statusClass = "bg-green-500/20 text-green-600 dark:text-green-400"
	case "WAITLIST":
		if s.Position != nil {
			status = fmt.Sprintf("Venteliste #%d", *s.Position)
		} else {
			status = "Venteliste"
		}
		statusClass = "bg-yellow-500/20 text-yellow-600 dark:text-yellow-400"
	case "PLANNED":
		status = "Planlagt"
		statusClass = "bg-blue-500/20 text-blue-600 dark:text-blue-400"
	}

	var instructors []string
	for _, i := range s.ClassData.Instructors {
		instructors = append(instructors, i.Name)
	}

	return SessionVM{
		Chain:        s.Chain,
		ClassID:      s.ClassData.ID,
		ActivityName: s.ClassData.Activity.Name,
		StartTime:    s.ClassData.StartTime.In(loc),
		EndTime:      s.ClassData.EndTime.In(loc),
		Studio:       s.ClassData.Location.Studio,
		Instructors:  instructors,
		Status:       status,
		StatusClass:  statusClass,
	}
}

func groupByDay(sessions []SessionVM, loc *time.Location) []DayGroup {
	sort.Slice(sessions, func(i, j int) bool {
		return sessions[i].StartTime.Before(sessions[j].StartTime)
	})

	var groups []DayGroup
	var currentLabel string

	for _, s := range sessions {
		label := formatDay(s.StartTime.In(loc))
		if label != currentLabel {
			groups = append(groups, DayGroup{Label: label})
			currentLabel = label
		}
		groups[len(groups)-1].Sessions = append(groups[len(groups)-1].Sessions, s)
	}

	return groups
}

var norWeekdays = [7]string{
	"Søndag", "Mandag", "Tirsdag", "Onsdag", "Torsdag", "Fredag", "Lørdag",
}

var norMonths = [13]string{
	"", "januar", "februar", "mars", "april", "mai", "juni",
	"juli", "august", "september", "oktober", "november", "desember",
}

func formatDay(t time.Time) string {
	return fmt.Sprintf("%s %d. %s", norWeekdays[t.Weekday()], t.Day(), norMonths[t.Month()])
}
