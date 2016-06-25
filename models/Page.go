package models

import (
	"../auth"
	"../conf"
	// "crypto/sha1"
	_ "database/sql"
	// "encoding/json"
	// "fmt"
	"github.com/julienschmidt/httprouter"
	// "io/ioutil"
	// "log"
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
	TotalPages    int
	PagerLinks    []pager

	IsSearch  bool
	IsReply   bool
	IsMessage bool
	IsOWA     bool

	ShowAlwaysTop   bool
	CurTag          string
	CurType         string
	Tags            map[int]string
	ArticlesPerPage int
}

func PageHandler(index bool, filterType string, w http.ResponseWriter, r *http.Request, ps httprouter.Params) {
	var page int
	if index {
		ServePage(w, "index", nil)
		return
	} else {
		_page, err := strconv.Atoi(ps.ByName("page"))
		if err != nil {
			ServePage(w, "404", nil)
			return
		}

		page = _page
	}

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

	payload.ShowAlwaysTop = page == 1
	payload.CurPage = page
	payload.CurTag = filter
	payload.CurType = filterType

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
		// "message" tag needs different functions
		user := auth.GetUser(r)
		userID, err := strconv.Atoi(filter)
		if err != nil {
			ServePage(w, "404", nil)
			return
		}

		payload.Messages, payload.TotalArticles = auth.GetMessages(page, user.ID, userID)
	} else {
		payload.Articles, payload.TotalArticles = auth.GetArticles(page, filter, filterType)
	}
	payload.Tags = conf.GlobalServerConfig.GetTags()

	maxPages := int(payload.TotalArticles / conf.GlobalServerConfig.ArticlesPerPage)
	if maxPages*conf.GlobalServerConfig.ArticlesPerPage != payload.TotalArticles {
		maxPages++
	}

	payload.TotalPages = maxPages
	payload.ArticlesPerPage = conf.GlobalServerConfig.ArticlesPerPage

	for i := page - 5; i <= page+5; i++ {
		if i >= 1 && i <= maxPages {
			if i == page {
				payload.PagerLinks = append(payload.PagerLinks, pager{})
			} else {
				s := "/" + filterType + "/" + filter + "/page/" + strconv.Itoa(i)
				payload.PagerLinks = append(payload.PagerLinks,
					pager{strings.Replace(s, "/"+filterType+"//", "/", -1), i})
			}
		}
	}

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
