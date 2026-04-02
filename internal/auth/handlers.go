package auth

import (
	"encoding/json"
	"fmt"
	"html/template"
	"net/http"
	"net/url"
	"time"

	"github.com/mathiazom/rezervo-unpoly/internal/config"
)

const (
	CookieAccess  = "rz_access"
	CookieRefresh = "rz_refresh"
	CookiePKCE    = "rz_pkce"
)

type Handler struct {
	Cfg  *config.Config
	Tmpl *template.Template
}

func (h *Handler) RenderLogin(w http.ResponseWriter, r *http.Request) {
	if _, ok := h.GetAccessToken(w, r); ok {
		http.Redirect(w, r, "/bookings", http.StatusFound)
		return
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	if err := h.Tmpl.ExecuteTemplate(w, "login.html", nil); err != nil {
		http.Error(w, "Intern feil", http.StatusInternalServerError)
	}
}

func (h *Handler) HandleAuthStart(w http.ResponseWriter, r *http.Request) {
	verifier, err := GenerateRandom(32)
	if err != nil {
		http.Error(w, "Intern feil", http.StatusInternalServerError)
		return
	}
	state, err := GenerateRandom(16)
	if err != nil {
		http.Error(w, "Intern feil", http.StatusInternalServerError)
		return
	}

	signed, err := SignPKCE(verifier, state, h.Cfg.SecretKey)
	if err != nil {
		http.Error(w, "Intern feil", http.StatusInternalServerError)
		return
	}

	http.SetCookie(w, &http.Cookie{
		Name:     CookiePKCE,
		Value:    signed,
		Path:     "/callback",
		HttpOnly: true,
		Secure:   h.Cfg.Secure,
		SameSite: http.SameSiteLaxMode,
		MaxAge:   600,
	})

	params := url.Values{
		"response_type":         {"code"},
		"client_id":             {h.Cfg.ClientID},
		"redirect_uri":          {h.Cfg.AppURL + "/callback"},
		"scope":                 {"openid offline_access"},
		"state":                 {state},
		"code_challenge":        {CodeChallenge(verifier)},
		"code_challenge_method": {"S256"},
	}
	http.Redirect(w, r, h.Cfg.FusionAuthURL+"/oauth2/authorize?"+params.Encode(), http.StatusFound)
}

func (h *Handler) HandleCallback(w http.ResponseWriter, r *http.Request) {
	c, err := r.Cookie(CookiePKCE)
	if err != nil {
		http.Redirect(w, r, "/login", http.StatusFound)
		return
	}

	verifier, expectedState, err := VerifyPKCE(c.Value, h.Cfg.SecretKey)
	if err != nil {
		http.Redirect(w, r, "/login", http.StatusFound)
		return
	}

	if r.URL.Query().Get("state") != expectedState {
		http.Redirect(w, r, "/login", http.StatusFound)
		return
	}

	code := r.URL.Query().Get("code")
	if code == "" {
		http.Redirect(w, r, "/login", http.StatusFound)
		return
	}

	accessToken, refreshToken, err := h.exchangeCode(code, verifier)
	if err != nil {
		http.Error(w, fmt.Sprintf("Autentisering feilet: %v", err), http.StatusBadGateway)
		return
	}

	http.SetCookie(w, &http.Cookie{
		Name:   CookiePKCE,
		Value:  "",
		Path:   "/callback",
		MaxAge: -1,
	})

	h.SetTokenCookies(w, accessToken, refreshToken)
	http.Redirect(w, r, "/", http.StatusFound)
}

func (h *Handler) HandleLogout(w http.ResponseWriter, r *http.Request) {
	ClearCookie(w, CookieAccess, "/", h.Cfg.Secure)
	ClearCookie(w, CookieRefresh, "/", h.Cfg.Secure)
	http.Redirect(w, r, "/login", http.StatusFound)
}

func (h *Handler) exchangeCode(code, verifier string) (accessToken, refreshToken string, err error) {
	params := url.Values{
		"grant_type":    {"authorization_code"},
		"client_id":     {h.Cfg.ClientID},
		"code":          {code},
		"redirect_uri":  {h.Cfg.AppURL + "/callback"},
		"code_verifier": {verifier},
		"scope":         {"openid offline_access"},
	}
	resp, err := http.PostForm(h.Cfg.FusionAuthInternalURL+"/oauth2/token", params)
	if err != nil {
		return "", "", err
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return "", "", fmt.Errorf("FusionAuth svarte med %d", resp.StatusCode)
	}

	var result struct {
		AccessToken  string `json:"access_token"`
		RefreshToken string `json:"refresh_token"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return "", "", err
	}
	if result.AccessToken == "" || result.RefreshToken == "" {
		return "", "", fmt.Errorf("manglende tokens i svar fra FusionAuth")
	}
	return result.AccessToken, result.RefreshToken, nil
}

func (h *Handler) doRefresh(refreshToken string) (accessToken, newRefresh string, err error) {
	params := url.Values{
		"grant_type":    {"refresh_token"},
		"client_id":     {h.Cfg.ClientID},
		"refresh_token": {refreshToken},
	}
	resp, err := http.PostForm(h.Cfg.FusionAuthInternalURL+"/oauth2/token", params)
	if err != nil {
		return "", "", err
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return "", "", fmt.Errorf("refresh feilet med %d", resp.StatusCode)
	}

	var result struct {
		AccessToken  string `json:"access_token"`
		RefreshToken string `json:"refresh_token"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return "", "", err
	}
	if result.AccessToken == "" {
		return "", "", fmt.Errorf("manglende access_token i refresh-svar")
	}
	return result.AccessToken, result.RefreshToken, nil
}

// GetAccessToken retrieves a valid access token from the request cookies,
// refreshing silently if the access token is missing or near expiry.
// Returns ("", false) if authentication is not possible — caller must redirect to login.
func (h *Handler) GetAccessToken(w http.ResponseWriter, r *http.Request) (string, bool) {
	if c, err := r.Cookie(CookieAccess); err == nil {
		if token, err := DecryptToken(c.Value, h.Cfg.SecretKey); err == nil {
			if exp, err := JWTExpiry(token); err == nil && time.Until(exp) > 30*time.Second {
				return token, true
			}
		}
	}

	rc, err := r.Cookie(CookieRefresh)
	if err != nil {
		return "", false
	}

	newAccess, newRefresh, err := h.doRefresh(rc.Value)
	if err != nil {
		ClearCookie(w, CookieAccess, "/", h.Cfg.Secure)
		ClearCookie(w, CookieRefresh, "/", h.Cfg.Secure)
		return "", false
	}

	h.SetTokenCookies(w, newAccess, newRefresh)
	return newAccess, true
}

func (h *Handler) SetTokenCookies(w http.ResponseWriter, accessToken, refreshToken string) {
	if enc, err := EncryptToken(accessToken, h.Cfg.SecretKey); err == nil {
		http.SetCookie(w, &http.Cookie{
			Name:     CookieAccess,
			Value:    enc,
			Path:     "/",
			HttpOnly: true,
			Secure:   h.Cfg.Secure,
			SameSite: http.SameSiteLaxMode,
			MaxAge:   3600,
		})
	}
	if refreshToken != "" {
		http.SetCookie(w, &http.Cookie{
			Name:     CookieRefresh,
			Value:    refreshToken,
			Path:     "/",
			HttpOnly: true,
			Secure:   h.Cfg.Secure,
			SameSite: http.SameSiteLaxMode,
			MaxAge:   60 * 60 * 24 * 30,
		})
	}
}

func ClearCookie(w http.ResponseWriter, name, path string, secure bool) {
	http.SetCookie(w, &http.Cookie{
		Name:     name,
		Value:    "",
		Path:     path,
		HttpOnly: true,
		Secure:   secure,
		SameSite: http.SameSiteLaxMode,
		MaxAge:   -1,
	})
}

// RedirectToLogin sends a login redirect. For Unpoly fragment requests it returns
// a 401 so the browser can handle the redirect client-side via up-fail-target.
func RedirectToLogin(w http.ResponseWriter, r *http.Request) {
	if r.Header.Get("X-Up-Version") != "" {
		w.Header().Set("X-Up-Location", "/login")
		w.WriteHeader(http.StatusUnauthorized)
		return
	}
	http.Redirect(w, r, "/login", http.StatusFound)
}
