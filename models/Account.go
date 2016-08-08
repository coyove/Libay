package models

import (
	"../auth"

	"github.com/julienschmidt/httprouter"

	"net/http"
)

func (th ModelHandler) POST_login(w http.ResponseWriter, r *http.Request, ps httprouter.Params) {
	Return(w, auth.ServeLogin(w, r))
}

func (th ModelHandler) POST_register(w http.ResponseWriter, r *http.Request, ps httprouter.Params) {
	Return(w, auth.ServeRegister(w, r))
}

func (th ModelHandler) POST_logout(w http.ResponseWriter, r *http.Request, ps httprouter.Params) {
	Return(w, auth.ServeLogout(w, r))
}
