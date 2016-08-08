package models

import (
	"../auth"
	"../conf"

	"github.com/golang/glog"
	"github.com/julienschmidt/httprouter"

	_ "database/sql"
	// "html"
	"net/http"
	"strconv"
	// "strings"
	"time"
)

type AuthUserArticle struct {
	ID        int
	Title     string
	Tag       string
	Timestamp int
	Deleted   bool
}

func (th ModelHandler) GET_user_ID(w http.ResponseWriter, r *http.Request, ps httprouter.Params) {
	id, err := strconv.Atoi(ps.ByName("id"))
	if err != nil || id <= 0 {
		ServePage(w, "404", nil)
		return
	}

	var payload struct {
		User          auth.AuthUser
		TotalArticles int
		Tags          map[int]string
	}

	payload.User = auth.GetUserByID(id)
	if payload.User.ID == 0 {
		ServePage(w, "404", nil)
		return
	}
	payload.Tags = conf.GlobalServerConfig.GetTags()
	ServePage(w, "user", payload)
}

func (th ModelHandler) POST_user_update_comment(w http.ResponseWriter, r *http.Request, ps httprouter.Params) {
	u := auth.GetUser(r)
	if u.Name == "" {
		Return(w, "Err::Privil::Invalid_User")
	}

	c := r.FormValue("comment")
	if len(c) > 512 {
		Return(w, "Err::Post::Comment_Too_Long")
		return
	}

	comment := auth.Escape(c)

	_, err := auth.Gdb.Exec(`UPDATE user_info SET comment = '` + comment + `' WHERE id = ` + strconv.Itoa(u.ID))
	if err == nil {
		w.Write([]byte("ok"))
		auth.Guser.Remove(u.ID)
	} else {
		glog.Errorln("Database:", err)
		Return(w, "Err::DB::General_Failure")
	}
}

func (th ModelHandler) POST_unread_message_ID(w http.ResponseWriter, r *http.Request, ps httprouter.Params) {
	u := auth.GetUser(r)
	if u.Name == "" {
		Return(w, "Err::Privil::Invalid_User")
	}

	id, err := strconv.Atoi(ps.ByName("id"))
	if err != nil {
		Return(w, "Err::Router::Invalid_Article_Id")
		return
	}

	if id == 0 {
		messageLimit := int(time.Now().UnixNano()/1e6 - 3600000*24*365)
		_, err = auth.Gdb.Exec(`
        UPDATE articles 
        SET    read = true
        WHERE 
            tag = ` + strconv.Itoa(u.ID+100000) + `
        AND created_at > ` + strconv.Itoa(messageLimit))
	} else {
		_, err = auth.Gdb.Exec(`
        UPDATE articles 
        SET    read = false 
        WHERE 
            id = ` + strconv.Itoa(id) + ` 
        AND 
            tag = ` + strconv.Itoa(u.ID+100000))
	}
	if err == nil {
		auth.Gcache.Remove(`.+-` + strconv.Itoa(id) + `-(true|false)`)
		w.Write([]byte("ok"))
	} else {
		glog.Errorln("Database:", err)
		Return(w, "Err::DB::General_Failure")
	}
}

// PAGE: Serve user login page and user account panel page
func (th ModelHandler) GET_account(w http.ResponseWriter, r *http.Request, ps httprouter.Params) {
	var payload struct {
		auth.AuthUser
		UserPrivilege map[string]interface{}
		IsLoggedIn    bool
	}

	payload.AuthUser = auth.GetUser(r, w)
	payload.IsLoggedIn = payload.AuthUser.Name != ""
	payload.UserPrivilege = make(map[string]interface{})

	if payload.AuthUser.Group == "admin" {
		payload.UserPrivilege["Admin"] = true
	} else {
		if g, e := conf.GlobalServerConfig.Privilege[payload.AuthUser.Group]; !e {
		} else {
			for k, v := range g.(map[string]interface{}) {
				if _v, ok := v.(bool); ok {
					payload.UserPrivilege[k] = _v
				}
			}
		}
	}

	payload.UserPrivilege["Cooldown"] = conf.GlobalServerConfig.GetInt(payload.AuthUser.Group, "Cooldown")
	ServePage(w, "account", payload)
}

// PAGE: Serve new user register page
func (th ModelHandler) GET_account_register(w http.ResponseWriter, r *http.Request, ps httprouter.Params) {
	var payload struct {
		IsOpen bool
	}
	payload.IsOpen = conf.GlobalServerConfig.AllowRegistration
	ServePage(w, "register", payload)
}
