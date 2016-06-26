package models

import (
	"../auth"
	"../conf"

	"github.com/golang/glog"
	"github.com/julienschmidt/httprouter"

	_ "database/sql"
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"
)

func (th ModelHandler) GET_article_ID(w http.ResponseWriter, r *http.Request, ps httprouter.Params) {
	id, err := strconv.Atoi(ps.ByName("id"))
	if err != nil {
		ServePage(w, "404", nil)
		return
	}

	var payload struct {
		Article    auth.Article
		AuthorSelf bool

		CanMakeTop    bool
		CanMakeLocked bool

		User       auth.AuthUser
		IsLoggedIn bool
		IsMessage  bool
	}
	u := auth.GetUser(r)
	vtt := conf.GlobalServerConfig.GetPrivilege(u.Group, "ViewOtherTrash")

	payload.Article = auth.GetArticle(r, u, id, false)
	payload.AuthorSelf = (u.ID == payload.Article.AuthorID || vtt)

	payload.CanMakeTop = conf.GlobalServerConfig.GetPrivilege(u.Group, "AnnounceArticle")
	payload.CanMakeLocked = conf.GlobalServerConfig.GetPrivilege(u.Group, "MakeLocked")

	payload.User = u
	payload.IsLoggedIn = u.Name != ""

	if payload.Article.Deleted {
		if u.ID != payload.Article.AuthorID && !vtt {
			ServePage(w, "404", nil)
			return
		}

		if u.ID == 0 && !vtt {
			ServePage(w, "404", nil)
			return
		}
	}

	ServePage(w, "article", payload)
}

func (th ModelHandler) GET_article_ID_history(w http.ResponseWriter, r *http.Request, ps httprouter.Params) {
	id, err := strconv.Atoi(ps.ByName("id"))
	// u := auth.GetUser(r)

	if err != nil {
		w.WriteHeader(503)
		return
	}

	type pair struct {
		Date time.Time
		User string
	}
	ret := make(map[string]pair)

	rows, err := auth.Gdb.Query(`
        SELECT
            history.id, 
            history.date,
              users.nickname
        FROM 
            history 
        INNER JOIN 
            users ON users.id = history.user_id
        WHERE 
            article_id = ` + strconv.Itoa(id))

	if err != nil {
		glog.Errorln("Database:", err)
		w.WriteHeader(503)
		return
	}

	defer rows.Close()

	for rows.Next() {
		var id int
		var username string
		var t time.Time

		rows.Scan(&id, &t, &username)

		ret[strconv.Itoa(id)] = pair{t, username}
	}

	buf, _ := json.Marshal(ret)
	w.Write(buf)
}

func (th ModelHandler) GET_article_ID_history_HID(w http.ResponseWriter, r *http.Request, ps httprouter.Params) {
	aid, err := strconv.Atoi(ps.ByName("id"))
	hid, err2 := strconv.Atoi(ps.ByName("hid"))
	if err != nil || err2 != nil {
		w.WriteHeader(503)
		return
	}

	u := auth.GetUser(r)

	var d bool
	var uid int
	if auth.Gdb.QueryRow("SELECT deleted, author FROM articles WHERE id = "+strconv.Itoa(aid)).Scan(&d, &uid) != nil {
		w.WriteHeader(503)
		return
	}

	if d {
		if uid != u.ID || !conf.GlobalServerConfig.GetPrivilege(u.Group, "EditOthers") {
			w.WriteHeader(503)
			return
		}
	}

	var c, t string
	if auth.Gdb.QueryRow("SELECT title, content FROM history WHERE id = "+strconv.Itoa(hid)).Scan(&t, &c) != nil {
		w.WriteHeader(503)
	} else {
		w.Write([]byte("{\"Title\":\"" + t + "\", \"Content\": \"" + c + "\"}"))
	}
}

func (th ModelHandler) POST_delete_article_ID_ACTION(w http.ResponseWriter, r *http.Request, ps httprouter.Params) {
	if !auth.LogIP(r) {
		w.Write([]byte("Err::Router::Frequent_Access"))
		return
	}

	u := auth.GetUser(r)
	if !u.CanPost() {
		w.Write([]byte("Err::Privil::Post_Action_Denied"))
		return
	}

	if !auth.CheckCSRF(r) {
		w.Write([]byte("Err::CSRF::CSRF_Failure"))
		return
	}

	id, err := strconv.Atoi(ps.ByName("id"))
	if err != nil {
		w.Write([]byte("Err::Router::Invalid_Article_Id"))
		return
	}

	var authorID, tag int
	var locked bool

	if auth.Gdb.QueryRow("SELECT author, tag, locked FROM articles WHERE id = "+strconv.Itoa(id)).
		Scan(&authorID, &tag, &locked) != nil {
		w.Write([]byte("Err::DB::Select_Failure"))
		return
	}

	if authorID != u.ID && u.Group != "admin" {
		if conf.GlobalServerConfig.GetPrivilege(u.Group, "DeleteOthers") {
			// User with "DeleteOthers" privilege can delete others' articles
		} else {
			if tag > 100000 && u.ID == tag-100000 {
				// Both the receiver and the sender can delete the message
			} else {
				w.Write([]byte("Err::Privil::Delete_Restore_Action_Denied"))
				return
			}
		}
	}

	if locked && !conf.GlobalServerConfig.GetPrivilege(u.Group, "MakeLocked") {
		w.Write([]byte("Err::Post::Locked_Article"))
		return
	}

	_, err = auth.Gdb.Exec("UPDATE articles SET deleted = NOT deleted WHERE id = " + strconv.Itoa(id))

	if err == nil {
		auth.Gcache.Clear()
		w.Write([]byte("ok"))
	} else {
		glog.Errorln("Database:", err)
		w.Write([]byte("Err::DB::Update_Failure"))
	}
}

func (th ModelHandler) POST_lock_article_ID(w http.ResponseWriter, r *http.Request, ps httprouter.Params) {
	u := auth.GetUser(r)
	if !u.CanPost() {
		w.Write([]byte("Err::Privil::Post_Action_Denied"))
		return
	}

	if !auth.CheckCSRF(r) {
		w.Write([]byte("Err::CSRF::CSRF_Failure"))
		return
	}

	id, err := strconv.Atoi(ps.ByName("id"))
	if err != nil {
		w.Write([]byte("Err::Router::Invalid_Article_Id"))
		return
	}

	// User with "MakeLocked" privilege can (un)lock articles
	if u.Group != "admin" && !conf.GlobalServerConfig.GetPrivilege(u.Group, "MakeLocked") {
		w.Write([]byte("Err::Privil::Lock_Action_Denied"))
		return
	}

	_, err = auth.Gdb.Exec("UPDATE articles SET locked = NOT locked WHERE id = " + strconv.Itoa(id))

	if err == nil {
		auth.Gcache.Clear()
		w.Write([]byte("ok"))
	} else {
		glog.Errorln("Database:", err)
		w.Write([]byte("Err::DB::Update_Failure"))
	}
}

func (th ModelHandler) POST_top_article_ID(w http.ResponseWriter, r *http.Request, ps httprouter.Params) {
	u := auth.GetUser(r)

	id, err := strconv.Atoi(ps.ByName("id"))
	if err != nil {
		w.Write([]byte("Err::Router::Invalid_Article_Id"))
		return
	}

	if !auth.CheckCSRF(r) {
		w.Write([]byte("Err::CSRF::CSRF_Failure"))
		return
	}

	if !conf.GlobalServerConfig.GetPrivilege(u.Group, "AnnounceArticle") {
		w.Write([]byte("Err::Privil::Announce_Action_Denied"))
		return
	}

	row, err := auth.Gdb.Query("UPDATE articles SET stay_top = NOT stay_top WHERE id = " + strconv.Itoa(id))

	if err == nil {
		row.Close()
		auth.Gcache.Clear()
		w.Write([]byte("ok"))
	} else {
		glog.Errorln("Database:", err)
		w.Write([]byte("Err::DB::Update_Failure"))
	}
}

func (th ModelHandler) POST_post_ID(w http.ResponseWriter, r *http.Request, ps httprouter.Params) {
	if !auth.LogIP(r) {
		w.Write([]byte("Err::Router::Frequent_Access"))
		return
	}

	u := auth.GetUser(r)
	if !u.CanPost() {
		w.Write([]byte("Err::Privil::Post_Action_Denied"))
		return
	}

	if !auth.CheckCSRF(r) {
		w.Write([]byte("Err::CSRF::CSRF_Failure"))
		return
	}

	id, err := strconv.Atoi(ps.ByName("id"))
	if err != nil {
		w.Write([]byte("Err::Router::Invalid_Article_Id"))
		return
	}

	content := r.FormValue("content")
	if len(content) > conf.GlobalServerConfig.MaxArticleContentLength*1024 {
		ex := len(content) - conf.GlobalServerConfig.MaxArticleContentLength*1024
		w.Write([]byte(fmt.Sprintf("Err::Post::Content_Too_Long_%d_KiB_Exceeded", ex/1024)))
		return
	}

	title := r.FormValue("title")
	if len(title) > 512 {
		title = title[:512]
	} else if len(title) < 3 {
		w.Write([]byte("Err::Post::Title_Too_Short"))
		return
	}

	tag := r.FormValue("tag")
	if len(tag) > 128 {
		tag = tag[:128]
	}

	if r.FormValue("update") == "true" {
		w.Write([]byte(updateArticle(u, id, tag, title, content)))
	} else {
		w.Write([]byte(newArticle(r, u, id, tag, title, content)))
	}
}

func (th ModelHandler) GET_feed_TYPE(w http.ResponseWriter, r *http.Request, ps httprouter.Params) {
	if ps.ByName("type") == "rss" {
		w.Write([]byte(auth.GenerateRSS(false)))
	} else {
		w.Write([]byte(auth.GenerateRSS(true)))
	}
}

func newArticle(r *http.Request, user auth.AuthUser, id int, tag string, title string, content string) string {
	_tag := conf.GlobalServerConfig.GetTagIndex(auth.Escape(tag))
	_title := auth.Escape(title)
	_extracted1, _extracted2, _ := auth.ExtractContent(content, user)
	_preview := auth.Escape(_extracted1)

	if user.ID == 0 {
		ip := strings.Split(auth.GetIP(r), ".")

		if len(ip) >= 4 {
			content = "<div>[ IP: " + strings.Join(ip[:3], ".") + ".* ]</div>" + content
		} else {
			return "Err::Post::Cannot_Get_IP"
		}
	}

	_content := auth.Escape(_extracted2)

	if _tag == -1 {
		return "Err::Post::Invalid_Tag"
	}

	if user.ID == 0 && (_tag != conf.GlobalServerConfig.AnonymousArea && _tag != conf.GlobalServerConfig.ReplyArea) {
		return "Err::Post::Invalid_Tag"
	}

	if _tag == conf.GlobalServerConfig.MessageArea {
		_tag = id + 100000
		id = 0
	}

	cooldown := conf.GlobalServerConfig.GetInt(user.Group, "Cooldown")
	_now := auth.Time.Now()

	sql := `SELECT 
               new_article('%s',   %d,   '%s',     '%s',     '%s', '%s', %d,      %d, %d);`
	//                      |      |      |         |         |     |    |        |   |
	//                      V      V      V         V         V     V    V        V   V
	sql = fmt.Sprintf(sql, _title, _tag, _content, _preview, _now, _now, user.ID, id, cooldown)

	var succ int
	err := auth.Gdb.QueryRow(sql).Scan(&succ)

	if err == nil {
		// row.Close()
		if succ == 0 {
			auth.Gcache.Clear()
			return "ok"
		} else {
			return "Err::Post::Cooldown_" + strconv.Itoa(cooldown-succ) + "s"
		}
	} else {
		glog.Errorln("Database:", err)
		return "Err::DB::General_Failure"
	}
}

func updateArticle(user auth.AuthUser, id int, tag string, title string, content string) string {
	_tag := conf.GlobalServerConfig.GetTagIndex(auth.Escape(tag))
	_title := auth.Escape(title)
	_extracted1, _extracted2, _ := auth.ExtractContent(content, user)
	_preview := auth.Escape(_extracted1)
	_content := auth.Escape(_extracted2)

	if _tag == -1 {
		return "Err::Post::Invalid_Tag"
	}

	var authorID, revision int
	var oldContent, oldTitle string
	var oldTime time.Time
	var locked bool

	if auth.Gdb.QueryRow(`
        SELECT 
            author,
            title,
            content,
            modified_at,
            locked,
            rev 
        FROM 
            articles 
        WHERE 
            id = `+strconv.Itoa(id)).
		Scan(&authorID, &oldTitle, &oldContent, &oldTime, &locked, &revision) != nil {
		return "Err::DB::Select_Failure"
	}

	if revision >= conf.GlobalServerConfig.MaxRevision {
		locked = true
		auth.Gdb.Exec(`UPDATE articles SET locked = true WHERE id = ` + strconv.Itoa(id))
	}

	if authorID != user.ID && !conf.GlobalServerConfig.GetPrivilege(user.Group, "EditOthers") {
		return "Err::Privil::Edit_Action_Denied"
	}

	if 0 == user.ID {
		return "Err::Privil::Edit_Action_Denied"
	}

	if locked && !conf.GlobalServerConfig.GetPrivilege(user.Group, "MakeLocked") {
		return "Err::Post::Locked_Article"
	}

	cooldown := conf.GlobalServerConfig.GetInt(user.Group, "Cooldown")

	sql := `SELECT 
            update_article(%d, '%s',   %d,   %d,      '%s',     '%s',     '%s',            '%s',     %d,       '%s',       '%s',                 %d)`
	//                     |    |      |     |         |         |         |                |        |          |           |                    |
	//                     V    V      V     V         V         V         V                V        V          V           V                    V
	sql = fmt.Sprintf(sql, id, _title, _tag, user.ID, _content, _preview, auth.Time.Now(), oldTitle, authorID, oldContent, auth.Time.F(oldTime), cooldown)

	var succ int
	err := auth.Gdb.QueryRow(sql).Scan(&succ)

	if err == nil {
		// row.Close()
		if succ == 0 {
			auth.Gcache.Clear()
			return "ok"
		} else {
			return "Err::Post::Cooldown_" + strconv.Itoa(cooldown-succ) + "s"
		}
	} else {
		glog.Errorln("Database:", err)
		return "Err::DB::General_Failure"
	}
}
