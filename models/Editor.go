package models

import (
	"../auth"
	"../conf"

	"github.com/julienschmidt/httprouter"

	_ "database/sql"
	"fmt"
	"net/http"
	"strconv"
)

type EditorStruct struct {
	Username string
	Tags     map[int]conf.Tag
	ReplyTo  int
	Article  auth.Article

	Update bool

	Message             bool
	MessageReceiverName string

	IsLoggedIn bool

	HTMLTags  map[string]bool
	HTMLAttrs map[string]bool
}

func (th ModelHandler) POST_preview(w http.ResponseWriter, r *http.Request, ps httprouter.Params) {
	content := r.FormValue("content")
	if len(content) > conf.GlobalServerConfig.MaxArticleContentLength*1024 {
		ex := len(content) - conf.GlobalServerConfig.MaxArticleContentLength*1024
		Return(w, fmt.Sprintf("Err::Post::Content_Too_Long_%d_KiB_Exceeded", ex/1024))
		return
	}
	_, p, _ := auth.ExtractContent(content, auth.AuthUser{})
	w.Header().Set("Content-Type", "text/html; charset=utf-8")

	Return(w, p)
}

func PrepareEditor(r *http.Request) (EditorStruct, auth.AuthUser) {
	var payload EditorStruct
	u := auth.GetUser(r)
	payload.Username = u.Name
	payload.IsLoggedIn = u.Name != ""
	payload.Tags = conf.GlobalServerConfig.GetComplexTags()
	payload.HTMLTags = conf.GlobalServerConfig.HTMLTags
	payload.HTMLAttrs = conf.GlobalServerConfig.HTMLAttrs

	return payload, u
}

func (th ModelHandler) GET_new_article_ID(w http.ResponseWriter, r *http.Request, ps httprouter.Params) {
	payload, _ := PrepareEditor(r)

	id, err := strconv.Atoi(ps.ByName("id"))

	if err != nil {
		ServePage(w, r, "404", nil)
		return
	}

	payload.ReplyTo = id
	payload.Update = false
	ServePage(w, r, "editor", payload)
}

// PAGE: Serve editor page where user can edit an article by id
func (th ModelHandler) GET_edit_article_ID(w http.ResponseWriter, r *http.Request, ps httprouter.Params) {
	payload, u := PrepareEditor(r)

	id, err := strconv.Atoi(ps.ByName("id"))

	if err != nil {
		ServePage(w, r, "404", nil)
		return
	}

	payload.ReplyTo = id
	payload.Article = auth.GetArticle(r, u, id, true)
	payload.Update = true

	if payload.Article.AuthorID != u.ID && !conf.GlobalServerConfig.GetPrivilege(u.Group, "EditOthers") {
		if payload.Article.OriginalAuthorID != u.ID {
			ServePage(w, r, "404", nil)
			return
		}
	}

	if payload.Article.IsMessage {
		ServePage(w, r, "404", nil)
		return
	}

	ServePage(w, r, "editor", payload)
}

func (th ModelHandler) GET_new_message_ID(w http.ResponseWriter, r *http.Request, ps httprouter.Params) {
	payload, u := PrepareEditor(r)

	id, err := strconv.Atoi(ps.ByName("id"))

	if err != nil || u.Name == "" || id <= 0 {
		ServePage(w, r, "404", nil)
		return
	}

	payload.ReplyTo = id
	payload.Message = true
	payload.MessageReceiverName = auth.GetUserByID(id).NickName

	ServePage(w, r, "editor", payload)
}
