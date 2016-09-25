package models

import (
	"../auth"
	"../conf"

	"github.com/golang/glog"
	"github.com/julienschmidt/httprouter"

	_ "database/sql"
	"fmt"
	"net/http"
	"strconv"
)

func canUserViewThis(u auth.AuthUser, article auth.Article, admin bool) bool {
	if article.Deleted && !admin {
		if u.ID == 0 {
			return false
		}

		if u.ID != article.AuthorID && u.ID != article.OriginalAuthorID {
			return false
		}
	}

	if article.IsMessage {
		if u.ID == 0 {
			return false
		}

		if article.IsOthersMessage && !admin {
			return false
		}
	}

	if article.IsRestricted {
		return false
	}

	return true
}

func (th ModelHandler) GET_article_ID(w http.ResponseWriter, r *http.Request, ps httprouter.Params) {
	id, err := strconv.Atoi(ps.ByName("id"))
	if err != nil {
		ServePage(w, r, "404", nil)
		return
	}

	var payload struct {
		Article          auth.Article
		IsAuthorSelf     bool
		IsEditedByOthers bool

		CanMakeLocked bool

		User       auth.AuthUser
		IsLoggedIn bool
	}
	u := auth.GetUser(r, w)
	vtt := conf.GlobalServerConfig.GetPrivilege(u.Group, "ViewOthers")

	payload.Article = auth.GetArticle(r, u, id, false)
	payload.IsAuthorSelf = (u.ID == payload.Article.AuthorID || vtt || u.ID == payload.Article.OriginalAuthorID)
	payload.IsEditedByOthers = (payload.Article.AuthorID != payload.Article.OriginalAuthorID)

	payload.CanMakeLocked = conf.GlobalServerConfig.GetPrivilege(u.Group, "MakeLocked")

	payload.User = u
	payload.IsLoggedIn = u.Name != ""

	if canUserViewThis(u, payload.Article, vtt) {
		ServePage(w, r, "article", payload)
	} else {
		ServePage(w, r, "404", nil)
	}

}

func (th ModelHandler) GET_article_ID_raw_HID(w http.ResponseWriter, r *http.Request, ps httprouter.Params) {
	id, err := strconv.Atoi(ps.ByName("id"))
	if err != nil {
		ServePage(w, r, "404", nil)
		return
	}

	hid, err := strconv.Atoi(ps.ByName("hid"))
	if err != nil {
		ServePage(w, r, "404", nil)
		return
	}

	raw := ""
	u := auth.GetUser(r, w)
	article := auth.GetArticle(r, u, id, false)

	if !canUserViewThis(u, article, conf.GlobalServerConfig.GetPrivilege(u.Group, "ViewOthers")) {
		ServePage(w, r, "404", nil)
		return
	}

	if hid == 0 {
		raw = article.Raw
	} else {

		if his, err := auth.Select1("history", hid, "raw"); err != nil {
			ServePage(w, r, "404", nil)
			return
		} else {
			raw = his["raw"].(string)
		}
	}

	w.Header().Set("Content-Type", "text/plain; charset=utf-8")
	w.Write([]byte(auth.Unescape(raw)))
}

func (th ModelHandler) GET_article_ID_history(w http.ResponseWriter, r *http.Request, ps httprouter.Params) {
	id, err := strconv.Atoi(ps.ByName("id"))
	// u := auth.GetUser(r)

	if err != nil {
		Return(w, 503)
		return
	}

	type pair struct {
		Date int
		User string
	}
	ret := make(map[string]pair)

	rows, err := auth.Gdb.Query(`
        SELECT
            history.id,
            history.date,
            COALESCE(users.nickname, 'user' || history.user_id::TEXT)
        FROM
            history
        LEFT JOIN
            users ON users.id = history.user_id
        WHERE
            article_id = ` + strconv.Itoa(id))

	if err != nil {
		glog.Errorln("Database:", err)
		Return(w, 503)
		return
	}

	defer rows.Close()

	for rows.Next() {
		var id int
		var username string
		var t int

		rows.Scan(&id, &t, &username)

		ret[strconv.Itoa(id)] = pair{t, username}
	}

	Return(w, ret)
}

func (th ModelHandler) GET_article_ID_history_HID(w http.ResponseWriter, r *http.Request, ps httprouter.Params) {
	aid, err := strconv.Atoi(ps.ByName("id"))
	hid, err2 := strconv.Atoi(ps.ByName("hid"))
	if err != nil || err2 != nil {
		Return(w, 503)
		return
	}

	u := auth.GetUser(r)
	article := auth.GetArticle(r, u, aid, false)

	if !canUserViewThis(u, article, conf.GlobalServerConfig.GetPrivilege(u.Group, "EditOthers")) {
		Return(w, 503)
		return
	}

	if his, err := auth.Select1("history", hid, "title", "content", "raw"); err != nil {
		Return(w, 503)
	} else {
		var payload struct {
			Title   string
			Content string
			Raw     string
		}
		payload.Title = his["title"].(string)
		payload.Content = his["content"].(string)
		payload.Raw = his["raw"].(string)
		Return(w, payload)
	}
}

func (th ModelHandler) POST_delete_article_ID_ACTION(w http.ResponseWriter, r *http.Request, ps httprouter.Params) {
	if !auth.LogIP(r) {
		Return(w, "Err::Router::Frequent_Access")
		return
	}

	u := auth.GetUser(r)
	if !u.CanPost() {
		Return(w, "Err::Privil::Post_Action_Denied")
		return
	}

	id, err := strconv.Atoi(ps.ByName("id"))
	if err != nil {
		Return(w, "Err::Router::Invalid_Article_Id")
		return
	}

	article := auth.GetArticle(nil, auth.DummyUsers[0], id, true)
	if article.ID == 0 {
		Return(w, "Err::DB::Select_Failure")
		return
	}

	if article.AuthorID != u.ID && article.OriginalAuthorID != u.ID && u.Group != "admin" {
		if conf.GlobalServerConfig.GetPrivilege(u.Group, "DeleteOthers") {
			// User with "DeleteOthers" privilege can delete others' articles
		} else if u.ID == article.TagID-100000 {
			// Both the receiver and the sender can delete the message
		} else {
			Return(w, "Err::Privil::Delete_Restore_Action_Denied")
			return
		}
	}

	if article.Locked && !conf.GlobalServerConfig.GetPrivilege(u.Group, "MakeLocked") {
		Return(w, "Err::Post::Locked_Article")
		return
	}

	Return(w, auth.InvertArticleState(u, id, "deleted"))
}

func (th ModelHandler) POST_delete_messages_from_ID(w http.ResponseWriter, r *http.Request, ps httprouter.Params) {
	if !auth.LogIP(r) {
		Return(w, "Err::Router::Frequent_Access")
		return
	}

	u := auth.GetUser(r)

	id, err := strconv.Atoi(ps.ByName("id"))
	if err != nil || id < 0 || u.ID == 0 {
		Return(w, "Err::Router::Invalid_User_Id")
		return
	}

	if _, err := auth.Gdb.Exec(`
        UPDATE articles 
        SET    tag = 100000 
        WHERE 
            tag = ` + strconv.Itoa(u.ID+100000) + `
        AND
            author = ` + strconv.Itoa(id)); err != nil {
		Return(w, "Err::DB::Update_Failure")
	} else {
		Return(w, "ok")
	}
}

func (th ModelHandler) POST_lock_article_ID(w http.ResponseWriter, r *http.Request, ps httprouter.Params) {
	u := auth.GetUser(r)
	if !u.CanPost() {
		Return(w, "Err::Privil::Post_Action_Denied")
		return
	}

	id, err := strconv.Atoi(ps.ByName("id"))
	if err != nil {
		Return(w, "Err::Router::Invalid_Article_Id")
		return
	}

	// User with "MakeLocked" privilege can (un)lock articles
	if u.Group != "admin" && !conf.GlobalServerConfig.GetPrivilege(u.Group, "MakeLocked") {
		Return(w, "Err::Privil::Lock_Action_Denied")
		return
	}

	Return(w, auth.InvertArticleState(u, id, "locked"))
}

func (th ModelHandler) POST_post_ID(w http.ResponseWriter, r *http.Request, ps httprouter.Params) {
	if !auth.LogIP(r) {
		Return(w, "Err::Router::Frequent_Access")
		return
	}

	u := auth.GetUser(r)
	if !u.CanPost() {
		Return(w, "Err::Privil::Post_Action_Denied")
		return
	}

	id, err := strconv.Atoi(ps.ByName("id"))
	if err != nil {
		Return(w, "Err::Router::Invalid_Article_Id")
		return
	}

	content := r.FormValue("content")
	if ex := len(content) - conf.GlobalServerConfig.MaxArticleContentLength*1024; ex > 0 {
		Return(w, fmt.Sprintf("Err::Post::Content_Too_Long_%d_KiB_Exceeded", ex/1024))
		return
	}

	title := r.FormValue("title")
	if len(title) > 512 {
		title = title[:512]
	} else if len(title) < 3 {
		Return(w, "Err::Post::Title_Too_Short")
		return
	}

	tag := r.FormValue("tag")
	if len(tag) > 128 {
		tag = tag[:128]
	}

	if r.FormValue("update") == "true" {
		Return(w, auth.UpdateArticle(u, id, tag, title, content))
	} else {
		Return(w, auth.NewArticle(r, u, id, tag, title, content))
	}
}

func (th ModelHandler) GET_feed_TYPE(w http.ResponseWriter, r *http.Request, ps httprouter.Params) {
	w.Header().Add("Content-Type", "text/xml; charset=utf-8")
	a, _ := auth.GetArticles("1", "", "")

	if ps.ByName("type") == "rss" {
		Return(w, auth.GenerateRSS(a))
	} else {
		Return(w, auth.GenerateAtom(a))
	}
}
