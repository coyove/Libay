package conf

import (
	"database/sql"
	"encoding/json"
	"github.com/golang/glog"
	"html"
	"io/ioutil"
	"os"
	"strconv"
	"sync"
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

	MainJS  string
	MainCSS string

	AnonymousArea int
	ReplyArea     int
	MessageArea   int

	AllowRegistration bool
	ImagesAllowed     interface{}
	PostsAllowed      interface{}
	ArticlesPerPage   int
	Tags              interface{}
	AdminPassword     string
	MaxImageSize      int

	MaxArticleContentLength int
	MaxRevision             int
	PlaygroundMaxImages     int
	AccessLogging           bool
	Privilege               map[string]interface{}

	HTMLTags  map[string]bool
	HTMLAttrs map[string]bool

	MaxIdleConns int
	MaxOpenConns int

	CacheLifetime int
	CacheEntities int

	ConfigPath string

	sortedTags        map[int]string
	sortedTagsReverse map[string]int
	sortedTags2       map[int]Tag
	presetSqlQuery    string

	sync.RWMutex
}

type Tag struct {
	Name        string
	Description string
	Visible     bool
	Restricted  bool
	PermittedTo []string
	Short       string
	AnnounceID  int
}

func (sc *ServerConfig) GetTags() map[int]string {
	sc.RLock()
	defer sc.RUnlock()

	return sc.sortedTags
}

func (sc *ServerConfig) GetComplexTags() map[int]Tag {
	sc.RLock()
	defer sc.RUnlock()

	return sc.sortedTags2
}

func (sc *ServerConfig) GetPrivilege(group string, name string) bool {
	sc.RLock()
	defer sc.RUnlock()

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
	sc.RLock()
	defer sc.RUnlock()

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
	sc.RLock()
	defer sc.RUnlock()

	return sc.presetSqlQuery
}

func (sc *ServerConfig) InitTags(db *sql.DB) {
	sc.Lock()
	defer sc.Unlock()

	ret := make(map[int]string)
	retReverse := make(map[string]int)
	ret2 := make(map[int]Tag)

	sc.presetSqlQuery = ""

	rows, err := db.Query(`
        SELECT 
            id, 
            name, 
            description, 
            restricted, 
            hidden, 
            short,
            announce_id
        FROM 
            tags`)

	if err != nil {
		glog.Fatalln("Init tags failed")
		return
	}
	defer rows.Close()

	// for k, v := range list {
	for rows.Next() {
		var id, announceID int
		var hidden bool
		var name, description, restricted, short string

		rows.Scan(&id, &name, &description, &restricted, &hidden, &short, &announceID)

		t := Tag{}
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

		if hidden || id == sc.AnonymousArea || id == sc.ReplyArea || t.Restricted {
			sc.presetSqlQuery += (" AND tag != " + strconv.Itoa(id))
		}

		ret[id] = name
		retReverse[name] = id

		t.Name = name
		t.Description = html.UnescapeString(description)
		t.Visible = !hidden
		t.Short = short
		t.AnnounceID = announceID

		ret2[id] = t
	}
	sc.presetSqlQuery += " and tag <= 65536 "

	sc.sortedTags = ret
	sc.sortedTags2 = ret2
	sc.sortedTagsReverse = retReverse
}

func (sc *ServerConfig) GetTagIndex(t string) int {
	sc.RLock()
	defer sc.RUnlock()

	if _t, err := strconv.Atoi(t); _t >= 100000 && err == nil {
		return _t
	} else if err == nil && _t > 0 && _t <= 65536 {
		if _, ok := sc.sortedTags[_t]; ok {
			return _t
		}
	}

	if v, ok := sc.sortedTagsReverse[t]; ok {
		return v
	}

	return -1
}

func (sc *ServerConfig) GetIndexTag(t int) string {
	sc.RLock()
	defer sc.RUnlock()

	if t >= 100000 {
		return sc.sortedTags[sc.MessageArea]
	}

	if tag, e := sc.sortedTags[t]; e {
		return tag
	} else {
		return "tag" + strconv.Itoa(t)
	}
}

func (sc *ServerConfig) GetImagesAllowedGroups() []string {
	sc.RLock()
	defer sc.RUnlock()

	list := sc.ImagesAllowed.([]interface{})
	ret := make([]string, len(list))
	for i, v := range list {
		ret[i] = v.(string)
	}

	return ret
}

func (sc *ServerConfig) GetPostsAllowedGroups() []string {
	sc.RLock()
	defer sc.RUnlock()

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
