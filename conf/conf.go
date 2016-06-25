package conf

import (
	"database/sql"
	"encoding/json"
	"github.com/golang/glog"
	"html"
	"io/ioutil"
	"os"
	"strconv"
)

type ServerConfig struct {
	Connect string
	Salt    string
	Listen  string

	CDNPrefix string

	Host        string
	DebugHost   string
	Referer     string
	Description string
	Title       string
	Author      string
	Email       string

	AnonymousArea int
	ReplyArea     int
	MessageArea   int

	AllowRegistration       bool
	ImagesAllowed           interface{}
	PostsAllowed            interface{}
	ArticlesPerPage         int
	Tags                    interface{}
	AdminPassword           string
	MaxImageSize            int
	MaxImageSizeGuest       int
	MaxArticleContentLength int
	MaxRevision             int
	PlaygroundMaxImages     int
	AllowAnonymousUpload    bool
	Privilege               map[string]interface{}

	HTMLTags  map[string]bool
	HTMLAttrs map[string]bool

	MaxIdleConns int
	MaxOpenConns int

	CacheLifetime int
	CacheEntities int

	ConfigPath string

	sortedTags  map[int]string
	sortedTags2 map[int]Tag
	// sortedVisibleTags []string
	presetSqlQuery string
}

type Tag struct {
	Name        string
	Description string
	Visible     bool
	Restricted  bool
	PermittedTo []string
	Short       string
}

func (sc *ServerConfig) GetTags() map[int]string {
	return sc.sortedTags
}

func (sc *ServerConfig) GetComplexTags() map[int]Tag {
	return sc.sortedTags2
}

func (sc *ServerConfig) GetPrivilege(group string, name string) bool {
	if group == "admin" {
		return true
	}

	g, e := sc.Privilege[group]

	if !e {
		return false
	}

	return g.(map[string]interface{})[name].(bool)
}

func (sc *ServerConfig) GetInt(group string, name string) int {
	if group == "admin" {
		return 0
	}

	g, e := sc.Privilege[group]

	if !e {
		return 64
	}

	return int(g.(map[string]interface{})[name].(float64))
}

func (sc *ServerConfig) GetSQL() string {
	return sc.presetSqlQuery
}

func (sc *ServerConfig) InitTags(db *sql.DB) {
	// list := sc.Tags.(map[string]interface{})

	ret := make(map[int]string)
	ret2 := make(map[int]Tag)

	sc.presetSqlQuery = ""

	// buf, _ := json.Marshal(t.PermittedTo)
	rows, err := db.Query("SELECT id, name, description, restricted, hidden, short FROM tags;")

	if err != nil {
		glog.Fatalln("Init tags failed")
		return
	}
	defer rows.Close()

	// for k, v := range list {
	for rows.Next() {
		var id int
		var hidden bool
		var name, description, restricted, short string

		rows.Scan(&id, &name, &description, &restricted, &hidden, &short)

		if hidden || id == sc.AnonymousArea || id == sc.ReplyArea {
			sc.presetSqlQuery += (" AND tag != " + strconv.Itoa(id))
		}

		ret[id] = name
		t := Tag{}
		t.Name = name
		t.Description = html.UnescapeString(description) // _v["description"].(string)
		t.Visible = !hidden                              // !_v["hidden"].(bool)
		t.Short = short                                  // _v["short"].(string)
		// log.Println(id, name, restricted, hidden, short)

		// ra := _v["restricted"].([]interface{})
		var arr interface{}
		t.PermittedTo = make([]string, 0)
		json.Unmarshal([]byte(restricted), &arr)

		if arr == nil {
			t.Restricted = false
		} else {
			ra := arr.([]interface{})

			if (len(ra) == 1 && ra[0].(string) == "") || (len(ra) < 1) {
				t.Restricted = false
			} else {
				t.Restricted = true
				for _, v := range ra {
					t.PermittedTo = append(t.PermittedTo, v.(string))
				}
			}
		}

		ret2[id] = t
	}
	sc.presetSqlQuery += " and tag <= 65536 "

	sc.sortedTags = ret
	sc.sortedTags2 = ret2
	// sc.sortedVisibleTags = ret3
}

func (sc *ServerConfig) GetTagIndex(t string) int {
	_t, _ := strconv.Atoi(t)
	if _t >= 100000 {
		return _t
	}

	for k, v := range sc.sortedTags2 {
		if v.Name == t {
			return k
		}
	}

	return -1
}

func (sc *ServerConfig) GetIndexTag(t int) string {
	if t >= 100000 {
		return sc.sortedTags[sc.MessageArea]
	}

	return sc.sortedTags[t]
}

func (sc *ServerConfig) GetImagesAllowedGroups() []string {
	list := sc.ImagesAllowed.([]interface{})
	ret := make([]string, len(list))
	for i, v := range list {
		ret[i] = v.(string)
	}

	return ret
}

func (sc *ServerConfig) GetPostsAllowedGroups() []string {
	list := sc.PostsAllowed.([]interface{})
	ret := make([]string, len(list))
	for i, v := range list {
		ret[i] = v.(string)
	}

	return ret
}

var GlobalServerConfig ServerConfig

func LoadConfig(f string, db *sql.DB) {
	_conf, err := ioutil.ReadFile(f)
	if err != nil {
		glog.Fatalln("No config file found, exiting...")
		os.Exit(1)
	}

	err = json.Unmarshal(_conf, &GlobalServerConfig)
	if err != nil {
		glog.Fatalln("Invalid config file, exiting...", err)
		os.Exit(1)
	}
	if db != nil {
		GlobalServerConfig.InitTags(db)
	}
}
