package models

import (
	// "../auth"
	"../conf"

	"github.com/golang/glog"
	"io/ioutil"
	"net/http"
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
	Content *template.Template
	Time    time.Time
}

var templates map[string]_Template

var ModelHandlerDummy ModelHandler
var ServerChecksum string
var ConfigChecksum string
var DatabaseVersion string
var ServerLoad string
var ServerLoadi float64
var ServerStartUp time.Time
var ServerTotalRenderTime int64
var ServerTotalRenderCount int64

func ServePage(w http.ResponseWriter, fp string, pl interface{}) {

	if fp == "404" {
		w.WriteHeader(404)
	}

	var title struct {
		Title      string
		URL        string
		CDN        string
		CurrentNav string
	}
	title.Title = conf.GlobalServerConfig.Title
	title.URL = conf.GlobalServerConfig.Host
	title.CDN = conf.GlobalServerConfig.CDNPrefix

	switch fp {
	case "404":
	case "about":
		title.CurrentNav = "nv-about"
	case "account":
		title.CurrentNav = "nv-console"
	case "article":
		title.CurrentNav = "nv-article"
	case "index":
		title.CurrentNav = "nv-index"
	case "articles":
		ps := pl.(PageStruct)
		// if ps.CurType == "" && ps.CurPage == 1 {

		// } else {
		switch ps.CurType {
		case "":
			title.CurrentNav = "nv-articles"
		case "ua":
			title.CurrentNav = "nv-user-articles"
		case "tag":
			title.CurrentNav = "nv-tag-articles"
		case "reply":
			title.CurrentNav = "nv-replies"
		case "message":
			title.CurrentNav = "nv-messages"
		case "owa":
			title.CurrentNav = "nv-owa"
		}
		// }
	case "bootstrap":
		title.CurrentNav = "nv-bootstrap"
	case "config":
		title.CurrentNav = "nv-config"
	case "database":
		title.CurrentNav = "nv-database"
	case "editor":
		title.CurrentNav = "nv-new-article"
	case "footer":
	case "header":
	case "list":
	case "playground":
		title.CurrentNav = "nv-playground"
	case "register":
		title.CurrentNav = "nv-register"
	case "tags":
		title.CurrentNav = "nv-tags"
	case "user":
	}

	t := template.Must(templates["header.html"].Content, nil)
	t.Execute(w, title)

	t = template.Must(templates[fp+".html"].Content, nil)
	t.Execute(w, pl)

	t = template.Must(templates["footer.html"].Content, nil)
	var payload struct {
		CurTime         string
		RunTime         string
		AvgRenderTime   string
		Checksum        string
		ConfigChecksum  string
		DatabaseVersion string
		Load            string
		AvgLoad         string
		CSRF            string
	}

	runTime := time.Now().Sub(ServerStartUp).Minutes()
	payload.RunTime = strconv.Itoa(int(runTime)) + "." + strconv.Itoa(int(time.Now().Second()/10))
	payload.CurTime = templates[fp+".html"].Time.Format(time.RFC1123)
	payload.AvgLoad = strconv.FormatFloat(ServerLoadi/runTime, 'f', 2, 64)
	if ServerTotalRenderCount > 0 {
		payload.AvgRenderTime = strconv.FormatFloat(float64(ServerTotalRenderTime/ServerTotalRenderCount)/1e6, 'f', 6, 64)
	}

	// if fp == "editor" || fp == "list" || fp == "account" ||
	// 	fp == "user" || fp == "register" || fp == "article" ||
	// 	fp == "playground" || fp == "database" || fp == "config" ||
	// 	fp == "bootstrap" {
	// s := strconv.Itoa(int(time.Now().Unix()))
	payload.CSRF = ""
	// }

	payload.Checksum = ServerChecksum
	payload.ConfigChecksum = ConfigChecksum
	payload.Load = ServerLoad
	payload.DatabaseVersion = strings.Split(DatabaseVersion, ",")[0]

	t.Execute(w, payload)
}

func LoadTemplates() {
	templates = make(map[string]_Template)

	files, _ := ioutil.ReadDir("./templates")
	for _, f := range files {
		// buf, _ := ioutil.ReadFile("./templates/" + f.Name())
		t, _ := template.ParseFiles("./templates/" + f.Name())
		templates[f.Name()] = _Template{t, f.ModTime()}
	}

	go func() {
		for {
			files, _ := ioutil.ReadDir("./templates")
			for _, f := range files {
				if f.ModTime().Unix() != templates[f.Name()].Time.Unix() {
					t, _ := template.ParseFiles("./templates/" + f.Name())
					templates[f.Name()] = _Template{t, f.ModTime()}
					glog.Infoln("Template reloaded:", f.Name())
				}
			}

			time.Sleep(30 * time.Second)
		}
	}()
}
