package models

import (
	"../auth"
	"../conf"
	// "crypto/sha1"
	_ "database/sql"
	// "encoding/json"
	// "fmt"
	"github.com/golang/glog"
	"github.com/julienschmidt/httprouter"
	"net/http"
	// "reflect"
	// "os"
	// "os/exec"
	// "path/filepath"
	// "os"
	"html"
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

// func (th ModelHandler) GET_user_ID_type_TYPE_page_PAGE(w http.ResponseWriter, r *http.Request, ps httprouter.Params) {
// 	uid, err := strconv.Atoi(ps.ByName("id"))
// 	page, err2 := strconv.Atoi(ps.ByName("page"))
// 	u := auth.GetUser(r)

// 	if err != nil || u.Name == "" || err2 != nil {
// 		ServePage(w, "404", nil)
// 		return
// 	}
// 	var payload struct {
// 		Articles []AuthUserArticle
// 		Tags     map[int]string
// 		IsAdmin  bool
// 		IsTrash  bool
// 	}

// 	_app := 1000
// 	_start := _app * (page - 1)

// 	ret := make([]AuthUserArticle, 0)

// 	if u.ID != uid && !conf.GlobalServerConfig.GetPrivilege(u.Group, "ViewOtherTrash") {
// 		ServePage(w, "404", nil)
// 		return
// 	}

// 	viewType := strconv.FormatBool(ps.ByName("type") == "trash")
// 	payload.IsTrash = ps.ByName("type") == "trash"

// 	rows, err := auth.Gdb.Query(`select
// 		id, title, tag, created_at, deleted
// 		from articles
// 		where author=` + strconv.Itoa(uid) +
// 		` and deleted=` + viewType + ` order by id desc
// 		offset ` + strconv.Itoa(_start) + " limit " + strconv.Itoa(_app))

// 	if err != nil {
// 		log.Println(err)
// 		ServePage(w, "404", nil)
// 		return
// 	}

// 	defer rows.Close()

// 	for rows.Next() {
// 		var id, tag int
// 		var title string
// 		var d time.Time
// 		var deleted bool

// 		rows.Scan(&id, &title, &tag, &d, &deleted)
// 		// title = html.UnescapeString(title)

// 		ret = append(ret, AuthUserArticle{id, title,
// 			conf.GlobalServerConfig.GetIndexTag(tag),
// 			int(d.Unix()), deleted})
// 	}

// 	payload.Articles = ret
// 	payload.Tags = conf.GlobalServerConfig.GetTags()

// 	ServePage(w, "list", payload)
// }

// func (th ModelHandler) GET_global_TYPE_page_PAGE(w http.ResponseWriter, r *http.Request, ps httprouter.Params) {
// 	u := auth.GetUser(r)
// 	page, err := strconv.Atoi(ps.ByName("page"))

// 	if !conf.GlobalServerConfig.GetPrivilege(u.Group, "ViewGlobalTrash") || err != nil {
// 		ServePage(w, "404", nil)
// 		return
// 	}

// 	var payload struct {
// 		Articles []AuthUserArticle
// 		Tags     map[int]string
// 		IsAdmin  bool
// 		IsTrash  bool
// 	}

// 	payload.Tags = conf.GlobalServerConfig.GetTags()

// 	_app := 1000
// 	_start := _app * (page - 1)

// 	// payload.DeletedArticles = auth.GetDeletedArticles(u)
// 	payload.Articles = make([]AuthUserArticle, 0)
// 	payload.IsTrash = ps.ByName("type") == "trash"

// 	rows2, err := auth.Gdb.Query(`select
// 		id, title, tag, created_at, deleted
// 		from articles
// 		where deleted=` + strconv.FormatBool(payload.IsTrash) + `
// 		order by id desc
// 		offset ` + strconv.Itoa(_start) + " limit " + strconv.Itoa(_app))

// 	if err == nil {
// 		defer rows2.Close()

// 		for rows2.Next() {
// 			var id, tag int
// 			var title string
// 			var d time.Time
// 			var deleted bool

// 			rows2.Scan(&id, &title, &tag, &d, &deleted)
// 			payload.Articles = append(payload.Articles,
// 				AuthUserArticle{
// 					id, title,
// 					conf.GlobalServerConfig.GetIndexTag(tag),
// 					int(d.Unix()),
// 					deleted,
// 				})
// 		}
// 	}

// 	payload.IsAdmin = true

// 	ServePage(w, "list", payload)
// }

// PAGE: List all articles posted by user id
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
			false,
			comment,
			avatar,
			usage,
			""}
	} else {
		glog.Errorln("Database:", err)
	}
	// }

	payload.Tags = conf.GlobalServerConfig.GetTags()
	// payload.TotalArticles = auth.GetArticlesCount(ps.ByName("id"), "ua")

	ServePage(w, "user", payload)
}

func (th ModelHandler) POST_user_update_comment(w http.ResponseWriter, r *http.Request, ps httprouter.Params) {
	u := auth.GetUser(r)
	if u.Name == "" {
		w.Write([]byte("Err::Privil::Invalid_User"))
	}

	c := r.FormValue("comment")
	if len(c) > 512 {
		w.Write([]byte("Err::Post::Comment_Too_Long"))
		return
	}

	comment := html.EscapeString(c)

	_, err := auth.Gdb.Exec(`UPDATE user_info SET comment = '` + comment + `' WHERE id = ` + strconv.Itoa(u.ID))
	if err == nil {
		w.Write([]byte("ok"))
	} else {
		glog.Errorln("Database:", err)
		w.Write([]byte("Err::DB::General_Failure"))
	}
}

// PAGE: Serve user login page and user account panel page
func (th ModelHandler) GET_account(w http.ResponseWriter, r *http.Request, ps httprouter.Params) {
	type up struct {
		Name  string
		Value bool
	}
	var payload struct {
		auth.AuthUser
		UserPrivilege []up
		IsLoggedIn    bool
	}

	payload.AuthUser = auth.GetUser(r)
	payload.IsLoggedIn = payload.AuthUser.Name != ""
	payload.UserPrivilege = make([]up, 0)

	if payload.AuthUser.Group == "admin" {
		payload.UserPrivilege = append(payload.UserPrivilege, up{"Admin", true})
	} else {
		if g, e := conf.GlobalServerConfig.Privilege[payload.AuthUser.Group]; !e {
			payload.UserPrivilege = append(payload.UserPrivilege, up{"None", true})
		} else {
			for k, v := range g.(map[string]interface{}) {
				_v, err := v.(bool)
				if err {
					payload.UserPrivilege = append(payload.UserPrivilege, up{k, _v})
				}
			}
		}
	}
	payload.UserPrivilege = append(payload.UserPrivilege, up{
		"Cooldown:" + strconv.Itoa(conf.GlobalServerConfig.GetInt(payload.AuthUser.Group, "Cooldown")),
		true,
	})
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
