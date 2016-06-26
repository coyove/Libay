package models

import (
	"../auth"
	"../conf"
	"crypto/sha1"
	_ "database/sql"
	"encoding/json"
	"fmt"
	"github.com/golang/glog"
	"github.com/julienschmidt/httprouter"
	"html"
	"io/ioutil"
	"net/http"
	"reflect"
	// "os"
	// "os/exec"
	"os"
	"runtime"
	"strconv"
	"strings"
	"time"
)

type TableRow struct {
	Columns     []string
	ColumnTypes []string
	ColumnName  []string
}

func ReadTableDirect(table string, page int, whereStat string) ([]string, []TableRow, int) {
	ret := make([]TableRow, 0)
	columnNames := make([]string, 0)

	if whereStat != "" {
		whereStat = " WHERE " + whereStat
	}

	_app := conf.GlobalServerConfig.ArticlesPerPage
	_start := _app * (page - 1)

	var count, rowCount int

	cols, err := auth.Gdb.Query("SELECT column_name FROM information_schema.columns WHERE table_name = '" + table + "'")
	if err != nil {
		return columnNames, ret, 0
	}

	defer cols.Close()

	for cols.Next() {
		var cn string
		cols.Scan(&cn)
		columnNames = append(columnNames, cn)
	}

	count = len(columnNames)

	if auth.Gdb.QueryRow(`
        SELECT
            COUNT(id)
        FROM `+table+
		whereStat).Scan(&rowCount) != nil {
		return columnNames, ret, 0
	}

	rows, err := auth.Gdb.Query(`
        SELECT
            *
        FROM ` + table +
		whereStat + `
        ORDER BY id DESC 
        OFFSET ` + strconv.Itoa(_start) + " LIMIT " + strconv.Itoa(_app))
	if err != nil {
		return columnNames, ret, 0
	}

	defer rows.Close()

	ptrCols := make([]interface{}, count)
	ptrs := make([]interface{}, count)
	for i, _ := range ptrs {
		ptrCols[i] = &(ptrs[i])
	}

	for rows.Next() {
		rows.Scan(ptrCols...)
		row := TableRow{}
		row.ColumnTypes = make([]string, 0)
		row.Columns = make([]string, 0)

		for _, v := range ptrCols {
			_v := *(v.(*interface{}))
			row.ColumnTypes = append(row.ColumnTypes, reflect.ValueOf(_v).String())

			s := ""

			switch _v.(type) {
			case []uint8:
				s = string(_v.([]byte))
			case int64:
				s = strconv.Itoa(int(_v.(int64)))
			case bool:
				s = strconv.FormatBool(_v.(bool))
			case time.Time:
				ts := (_v.(time.Time)).Unix()
				s = "ts:" + strconv.Itoa(int(ts))
			default:
				// log.Println("unknown type", reflect.ValueOf(_v).String())
			}

			if len(s) > 48 {
				s = s[:48] + "..."
			}
			row.Columns = append(row.Columns, s)
		}
		ret = append(ret, row)
	}

	return columnNames, ret, rowCount
}

func DeleteRowsDirect(table string, ids []int) string {
	sql := `delete from ` + table + ` where `
	for i, v := range ids {
		sql += "id = " + strconv.Itoa(v)
		if i < len(ids)-1 {
			sql += " or "
		}
	}

	if table == "images" {
		sql2 := `select image from ` + table + ` where `
		for i, v := range ids {
			sql2 += "id = " + strconv.Itoa(v)
			if i < len(ids)-1 {
				sql2 += " or "
			}
		}

		rows, err := auth.Gdb.Query(sql2)
		if err == nil {
			defer rows.Close()

			for rows.Next() {
				var img string
				rows.Scan(&img)
				os.Remove("./images/" + img)
				os.Remove("./thumbs/" + img)
			}
		}
	}

	_, err := auth.Gdb.Exec(sql)

	if err == nil {
		return "ok"
	} else {
		glog.Errorln("Database:", err)
		return "Err::DB::General_Failure"
	}
}

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
		TableRows      []TableRow
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
	payload.TableColumns, payload.TableRows, count = ReadTableDirect(payload.Table, page, payload.WhereStatement)
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
		w.WriteHeader(503)
		return
	}

	ids := make([]int, 0)
	for _, v := range strings.Split(r.FormValue("ids"), ",") {
		id, err := strconv.Atoi(v)
		if err == nil {
			ids = append(ids, id)
		}
	}

	w.Write([]byte(DeleteRowsDirect(ps.ByName("table"), ids)))
}

func (th ModelHandler) POST_database_TABLE_exec(w http.ResponseWriter, r *http.Request, ps httprouter.Params) {
	u := auth.GetUser(r)

	if u.Group != "admin" || !auth.CheckCSRF(r) {
		w.WriteHeader(503)
		return
	}

	_, err := auth.Gdb.Exec(r.FormValue("statement"))
	if err == nil {
		w.Write([]byte("ok"))
	} else {
		w.Write([]byte(fmt.Sprintf("Err::DB::General_Failure_%s", err)))
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
		w.WriteHeader(503)
		return
	}
	runtime.GC()
	w.Write([]byte("GC OK"))
}

func (th ModelHandler) POST_config_update(w http.ResponseWriter, r *http.Request, ps httprouter.Params) {
	u := auth.GetUser(r)

	if u.Group != "admin" || !auth.CheckCSRF(r) {
		w.WriteHeader(503)
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
			w.Write([]byte("ok"))
		} else {
			w.Write([]byte("Err::IO::File_IO_Failure"))
		}
	} else {
		conf.GlobalServerConfig.Lock()
		json.Unmarshal(oldConfig, &conf.GlobalServerConfig)
		conf.GlobalServerConfig.Unlock()

		glog.Errorln("New config is invalid")
		w.Write([]byte("Err::IO::File_IO_Failure"))
	}
}

func (th ModelHandler) POST_tags_update(w http.ResponseWriter, r *http.Request, ps httprouter.Params) {
	u := auth.GetUser(r)

	if u.Group != "admin" || !auth.CheckCSRF(r) {
		w.WriteHeader(503)
		return
	}

	glog.Infoln("Tags updated")
	conf.GlobalServerConfig.InitTags(auth.Gdb)

	w.Write([]byte("ok"))
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
	payload.Content = html.EscapeString(string(buf))
	payload.File = ps.ByName("file")

	ServePage(w, "bootstrap", payload)
}

func (th ModelHandler) POST_bootstrap_FILE(w http.ResponseWriter, r *http.Request, ps httprouter.Params) {
	u := auth.GetUser(r)

	if u.Group != "admin" || !auth.CheckCSRF(r) {
		w.WriteHeader(503)
		return
	}

	old, _ := ioutil.ReadFile("./templates/" + ps.ByName("file"))
	err1 := ioutil.WriteFile("./templates/"+ps.ByName("file")+".bk", old, 0644)
	err2 := ioutil.WriteFile("./templates/"+ps.ByName("file"), []byte(r.FormValue("content")), 0644)

	if err1 == nil && err2 == nil {
		w.Write([]byte("ok"))
	} else {
		w.Write([]byte("Err::IO::File_IO_Failure"))
	}
}

func (th ModelHandler) GET_cache(w http.ResponseWriter, r *http.Request, ps httprouter.Params) {
	u := auth.GetUser(r)

	if u.Group != "admin" {
		w.WriteHeader(503)
		return
	}

	cc := auth.Gcache.GetLowLevelCache()
	caches := make([]string, 0)

	for k, _ := range cc {
		caches = append(caches, k.(string))
	}

	buf, _ := json.Marshal(caches)
	w.Write(buf)
}
