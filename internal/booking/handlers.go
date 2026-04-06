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

type ClassDetailVM struct {
	Chain                 string
	ClassID               string
	ActivityName          string
	Description           string
	AdditionalInformation string
	CancelText            string
	StartTime             time.Time
	EndTime               time.Time
	DurationMinutes       int
	Studio                string
	Room                  string
	Instructors           []string
	IsCancelled           bool
	IsBooked              bool
	IsWaitlisted          bool
	WaitlistPosition      int
	BookedSlots           int
	TotalSlots            int
	WaitingListCount      int
	HasSlotData           bool
}

type ClassSlotsVM struct {
	Chain            string
	ClassID          string
	BookedSlots      int
	TotalSlots       int
	WaitingListCount int
	HasSlotData      bool
}

type DayGroup struct {
	Label    string
	Sessions []SessionVM
}

type BookingData struct {
	Days  []DayGroup
	Error string
}

// genericUpTargets are selectors Unpoly uses for full-page navigation.
// Requests with these targets should receive a full-page response so that
// header and body styling are not lost on back/forward navigation.
var genericUpTargets = map[string]struct{}{
	"main": {}, "body": {}, "html": {}, "[up-main]": {}, ":has(main)": {},
}

func (h *Handler) render(w http.ResponseWriter, r *http.Request, full, fragment string, data any) {
	target := r.Header.Get("X-Up-Target")
	_, isGeneric := genericUpTargets[target]
	name := full
	if target != "" && !isGeneric {
		name = fragment
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.Header().Set("Vary", "X-Up-Target")
	if err := h.Tmpl.ExecuteTemplate(w, name, data); err != nil {
		http.Error(w, "Mal-feil", http.StatusInternalServerError)
	}
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

	h.render(w, r, "bookings.html", "bookings_main", data)
}

func (h *Handler) HandleClassDetail(w http.ResponseWriter, r *http.Request) {
	token, ok := h.Auth.GetAccessToken(w, r)
	if !ok {
		auth.RedirectToLogin(w, r)
		return
	}

	chain := r.PathValue("chain")
	classID := r.PathValue("classId")

	detail, err := h.API.GetClassDetail(token, chain, classID)
	if err != nil {
		if errors.Is(err, api.ErrUnauthorized) {
			auth.RedirectToLogin(w, r)
			return
		}
		http.Error(w, "Kunne ikke hente klasseinformasjon", http.StatusBadGateway)
		return
	}

	sessions, _ := h.API.GetUserSessions(token)
	isBooked := false
	isWaitlisted := false
	var waitlistPosition int
	for _, s := range sessions {
		if s.Chain != chain || s.ClassData.ID != classID {
			continue
		}
		switch s.Status {
		case "BOOKED":
			isBooked = true
		case "WAITLIST":
			isWaitlisted = true
			if s.Position != nil {
				waitlistPosition = *s.Position
			}
		}
	}

	vm := ClassDetailVM{
		Chain:            chain,
		ClassID:          classID,
		ActivityName:     detail.Activity.Name,
		StartTime:        detail.StartTime.In(h.Loc),
		EndTime:          detail.EndTime.In(h.Loc),
		DurationMinutes:  int(detail.EndTime.Sub(detail.StartTime).Minutes()),
		Studio:           detail.Location.Studio,
		IsCancelled:      detail.IsCancelled,
		IsBooked:         isBooked,
		IsWaitlisted:     isWaitlisted,
		WaitlistPosition: waitlistPosition,
	}
	if detail.Activity.Description != nil {
		vm.Description = *detail.Activity.Description
	}
	if detail.Activity.AdditionalInformation != nil {
		vm.AdditionalInformation = *detail.Activity.AdditionalInformation
	}
	if detail.CancelText != nil {
		vm.CancelText = *detail.CancelText
	}
	if detail.Location.Room != nil {
		vm.Room = *detail.Location.Room
	}
	for _, i := range detail.Instructors {
		vm.Instructors = append(vm.Instructors, i.Name)
	}
	if detail.TotalSlots != nil && detail.AvailableSlots != nil {
		vm.BookedSlots = *detail.TotalSlots - *detail.AvailableSlots
		vm.TotalSlots = *detail.TotalSlots
		vm.HasSlotData = true
	}
	if detail.WaitingListCount != nil {
		vm.WaitingListCount = *detail.WaitingListCount
	}

	h.render(w, r, "class_detail.html", "class_detail_main", vm)
}

func (h *Handler) HandleClassSlots(w http.ResponseWriter, r *http.Request) {
	token, ok := h.Auth.GetAccessToken(w, r)
	if !ok {
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}

	chain := r.PathValue("chain")
	classID := r.PathValue("classId")

	detail, err := h.API.GetClassDetail(token, chain, classID)
	if err != nil {
		if errors.Is(err, api.ErrUnauthorized) {
			http.Error(w, "Unauthorized", http.StatusUnauthorized)
			return
		}
		http.Error(w, "Kunne ikke hente klasseinformasjon", http.StatusBadGateway)
		return
	}

	vm := ClassSlotsVM{Chain: chain, ClassID: classID}
	var etag string
	if detail.TotalSlots != nil && detail.AvailableSlots != nil {
		booked := *detail.TotalSlots - *detail.AvailableSlots
		vm.BookedSlots = booked
		vm.TotalSlots = *detail.TotalSlots
		vm.HasSlotData = true
		waitlist := 0
		if detail.WaitingListCount != nil {
			waitlist = *detail.WaitingListCount
			vm.WaitingListCount = waitlist
		}
		etag = fmt.Sprintf(`"%d/%d/%d"`, booked, *detail.TotalSlots, waitlist)
	}

	if etag != "" {
		w.Header().Set("ETag", etag)
		if r.Header.Get("If-None-Match") == etag {
			w.WriteHeader(http.StatusNotModified)
			return
		}
	}

	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	if err := h.Tmpl.ExecuteTemplate(w, "class_slots", vm); err != nil {
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
	h.render(w, r, "cancel_modal.html", "cancel_modal_main", data)
}

func (h *Handler) HandleCancel(w http.ResponseWriter, r *http.Request) {
	token, ok := h.Auth.GetAccessToken(w, r)
	if !ok {
		auth.RedirectToLogin(w, r)
		return
	}

	chain := r.PathValue("chain")
	classID := r.PathValue("classId")

	if err := h.API.CancelBooking(token, chain, classID); err != nil {
		if errors.Is(err, api.ErrUnauthorized) {
			auth.ClearCookie(w, auth.CookieAccess, "/", h.Auth.Cfg.Secure)
			auth.ClearCookie(w, auth.CookieRefresh, "/", h.Auth.Cfg.Secure)
			auth.RedirectToLogin(w, r)
			return
		}
		data := h.buildBookingData(token)
		data.Error = "Kunne ikke avbestille timen. Prøv igjen."
		h.render(w, r, "bookings.html", "bookings_main", data)
		return
	}

	http.Redirect(w, r, "/bookings", http.StatusSeeOther)
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
