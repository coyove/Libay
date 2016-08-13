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

type BasePage struct {
	IndexPage string
	LastPage  string

	IsLastPage  bool
	IsIndexPage bool

	Nav auth.BackForth
}

type PageStruct struct {
	BasePage

	Articles []auth.Article

	IsReply bool
	IsOWA   bool
	IsTag   bool
	IsUA    bool

	OWA struct {
		IsViewingGlobal  bool
		IsViewingOther   bool
		ViewingOtherName string
	}

	UA struct {
		UserNickName string
	}

	Announce auth.Article

	CurTag  string
	CurType string
	Tags    map[int]string
}

type MessageStruct struct {
	BasePage

	Messages []auth.Message

	Message struct {
		IsViewingAll     bool
		ViewingOtherName string
	}
}

func PageHandler(filterType string, w http.ResponseWriter, r *http.Request, ps httprouter.Params) {
	page := ps.ByName("page")

	_startRender := time.Now().UnixNano()
	defer func() {
		d := time.Now().UnixNano() - _startRender
		atomic.AddInt64(&ServerTotalRenderTime, d)
		atomic.AddInt64(&ServerTotalRenderCount, 1)
	}()

	var payload PageStruct

	filter := ps.ByName(filterType)
	payload.IsReply = filterType == "reply"
	payload.IsOWA = filterType == "owa"
	payload.IsTag = filterType == "tag"
	payload.IsUA = filterType == "ua"

	payload.CurTag = filter
	payload.CurType = filterType

	payload.IsLastPage = page == "last"
	payload.IsIndexPage = page == "1"
	payload.IndexPage = strings.Replace("/"+filterType+"/"+filter+"/page/1", "/"+filterType+"//", "/", -1)
	payload.LastPage = payload.IndexPage[:len(payload.IndexPage)-1] + "last"

	if payload.IsReply {
		// You cannot view replies under an invalid article
		_filter, err := strconv.Atoi(filter)
		if err != nil || _filter <= 0 {
			ServePage(w, "404", nil)
			return
		}
	}

	if payload.IsTag {
		// No one can access "message" main tag or any tag whose index is > 100000
		_tag := conf.GlobalServerConfig.GetTagIndex(filter)
		if _tag == conf.GlobalServerConfig.MessageArea || _tag >= 100000 {
			ServePage(w, "404", nil)
			return
		}

		if strconv.Itoa(_tag) == filter {
			payload.CurTag = conf.GlobalServerConfig.GetIndexTag(_tag)
		}
	}

	if payload.IsOWA {
		user := auth.GetUser(r)
		_arr := strings.Split(filter, ":")
		userID, err := strconv.Atoi(_arr[0])

		if userID == 0 {
			// "0" means "global", only admin and users with "ViewOtherTrash" privilege can access
			if !conf.GlobalServerConfig.GetPrivilege(user.Group, "ViewOtherTrash") {
				ServePage(w, "404", nil)
				return
			}

			payload.OWA.IsViewingGlobal = true
		} else {
			// Each user by default can only access his own articles
			// Admin and users with "ViewOtherTrash" privilege can access others' articles
			if err != nil || (userID != user.ID && !conf.GlobalServerConfig.GetPrivilege(user.Group, "ViewOtherTrash")) {
				ServePage(w, "404", nil)
				return
			}
		}

		payload.OWA.IsViewingOther = userID != user.ID
		if payload.OWA.IsViewingOther {
			vu := auth.GetUserByID(userID)
			payload.OWA.ViewingOtherName = vu.Name + "(" + vu.NickName + ")"
		}
		payload.Tags = conf.GlobalServerConfig.GetTags()
	}

	if payload.IsUA {
		userID, err := strconv.Atoi(filter)
		if err != nil || userID <= 0 {
			ServePage(w, "404", nil)
			return
		}

		payload.UA.UserNickName = auth.GetUserByID(userID).NickName
	}

	payload.Articles, payload.Nav = auth.GetArticles(page, filter, filterType)

	if page == "1" {
		id := 0
		if filterType == "tag" {
			_tag := conf.GlobalServerConfig.GetTagIndex(filter)
			id = conf.GlobalServerConfig.GetComplexTags()[_tag].AnnounceID
		} else if filterType == "" {
			id = conf.GlobalServerConfig.GetComplexTags()[0].AnnounceID
		}

		payload.Announce = auth.GetArticle(r, auth.AuthUser{Group: "admin"}, id, false)
	}

	if len(payload.Articles) == 0 && page != "1" {
		http.Redirect(w, r, payload.IndexPage, http.StatusFound)
		return
	}

	if payload.IsTag {
		for i, _ := range payload.Articles {
			payload.Articles[i].Tag = ""
		}
	}

	ServePage(w, "articles", payload)
}

func MessageHandler(w http.ResponseWriter, r *http.Request, ps httprouter.Params) {

	_startRender := time.Now().UnixNano()
	defer func() {
		d := time.Now().UnixNano() - _startRender
		atomic.AddInt64(&ServerTotalRenderTime, d)
		atomic.AddInt64(&ServerTotalRenderCount, 1)
	}()

	var payload MessageStruct

	filter := ps.ByName("message")
	page := ps.ByName("page")

	payload.IsLastPage = page == "last"
	payload.IsIndexPage = page == "1"
	payload.IndexPage = "/message/" + filter + "/page/1"
	payload.LastPage = "/message/" + filter + "/page/last"

	user := auth.GetUser(r)
	userID, err := strconv.Atoi(filter)
	if err != nil || userID < 0 {
		ServePage(w, "404", nil)
		return
	}
	payload.Message.IsViewingAll = user.ID == userID
	payload.Message.ViewingOtherName = auth.GetUserByID(userID).NickName
	payload.Messages, payload.Nav = auth.GetMessages(page, user.ID, userID)

	if len(payload.Messages) == 0 && page != "1" {
		http.Redirect(w, r, payload.IndexPage, http.StatusFound)
		return
	}

	ServePage(w, "message", payload)
}

func (th ModelHandler) GET_page_PAGE(w http.ResponseWriter, r *http.Request, ps httprouter.Params) {
	PageHandler("", w, r, ps)
}

func (th ModelHandler) GET_tag_TAG_page_PAGE(w http.ResponseWriter, r *http.Request, ps httprouter.Params) {
	PageHandler("tag", w, r, ps)
}

func (th ModelHandler) GET_ua_UA_page_PAGE(w http.ResponseWriter, r *http.Request, ps httprouter.Params) {
	PageHandler("ua", w, r, ps)
}

func (th ModelHandler) GET_reply_REPLY_page_PAGE(w http.ResponseWriter, r *http.Request, ps httprouter.Params) {
	PageHandler("reply", w, r, ps)
}

func (th ModelHandler) GET_message_MESSAGE_page_PAGE(w http.ResponseWriter, r *http.Request, ps httprouter.Params) {
	MessageHandler(w, r, ps)
}

func (th ModelHandler) GET_owa_OWA_page_PAGE(w http.ResponseWriter, r *http.Request, ps httprouter.Params) {
	PageHandler("owa", w, r, ps)
}
