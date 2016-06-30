package models

import (
	"../auth"
	"../conf"

	"github.com/julienschmidt/httprouter"

	_ "database/sql"
	"net/http"
	"strconv"
	"strings"
	"sync/atomic"
	"time"
)

type pager struct {
	Link string
	Page int
}

type PageStruct struct {
	Articles      []auth.Article
	Messages      []auth.Message
	TotalArticles int
	CurPage       int

	IndexPage string
	LastPage  string

	Nav auth.BackForth

	IsSearch  bool
	IsReply   bool
	IsMessage bool
	IsOWA     bool

	IsLastPage  bool
	IsIndexPage bool

	IsOWAViewingGlobal  bool
	IsMessageViewingAll bool

	AnnounceContent string
	AnnounceID      int

	CurTag          string
	CurType         string
	Tags            map[int]string
	ArticlesPerPage int
}

func PageHandler(index bool, filterType string, w http.ResponseWriter, r *http.Request, ps httprouter.Params) {
	// var page int
	if index {
		ServePage(w, "index", nil)
		return
	}
	page := ps.ByName("page")

	_startRender := time.Now().UnixNano()
	defer func() {
		d := time.Now().UnixNano() - _startRender
		atomic.AddInt64(&ServerTotalRenderTime, d)
		atomic.AddInt64(&ServerTotalRenderCount, 1)
	}()

	var payload PageStruct

	filter := ps.ByName(filterType)
	payload.IsSearch = filterType == "search"
	payload.IsMessage = filterType == "message"
	payload.IsReply = filterType == "reply"
	payload.IsOWA = filterType == "owa"

	payload.IsIndexPage = page == "1"

	payload.CurTag = filter
	payload.CurType = filterType
	payload.IndexPage = strings.Replace("/"+filterType+"/"+filter+"/page/1", "/"+filterType+"//", "/", -1)
	payload.LastPage = payload.IndexPage[:len(payload.IndexPage)-1] + "last"
	payload.IsLastPage = page == "last"

	if filterType == "reply" {
		// You cannot view replies under an invalid article
		_filter, err := strconv.Atoi(filter)
		if err != nil || _filter <= 0 {
			ServePage(w, "404", nil)
			return
		}
	}

	if filterType == "tag" {
		// No one can access "message" main tag or any tag whose index is > 100000
		_tag := conf.GlobalServerConfig.GetTagIndex(filter)
		if _tag == conf.GlobalServerConfig.MessageArea || _tag > 100000 {
			ServePage(w, "404", nil)
			return
		}
	}

	if filterType == "owa" {
		user := auth.GetUser(r)
		_arr := strings.Split(filter, ":")
		userID, err := strconv.Atoi(_arr[0])

		if userID == 0 {
			// "0" means "global", only admin and users with "ViewOtherTrash" privilege can access
			if !conf.GlobalServerConfig.GetPrivilege(user.Group, "ViewOtherTrash") {
				ServePage(w, "404", nil)
				return
			}

			payload.IsOWAViewingGlobal = true
		} else {
			// Each user by default can only access his own articles
			// Admin and users with "ViewOtherTrash" privilege can access others' articles
			if err != nil || (userID != user.ID && !conf.GlobalServerConfig.GetPrivilege(user.Group, "ViewOtherTrash")) {
				ServePage(w, "404", nil)
				return
			}
		}
	}

	if filterType == "message" {
		user := auth.GetUser(r)
		userID, err := strconv.Atoi(filter)
		if err != nil || userID < 0 {
			ServePage(w, "404", nil)
			return
		}
		payload.IsMessageViewingAll = user.ID == userID
		payload.Messages, payload.Nav = auth.GetMessages(page, user.ID, userID)
	} else {
		payload.Articles, payload.Nav = auth.GetArticles(page, filter, filterType)

		if page == "1" {
			id := 0
			if filterType == "tag" {
				_tag := conf.GlobalServerConfig.GetTagIndex(filter)
				id = conf.GlobalServerConfig.GetComplexTags()[_tag].AnnounceID
			} else if filterType == "" {
				id = conf.GlobalServerConfig.GetComplexTags()[0].AnnounceID
			}

			payload.AnnounceContent = auth.GetArticle(r, auth.AuthUser{Group: "admin"}, id, false).Content
			payload.AnnounceID = id
		}
	}

	if len(payload.Articles) == 0 && len(payload.Messages) == 0 && page != "1" {
		http.Redirect(w, r,
			payload.IndexPage,
			http.StatusFound)
		return
	}

	payload.Tags = conf.GlobalServerConfig.GetTags()
	payload.ArticlesPerPage = conf.GlobalServerConfig.ArticlesPerPage

	ServePage(w, "articles", payload)
}

func (th ModelHandler) GET_page_PAGE(w http.ResponseWriter, r *http.Request, ps httprouter.Params) {
	PageHandler(false, "", w, r, ps)
}

func (th ModelHandler) GET_(w http.ResponseWriter, r *http.Request, ps httprouter.Params) {
	PageHandler(true, "", w, r, ps)
}

func (th ModelHandler) GET_search_SEARCH_page_PAGE(w http.ResponseWriter, r *http.Request, ps httprouter.Params) {
	PageHandler(false, "search", w, r, ps)
}

func (th ModelHandler) GET_tag_TAG_page_PAGE(w http.ResponseWriter, r *http.Request, ps httprouter.Params) {
	PageHandler(false, "tag", w, r, ps)
}

func (th ModelHandler) GET_ua_UA_page_PAGE(w http.ResponseWriter, r *http.Request, ps httprouter.Params) {
	PageHandler(false, "ua", w, r, ps)
}

func (th ModelHandler) GET_reply_REPLY_page_PAGE(w http.ResponseWriter, r *http.Request, ps httprouter.Params) {
	PageHandler(false, "reply", w, r, ps)
}

func (th ModelHandler) GET_message_MESSAGE_page_PAGE(w http.ResponseWriter, r *http.Request, ps httprouter.Params) {
	PageHandler(false, "message", w, r, ps)
}

func (th ModelHandler) GET_owa_OWA_page_PAGE(w http.ResponseWriter, r *http.Request, ps httprouter.Params) {
	PageHandler(false, "owa", w, r, ps)
}
