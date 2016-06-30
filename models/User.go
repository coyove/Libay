package models

import (
	"../auth"
	"../conf"

	"github.com/golang/glog"
	"github.com/julienschmidt/httprouter"

	_ "database/sql"
	"html"
	"net/http"
	"strconv"
	"strings"
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
	if err != nil {
		ServePage(w, "404", nil)
		return
	}

	var payload struct {
		User          auth.AuthUser
		TotalArticles int
		Tags          map[int]string
	}

	var nickname string
	var date, signupDate time.Time
	var status, group, comment, avatar string
	var usage int

	err = auth.Gdb.QueryRow(`
        SELECT
                users.nickname,
                users.last_login_date,
                users.signup_date,
            user_info.status,
            user_info.group,
            user_info.comment,
            user_info.avatar,
            user_info.image_usage 
        FROM
            users 
        INNER JOIN 
            user_info ON user_info.id = users.id
        WHERE
            users.id = `+strconv.Itoa(id)).
		Scan(&nickname,
		&date,
		&signupDate,
		&status,
		&group,
		&comment,
		&avatar,
		&usage)

	if err == nil {
		comment = html.UnescapeString(comment)
		payload.User = auth.AuthUser{id,
			"",
			nickname,
			int(date.Unix()),
			int(signupDate.Unix()),
			"",
			strings.Trim(status, " "),
			strings.Trim(group, " "),
			comment,
			avatar,
			usage,
			""}
	} else {
		glog.Errorln("Database:", err)
		ServePage(w, "404", nil)
		return
	}
	// }

	payload.Tags = conf.GlobalServerConfig.GetTags()
	// payload.TotalArticles = auth.GetArticlesCount(ps.ByName("id"), "ua")

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
		UserPrivilege map[string]bool
		IsLoggedIn    bool
	}

	payload.AuthUser = auth.GetUser(r)
	payload.IsLoggedIn = payload.AuthUser.Name != ""
	payload.UserPrivilege = make(map[string]bool)

	if payload.AuthUser.Group == "admin" {
		payload.UserPrivilege["Admin"] = true
	} else {
		if g, e := conf.GlobalServerConfig.Privilege[payload.AuthUser.Group]; !e {
			payload.UserPrivilege["None"] = true
		} else {
			for k, v := range g.(map[string]interface{}) {
				if _v, ok := v.(bool); ok {
					payload.UserPrivilege[k] = _v
				}
			}
		}
	}

	payload.UserPrivilege["Cooldown:"+
		strconv.Itoa(conf.GlobalServerConfig.GetInt(payload.AuthUser.Group, "Cooldown"))] = true
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
