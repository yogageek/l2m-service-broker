// #SM1-007

package middleware

import (
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"strings"

	"github.com/golang/glog"
	osb "github.com/pmorie/go-open-service-broker-client/v2"
)

var (
	Unauthorized   = "Unauthorized"
	mismatchHeader = "mismatch header information"

	authusername = os.Getenv("API_USERNAME")
	authpassword = os.Getenv("API_PASSWORD")
)

func BasicAuth(h http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {

		if r.RequestURI == "/healthz" {
			h.ServeHTTP(w, r)
			return
		}

		if statusCode, ok := headerVerifier(r); !ok {
			desc := fmt.Sprintf("%+v", r.Header)
			err := osb.HTTPStatusCodeError{
				StatusCode:   statusCode,
				ErrorMessage: &mismatchHeader,
				Description:  &desc,
			}
			writeOSBStatusCodeErrorResponse(w, err)
			glog.Error(err)
			return
		}

		glog.V(2).Info("checking auth...")
		// v2
		// DEPRECATED : moved to basic auth user and pass
		// auth := strings.TrimSpace(r.Header.Get("Authorization"))
		// if auth == "" {
		// 	desc := "token is null"
		// 	output := osb.HTTPStatusCodeError{
		// 		StatusCode:   http.StatusUnauthorized,
		// 		ErrorMessage: &Unauthorized,
		// 		Description:  &desc,
		// 	}
		// 	writeOSBStatusCodeErrorResponse(w, output)
		// 	glog.Error(output)
		// 	return
		// }

		var rUsername, rPassword string
		var ok bool
		if rUsername, rPassword, ok = r.BasicAuth(); !ok {
			unauthorizedResponse(w)
			return
		}

		// reset bool on continue
		ok = false

		// v2
		// DEPRECATED : moved to basic auth user and pass
		// token := r.Header.Get("Authorization")
		// s := strings.SplitN(token, " ", 2)
		// if len(s) != 2 {
		// 	output := osb.HTTPStatusCodeError{
		// 		StatusCode:   http.StatusUnauthorized,
		// 		ErrorMessage: &Unauthorized,
		// 	}
		// 	writeOSBStatusCodeErrorResponse(w, output)
		// 	glog.Error(output)
		// 	return
		// }

		var username, password string
		if username, password, ok = getAuthz(); !ok {
			unauthorizedResponse(w)
			return
		}

		if !authReview(username, password, rUsername, rPassword) {
			glog.Infof("%s; %s; %s; %s", username, password, rUsername, rPassword)
			unauthorizedResponse(w)
			return
		}

		h.ServeHTTP(w, r)
	})
}

//modify
func headerVerifier(c *http.Request) (int, bool) {
	if c.Header == nil {
		return http.StatusUnauthorized, false
	}
	if c.Method == "GET" {
		return 0, true
	}
	if c.Header.Get("Content-Type") != "application/json" {
		return http.StatusUnsupportedMediaType, false
	}
	return 0, true
}

func writeOSBStatusCodeErrorResponse(w http.ResponseWriter, osbErr osb.HTTPStatusCodeError) {
	_, err := json.Marshal(osbErr.ErrorMessage)
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		return
	}
	w.Header().Set("WWW-Authenticate", `Basic realm="restricted`)

	w.WriteHeader(osbErr.StatusCode)
}

func getAuthz() (username, password string, ok bool) {
	username, password = authusername, authpassword
	if username == "" && password == "" {
		glog.Error("API username & password is empty")
		return username, password, false
	}
	return username, password, true
}

func authReview(rUsername, rPassword, authusername, authpassword string) bool {
	if strings.Compare(rUsername, authusername) == 0 && strings.Compare(rPassword, authpassword) == 0 {
		return true
	}
	return false
}

func unauthorizedResponse(w http.ResponseWriter) {
	output := osb.HTTPStatusCodeError{
		StatusCode:   http.StatusUnauthorized,
		ErrorMessage: &Unauthorized,
	}
	writeOSBStatusCodeErrorResponse(w, output)
	glog.Error(output)
	return
}
