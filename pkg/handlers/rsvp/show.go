package rsvp

import (
	"encoding/base64"
	"net/http"
	"net/url"

	"github.com/skip2/go-qrcode"
	"github.com/temirov/RSVP/models"
	"github.com/temirov/RSVP/pkg/config"
	"github.com/temirov/RSVP/pkg/handlers"
	"github.com/temirov/RSVP/pkg/utils"
)

// ShowHandler handles GET /rsvps/?rsvp_id=ABC => protected printing page
// with QR pointing to GET /response/?event_id=XXX (unprotected).
func ShowHandler(appCtx *config.ApplicationContext) http.HandlerFunc {
	baseHandler := handlers.NewBaseHandler(appCtx, "RSVP", config.WebRSVPs)

	return func(w http.ResponseWriter, r *http.Request) {
		rsvpID := baseHandler.GetParam(r, config.RSVPIDParam)
		if rsvpID == "" {
			http.Error(w, "Missing rsvp_id", http.StatusBadRequest)
			return
		}
		if !baseHandler.ValidateMethod(w, r, http.MethodGet) {
			return
		}

		// Must be logged in
		sessionData, isAuth := baseHandler.RequireAuthentication(w, r)
		if !isAuth {
			return
		}

		var currentUser models.User
		if errUsr := currentUser.FindByEmail(appCtx.Database, sessionData.UserEmail); errUsr != nil {
			baseHandler.HandleError(w, errUsr, utils.DatabaseError, "User not found")
			return
		}

		var rsvpRec models.RSVP
		if errLoad := rsvpRec.FindByCode(appCtx.Database, rsvpID); errLoad != nil {
			baseHandler.HandleError(w, errLoad, utils.NotFoundError, "RSVP not found")
			return
		}

		var eventRec models.Event
		if errEvt := eventRec.FindByID(appCtx.Database, rsvpRec.EventID); errEvt != nil {
			baseHandler.HandleError(w, errEvt, utils.NotFoundError, "Event not found")
			return
		}
		if eventRec.UserID != currentUser.ID {
			http.Error(w, "Forbidden", http.StatusForbidden)
			return
		}

		// The QR points to GET /response/?event_id=XYZ
		publicURL := url.URL{
			Scheme: "http",
			Host:   r.Host,
			Path:   "/response/",
		}
		if r.TLS != nil {
			publicURL.Scheme = "https"
		}
		q := publicURL.Query()
		q.Set(config.EventIDParam, eventRec.ID)
		publicURL.RawQuery = q.Encode()
		finalLink := publicURL.String()

		codeBytes, errQR := qrcode.Encode(finalLink, qrcode.Medium, 256)
		if errQR != nil {
			baseHandler.HandleError(w, errQR, utils.ServerError, "Failed generating QR code")
			return
		}
		codeBase64 := base64.StdEncoding.EncodeToString(codeBytes)

		data := struct {
			RSVP        models.RSVP
			Event       models.Event
			QRCode      string
			PublicURL   string
			UserName    string
			UserPicture string
		}{
			RSVP:        rsvpRec,
			Event:       eventRec,
			QRCode:      codeBase64,
			PublicURL:   finalLink,
			UserName:    sessionData.UserName,
			UserPicture: sessionData.UserPicture,
		}

		renderErr := appCtx.Templates.ExecuteTemplate(w, "rsvp.html", data)
		if renderErr != nil {
			baseHandler.HandleError(w, renderErr, utils.ServerError, "Failed rendering rsvp.html")
		}
	}
}
