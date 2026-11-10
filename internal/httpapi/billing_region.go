package httpapi

import (
	"net/http"
	"time"

	"github.com/stewartmckee/canvasbin/internal/geo"
)

func (s *Server) billingRegion(r *http.Request) geo.Region {
	return geo.FromRequest(r)
}

func (s *Server) stickBillingRegion(w http.ResponseWriter, r *http.Request, reg geo.Region) {
	http.SetCookie(w, &http.Cookie{
		Name:     geo.CookieName(),
		Value:    string(reg),
		Path:     "/",
		MaxAge:   int((365 * 24 * time.Hour).Seconds()),
		HttpOnly: false,
		SameSite: http.SameSiteLaxMode,
		Secure:   s.secureCookies(r),
	})
}
