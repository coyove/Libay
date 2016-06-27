package models

import (
	"../auth"

	"github.com/julienschmidt/httprouter"

	"net/http"
)

func (th ModelHandler) POST_login_phase2(w http.ResponseWriter, r *http.Request, ps httprouter.Params) {
	Return(w, auth.ServeLoginPhase2(w, r))
}

func (th ModelHandler) POST_login_phase1(w http.ResponseWriter, r *http.Request, ps httprouter.Params) {
	Return(w, auth.ServeLoginPhase1(w, r))
}

func (th ModelHandler) POST_register(w http.ResponseWriter, r *http.Request, ps httprouter.Params) {
	Return(w, auth.ServeRegister(w, r))
}

func (th ModelHandler) POST_logout(w http.ResponseWriter, r *http.Request, ps httprouter.Params) {
	Return(w, auth.ServeLogout(w, r))
}
