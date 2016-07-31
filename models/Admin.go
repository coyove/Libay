package models

import (
	"../auth"
	"../conf"

	"github.com/golang/glog"
	"github.com/julienschmidt/httprouter"

	"crypto/sha1"
	_ "database/sql"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"regexp"
	"runtime"
	"strconv"
	"strings"
)

func (th ModelHandler) GET_database_TABLE_page_PAGE(w http.ResponseWriter, r *http.Request, ps httprouter.Params) {
	page, err := strconv.Atoi(ps.ByName("page"))
	u := auth.GetUser(r)

	if err != nil || u.Group != "admin" {
		ServePage(w, "404", nil)
		return
	}

	type pager struct {
		Link string
		Page int
	}
	var payload struct {
		Table          string
		Full           string
		WhereStatement string
		CurPage        int
		PagerLinks     []pager
		TableRows      []auth.TableRow
		TableColumns   []string
	}

	payload.CurPage = page
	payload.Table = ps.ByName("table")
	payload.Full = ps.ByName("table")
	if strings.Contains(payload.Table, ":") {
		tmp := strings.Split(payload.Table, ":")
		payload.Table = tmp[0]
		payload.WhereStatement = tmp[1]
	}

	var count int
	payload.TableColumns, payload.TableRows, count = auth.ReadTableDirect(payload.Table, page, payload.WhereStatement)
	maxPages := int(count/conf.GlobalServerConfig.ArticlesPerPage) + 1

	for i := page - 5; i <= page+5; i++ {
		if i >= 1 && i <= maxPages {
			if i == page {
				payload.PagerLinks = append(payload.PagerLinks, pager{})
			} else {
				payload.PagerLinks = append(payload.PagerLinks,
					pager{"/database/" + payload.Full + "/page/" + strconv.Itoa(i), i})
			}
		}
	}

	ServePage(w, "database", payload)
}

func (th ModelHandler) POST_database_TABLE_delete(w http.ResponseWriter, r *http.Request, ps httprouter.Params) {
	u := auth.GetUser(r)

	if u.Group != "admin" || !auth.CheckCSRF(r) {
		Return(w, 503)
		return
	}

	ids := make([]int, 0)
	for _, v := range strings.Split(r.FormValue("ids"), ",") {
		id, err := strconv.Atoi(v)
		if err == nil {
			ids = append(ids, id)
		}
	}

	ret := auth.DeleteRowsDirect(ps.ByName("table"), ids)
	auth.Gcache.Clear()
	Return(w, ret)
}

func (th ModelHandler) POST_database_TABLE_exec(w http.ResponseWriter, r *http.Request, ps httprouter.Params) {
	u := auth.GetUser(r)

	if u.Group != "admin" || !auth.CheckCSRF(r) {
		Return(w, 503)
		return
	}

	_, err := auth.Gdb.Exec(r.FormValue("statement"))
	if err == nil {
		auth.Gcache.Clear()
		Return(w, "ok")
	} else {
		Return(w, fmt.Sprintf("Err::DB::General_Failure_%s", err))
	}
}

func (th ModelHandler) GET_config_sheet(w http.ResponseWriter, r *http.Request, ps httprouter.Params) {
	u := auth.GetUser(r)

	if u.Group != "admin" {
		ServePage(w, "404", nil)
		return
	}

	buf, _ := json.Marshal(conf.GlobalServerConfig)
	var payload struct {
		JSON string
	}

	payload.JSON = string(buf)
	ServePage(w, "config", payload)
}

func (th ModelHandler) GET_gc(w http.ResponseWriter, r *http.Request, ps httprouter.Params) {
	u := auth.GetUser(r)

	if u.Group != "admin" {
		Return(w, 503)
		return
	}
	runtime.GC()
	Return(w, "GC OK")
}

func (th ModelHandler) POST_config_update(w http.ResponseWriter, r *http.Request, ps httprouter.Params) {
	u := auth.GetUser(r)

	if u.Group != "admin" || !auth.CheckCSRF(r) {
		Return(w, 503)
		return
	}

	path := conf.GlobalServerConfig.ConfigPath
	oldConfig, _ := json.Marshal(conf.GlobalServerConfig)
	newConfig := []byte(r.FormValue("config"))

	err1 := ioutil.WriteFile(path+".bk", oldConfig, 0644)

	conf.GlobalServerConfig.Lock()
	err3 := json.Unmarshal(newConfig, &conf.GlobalServerConfig)
	conf.GlobalServerConfig.Unlock()

	if err1 == nil && err3 == nil {
		glog.Infoln("Config updated")
		// conf.GlobalServerConfig.InitTags(auth.Gdb)
		ConfigChecksum = fmt.Sprintf("%x", sha1.Sum(newConfig))[:8]

		if ioutil.WriteFile(path, newConfig, 0644) == nil {
			Return(w, "ok")
		} else {
			Return(w, "Err::IO::File_IO_Failure")
		}
	} else {
		conf.GlobalServerConfig.Lock()
		json.Unmarshal(oldConfig, &conf.GlobalServerConfig)
		conf.GlobalServerConfig.Unlock()

		glog.Errorln("New config is invalid")
		Return(w, "Err::IO::File_IO_Failure")
	}
}

func (th ModelHandler) POST_tags_update(w http.ResponseWriter, r *http.Request, ps httprouter.Params) {
	u := auth.GetUser(r)

	if u.Group != "admin" || !auth.CheckCSRF(r) {
		Return(w, 503)
		return
	}

	glog.Infoln("Tags updated")
	conf.GlobalServerConfig.InitTags(auth.Gdb)

	Return(w, "ok")
}

func (th ModelHandler) GET_bootstrap(w http.ResponseWriter, r *http.Request, ps httprouter.Params) {
	u := auth.GetUser(r)

	if u.Group != "admin" {
		ServePage(w, "404", nil)
		return
	}

	var payload struct {
		List      bool
		Templates []string
	}

	payload.List = true
	if files, err := ioutil.ReadDir("./templates"); err == nil {
		for _, f := range files {
			if f.Name()[len(f.Name())-3:] != ".bk" {
				payload.Templates = append(payload.Templates, f.Name())
			}
		}
	}
	ServePage(w, "bootstrap", payload)
}

func (th ModelHandler) GET_bootstrap_FILE(w http.ResponseWriter, r *http.Request, ps httprouter.Params) {
	u := auth.GetUser(r)

	if u.Group != "admin" {
		ServePage(w, "404", nil)
		return
	}

	var payload struct {
		List    bool
		Content string
		File    string
	}

	payload.List = false
	buf, _ := ioutil.ReadFile("./templates/" + ps.ByName("file"))
	payload.Content = auth.Escape(string(buf))
	payload.File = ps.ByName("file")

	ServePage(w, "bootstrap", payload)
}

func (th ModelHandler) POST_bootstrap_FILE(w http.ResponseWriter, r *http.Request, ps httprouter.Params) {
	u := auth.GetUser(r)

	if u.Group != "admin" || !auth.CheckCSRF(r) {
		Return(w, 503)
		return
	}

	old, _ := ioutil.ReadFile("./templates/" + ps.ByName("file"))
	err1 := ioutil.WriteFile("./templates/"+ps.ByName("file")+".bk", old, 0644)
	err2 := ioutil.WriteFile("./templates/"+ps.ByName("file"), []byte(r.FormValue("content")), 0644)

	if err1 == nil && err2 == nil {
		Return(w, "ok")
	} else {
		Return(w, "Err::IO::File_IO_Failure")
	}
}

func (th ModelHandler) GET_cache(w http.ResponseWriter, r *http.Request, ps httprouter.Params) {
	u := auth.GetUser(r)

	if u.Group != "admin" {
		Return(w, 503)
		return
	}

	cc := auth.Gcache.GetLowLevelCache()
	cu := auth.Guser.GetLowLevelCache()

	timer := func(arr []interface{}) string {
		ret, div := "", "<div class=gd>"
		max := int64(0)
		for _, ut := range arr {
			ret += fmt.Sprintf("%.3f, ", float64(ut.(int64))/1e6)
			if ut.(int64) > max {
				max = ut.(int64)
			}
		}

		for _, ut := range arr {
			div += fmt.Sprintf("<span class=g style='height: %.0fpx'></span>", float64(ut.(int64))/float64(max)*12)
		}

		return ret + div + "</div>"
	}

	caches := []string{
		`<style>
			span.g { display: inline-block; background-color: black; width: 5px; margin-bottom: -1px; }
			div.gd { display: inline-block; line-height: 12px; }
    	</style>`,
		"<img src='./assets/test.png'>",
		"<hr>Recent SQL execution time:",
		"U: " + timer(auth.GuserTimer.Get()),
		"A: " + timer(auth.GarticleTimer.Get()),
		"M: " + timer(auth.GmessageTimer.Get()),
		"<hr>Pages:",
	}

	rindex := regexp.MustCompile(`(.+)--`)
	rpage := regexp.MustCompile(`(.+)-(.+)-(ua|owa|reply|tag)`)
	rarticle := regexp.MustCompile(`(\d+)-(\d+)-(true|false)`)
	makehref := func(url string) string {
		return fmt.Sprintf(`<a href='%s' target='_blank'>%s</a>`, url, url)
	}

	for k, v := range cc {
		name := k.(string)
		url := []string{}

		if rindex.MatchString(name) {
			url = []string{"/page/" + rindex.FindStringSubmatch(name)[1], ""}

		} else if rpage.MatchString(name) {
			pages := rpage.FindStringSubmatch(name)
			url = []string{fmt.Sprintf("/%s/%s/page/%s", pages[3], pages[2], pages[1]), ""}

		} else if rarticle.MatchString(name) {
			articles := rarticle.FindStringSubmatch(name)
			url = []string{"/article/" + articles[2], "/user/" + articles[1]}
		} else {
			url = []string{name, ""}
		}

		_, sec, hits := auth.Gcache.Info(v)

		if sec < 0 {
			caches = append(caches, fmt.Sprintf("Hits: %5d, waits purging: %s %s", hits, makehref(url[0]), makehref(url[1])))
		} else {
			caches = append(caches, fmt.Sprintf("Hits: %5d, expire in %2ds: %s %s", hits, sec, makehref(url[0]), makehref(url[1])))
		}
	}

	caches = append(caches, "<hr>Users:")

	for _, v := range cu {
		_v, sec, hits := auth.Gcache.Info(v)
		user := _v.(auth.AuthUser)
		url := fmt.Sprintf("<a href='/user/%d' target='_blank'>%s(%s)</a>", user.ID, user.Name, user.NickName)

		if sec < 0 {
			caches = append(caches, fmt.Sprintf("Hits: %5d, waits purging: %s", hits, url))
		} else {
			caches = append(caches, fmt.Sprintf("Hits: %5d, expire in %2ds: %s", hits, sec, url))
		}
	}

	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	Return(w, "<pre>"+strings.Join(caches, "<br>")+"</pre>")
}
