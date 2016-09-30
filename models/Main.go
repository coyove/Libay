package models

import (
	"../auth"
	"../conf"

	"github.com/golang/glog"

	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"os"
	"strconv"
	"strings"
	"text/template"
	"time"
)

// Err::CSRF::CSRF_Failure

// Err::Privil::Invalid_User
// Err::Privil::Post_Action_Denied
// Err::Privil::Edit_Action_Denied
// Err::Privil::Delete_Restore_Action_Denied
// Err::Privil::Announce_Action_Denied

// Err::Post::Comment_Too_Long
// Err::Post::Content_Too_Long_%d_KiB_Exceeded
// Err::Post::Title_Too_Short
// Err::Post::Cannot_Get_IP
// Err::Post::Invalid_Tag
// Err::Post::Cooldown
// Err::Post::Locked_Article

// Err::Router::Invalid_Article_Id
// Err::Router::Frequent_Access

// Err::DB::Update_Failure
// Err::DB::Select_Failure
// Err::DB::Insert_Failure
// Err::DB::General_Failure

// Err::IO::File_IO_Failure

// Err::Login::Empty_Username
// Err::Login::Empty_Password
// Err::Login::Cooldown
// Err::Login::Account_Locked
// Err::Login::Retry

// Err::Regr::Registration_Closed
// Err::Regr::Username_Too_Short
// Err::Regr::Nickname_Too_Short
// Err::Regr::Public_Key_Existed
// Err::Regr::Invalid_Public_Key
// Err::Regr::Username_Nickname_Existed

type ModelHandler struct {
}

type _Template struct {
	Content map[string]*template.Template
	Time    time.Time
}

var templates map[string]_Template

var ModelHandlerDummy ModelHandler
var ServerHostname string
var DatabaseVersion string
var ServerLoad string
var ServerStartUp time.Time
var ServerTotalRenderTime int64
var ServerTotalRenderCount int64

func ServePage(w http.ResponseWriter, r *http.Request, fp string, pl interface{}) {

	if fp == "404" {
		w.WriteHeader(404)
	}

	var title struct {
		Title       string
		URL         string
		CDN         string
		ImageServer string
		MainJS      string
		MainCSS     string
		CurrentNav  string
	}

	title.Title = conf.GlobalServerConfig.Title
	title.URL = conf.GlobalServerConfig.Host
	title.CDN = conf.GlobalServerConfig.CDNPrefix
	title.ImageServer = conf.GlobalServerConfig.ImageHost
	title.MainCSS = conf.GlobalServerConfig.MainCSS
	title.MainJS = conf.GlobalServerConfig.MainJS

	switch fp {
	case "404", "footer", "header":
	case "account":
		title.CurrentNav = "nv-console"
	case "editor":
		title.CurrentNav = "nv-new-article"
	case "articles":
		ps := pl.(PageStruct)

		switch ps.CurType {
		case "":
			title.CurrentNav = "nv-articles"
		case "ua":
			title.CurrentNav = "nv-user-articles"
		case "tag":
			title.CurrentNav = "nv-tag-articles"
		case "reply":
			title.CurrentNav = "nv-replies"
		case "owa":
			title.CurrentNav = "nv-owa"
		}
	default:
		title.CurrentNav = "nv-" + fp
	}

	userLang := conf.GlobalServerConfig.GlobalDefaultLang
	if cookie, err := r.Cookie("upl"); err == nil {
		userLang = cookie.Value
		// } else {
		// 	mat := findPreferredLang.FindStringSubmatch(r.Header.Get("Accept-Language"))
		// 	if len(mat) > 1 {
		// 		userLang = strings.ToLower(mat[1])
		// 	}
	}

	if _, e := templates["header.html"].Content[userLang]; !e {
		userLang = conf.GlobalServerConfig.GlobalDefaultLang
	}

	t := template.Must(templates["header.html"].Content[userLang], nil)
	t.Execute(w, title)

	t = template.Must(templates[fp+".html"].Content[userLang], nil)
	t.Execute(w, pl)

	t = template.Must(templates["footer.html"].Content[userLang], nil)
	var payload struct {
		CurTime         string
		RunTime         string
		AvgRenderTime   string
		Hostname        string
		DatabaseVersion string
		Load            string

		CSRF string
	}

	payload.RunTime = fmt.Sprintf("%.1f", time.Now().Sub(ServerStartUp).Hours())
	payload.CurTime = templates[fp+".html"].Time.Format(time.RFC1123)

	if ServerTotalRenderCount > 0 {
		payload.AvgRenderTime = strconv.FormatFloat(
			float64(ServerTotalRenderTime/ServerTotalRenderCount)/1e6, 'f', 3, 64)
	}

	payload.CSRF = ""
	payload.Hostname = ServerHostname

	payload.Load = ServerLoad
	payload.DatabaseVersion = strings.Split(DatabaseVersion, ",")[0]

	t.Execute(w, payload)
}

func LoadTemplates() {
	templates = make(map[string]_Template)

	files, _ := ioutil.ReadDir("./templates")
	parse := func(f os.FileInfo) {
		temp, e := templates[f.Name()]
		if !e {
			temp = _Template{}
		}

		if temp.Content == nil {
			temp.Content = make(map[string]*template.Template)
		}

		temp.Time = f.ModTime()

		for lang, pret := range auth.TranslateTemplate("./templates/"+f.Name(), "./i18n.json") {
			t, _ := template.New(f.Name()).Parse(pret)
			temp.Content[lang] = t
		}

		templates[f.Name()] = temp
	}

	for _, f := range files {
		parse(f)
	}

	go func() {
		for {
			files, _ := ioutil.ReadDir("./templates")
			for _, f := range files {
				if f.ModTime().Unix() != templates[f.Name()].Time.Unix() {
					parse(f)
					glog.Infoln("Template reloaded:", f.Name())
				}
			}

			time.Sleep(30 * time.Second)
		}
	}()
}

func Return(w http.ResponseWriter, v interface{}) {
	switch v.(type) {
	case int:
		w.WriteHeader(v.(int))
	case []byte:
		w.Write(v.([]byte))
	case string:
		w.Write([]byte(v.(string)))
	default:
		buf, _ := json.Marshal(v)
		w.Write(buf)
	}
}
