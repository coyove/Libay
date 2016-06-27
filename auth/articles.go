package auth

import (
	"../conf"
	_ "database/sql"
	"fmt"
	"github.com/golang/glog"
	"github.com/gorilla/feeds"
	"html"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"time"
)

type Article struct {
	ID           int
	Title        string
	Tag          string
	Author       string
	AuthorID     int
	Content      string
	Timestamp    int
	ModTimestamp int
	Deleted      bool
	Locked       bool
	StayTop      bool
	Hits         int
	ParentID     int
	ParentTitle  string
	Children     int
	// EmptyContent bool
	IsRestricted bool
	IsMessage    bool
	Revision     int
}

type Message struct {
	ID           int
	Title        string
	Preview      string
	ReceiverID   int
	ReceiverName string
	SenderID     int
	SenderName   string
	Sentout      bool
	Timestamp    int
}

var articleCounter struct {
	sync.RWMutex
	articles map[int]int
}

func incrCounter(id int) {
	articleCounter.Lock()
	articleCounter.articles[id]++
	articleCounter.Unlock()
}

func ArticleCounter() {
	ticker := time.NewTicker(5 * time.Minute)
	articleCounter.articles = make(map[int]int)

	defer ticker.Stop()
	for {
		select {
		case <-ticker.C:
			articleCounter.Lock()
			var query string = ""
			for id, c := range articleCounter.articles {
				query += "UPDATE articles SET hits=hits+" + strconv.Itoa(c) + " WHERE id=" + strconv.Itoa(id) + ";"
				delete(articleCounter.articles, id)
			}

			_, err := Gdb.Exec(query)
			if err != nil {
				glog.Errorln("Database:", err)
			}

			articleCounter.Unlock()
		}
	}
}

var itoa = strconv.Itoa

func GetArticles(page int, filter string, filterType string) (ret []Article, totalArticles int) {
	ret = make([]Article, 0)

	cacheKey := fmt.Sprintf("%d-%s-%s", page, filter, filterType)
	if v, e := Gcache.Get(cacheKey); e {
		_v := v.([]interface{})
		return _v[0].([]Article), _v[1].(int)
	}

	defer func() {
		Gcache.Add(cacheKey, []interface{}{ret, totalArticles}, conf.GlobalServerConfig.CacheLifetime)
	}()

	_app := conf.GlobalServerConfig.ArticlesPerPage
	_start := _app * (page - 1)

	onlyTag := "articles.deleted = false"
	orderBy := "modified_at DESC"

	switch filterType {
	case "ua":
		if _, err := strconv.Atoi(filter); err == nil {
			/*
			   filter = user's ID

			   Average visitors/users trying to access someone's article-list,
			   Show them the ICEBERG and filter out all the hidden articles.
			*/
			onlyTag += " AND (articles.author = " + filter +
				" OR articles.original_author = " + filter + ") " +
				conf.GlobalServerConfig.GetSQL()
		} else {
			/*
			   filter is not a valid number
			*/
			return //ret, 0
		}
	case "tag":
		/*
		   filter = tag's name

		   GetTagIndex converts name to index, returns -1 if name is not found
		*/
		_index := conf.GlobalServerConfig.GetTagIndex(filter)
		onlyTag += " AND articles.tag = " + itoa(_index)
	case "owa":
		/*
		   filter = user-id:tag-id-1:tag-id-2:....:tag-id-n

		   "owa" means showing all articles, authentication was made in models/Page.go.
		   Signed user can view his own articles, users with "ViewOtherTrash" privilege can
		   view others' articles.
		*/
		_arr := strings.Split(filter, ":")
		if _arr[0] == "0" {
			/*
			   "0" means accessing all articles.
			*/
			onlyTag = "1 = 1"
		} else {
			// Note this overrides "articles.deleted=false"
			onlyTag = "(articles.author = " + _arr[0] +
				" OR articles.original_author = " + _arr[0] + ")"
		}
		for i := 1; i < len(_arr); i++ {
			_id, err := strconv.Atoi(_arr[i])
			if err != nil {
				return //ret, 0
			}

			if _id == conf.GlobalServerConfig.MessageArea {
				onlyTag += " AND articles.tag <= 65536"
			} else {
				onlyTag += " AND articles.tag != " + _arr[i]
			}
		}
	case "reply":
		if _, err := strconv.Atoi(filter); err == nil {
			// Replies should be sorted by CREATION date not by MODIFICATION date
			onlyTag += " AND articles.parent = " + filter
			orderBy = "created_at DESC"
		} else {
			return //ret, 0
		}
	default:
		onlyTag += conf.GlobalServerConfig.GetSQL()
	}

	if page == 1 {
		orderBy = "stay_top DESC," + orderBy
	}
	// log.Println(conf.GlobalServerConfig.GetSQL())
	rows, err := Gdb.Query(`
        SELECT
            articles.id, 
            articles.title, 
            articles.tag as tag, 
            articles.author, 
               users.nickname, 
            articles.preview,
            articles.created_at,
            articles.modified_at,
            articles.stay_top,
            articles.deleted,
            articles.hits,
            articles.children
        FROM
            articles 
        INNER JOIN 
            users ON users.id = articles.author
        WHERE
            ` + onlyTag + ` 
        ORDER BY
            ` + orderBy + ` 
        OFFSET ` + itoa(_start) + " LIMIT " + itoa(_app))

	if err != nil {
		glog.Errorln("Database:", err)
		return //ret, 0
	}

	defer rows.Close()

	for rows.Next() {
		var id, tag, authorID, hits, childrenCount int
		var title, author, preview string
		var createdAt, modifiedAt time.Time
		var stayTop, deleted bool

		rows.Scan(&id, &title, &tag, &authorID, &author, &preview,
			&createdAt, &modifiedAt,
			&stayTop, &deleted,
			&hits, &childrenCount)

		_tag := conf.GlobalServerConfig.GetIndexTag(tag)
		if tag <= 65536 {
			if conf.GlobalServerConfig.GetComplexTags()[tag].Restricted {
				preview = ""
			}
		}

		ret = append(ret, Article{id, title, _tag, author, authorID, preview,
			int(createdAt.Unix()),
			int(modifiedAt.Unix()),
			deleted,
			false,
			stayTop,
			hits, 0, "", childrenCount,
			false,
			false,
			0})
	}

	// var totalArticles int

	if Gdb.QueryRow(`
        SELECT COUNT(id)
        FROM   articles 
        WHERE `+onlyTag).Scan(&totalArticles) != nil {
		glog.Errorln("Database:", err)
		return // ret, 0
	}

	return // ret, totalArticles
}

func SearchArticles(r *http.Request, page int, filter string) []Article {
	ret := make([]Article, 0)
	return ret
}

func GetMessages(page int, userID int, lookupID int) ([]Message, int) {
	ret := make([]Message, 0)

	_app := conf.GlobalServerConfig.ArticlesPerPage
	_start := _app * (page - 1)

	onlyTag := fmt.Sprintf(" AND articles.tag >= 100000 AND (articles.author = %d OR articles.tag = %d) ",
		userID, userID+100000)

	if userID != lookupID {
		onlyTag += fmt.Sprintf(" AND (articles.author = %d OR articles.tag = %d) ", lookupID, lookupID+100000)
	}

	rows, err := Gdb.Query(`
        SELECT
            articles.id, 
            articles.title, 
            articles.preview,
            articles.tag as tag, 
                  u2.nickname, 
            articles.author, 
               users.nickname, 
            articles.created_at
        FROM
            articles 
        INNER JOIN
            users ON users.id = articles.author
        INNER JOIN 
            users AS u2 ON u2.id = articles.tag - 100000
        WHERE 
            articles.deleted = false ` + onlyTag + ` 
        ORDER BY
            created_at DESC 
        OFFSET ` + strconv.Itoa(_start) + " LIMIT " + strconv.Itoa(_app))

	if err != nil {
		glog.Errorln("Database:", err)
		return ret, 0
	}

	defer rows.Close()

	for rows.Next() {
		var id, senderID, receiverID int
		var title, senderName, receiverName, preview string
		var createdAt time.Time

		rows.Scan(&id, &title, &preview,
			&receiverID, &receiverName,
			&senderID, &senderName,
			&createdAt)

		ret = append(ret, Message{id, title, preview,
			receiverID - 100000, receiverName,
			senderID, senderName,
			senderID == userID,
			int(createdAt.Unix())})
	}

	var totalArticles int

	if Gdb.QueryRow(`
        SELECT COUNT(id)
        FROM   articles 
        WHERE  articles.deleted = false `+onlyTag).Scan(&totalArticles) != nil {
		glog.Errorln("Database:", err)
		return ret, 0
	}

	return ret, totalArticles
}

func GetArticle(r *http.Request, user AuthUser, id int, noEscape bool) (ret Article) {
	// var ret Article
	cacheKey := fmt.Sprintf("%d-%d-%v", user.ID, id, noEscape)

	if v, e := Gcache.Get(cacheKey); e {
		return v.(Article)
	}

	defer func() {
		Gcache.Add(cacheKey, ret, conf.GlobalServerConfig.CacheLifetime)
	}()

	if LogIPnv(r) {
		incrCounter(id)
	}

	rows, err := Gdb.Query(`
        SELECT 
            articles.id, 
            articles.title, 
            articles.tag, 
            articles.content, 
            articles.author, 
               users.nickname, 
            articles.created_at,
            articles.modified_at,
            articles.deleted,
            articles.locked,
            articles.stay_top,
            articles.hits,
            articles.parent,
            articles.children,
            articles.rev,
            (SELECT 
                sub.title 
            FROM 
                articles AS sub 
            WHERE 
                sub.id = articles.parent) AS parent_title
        FROM 
            articles 
        INNER JOIN 
            users ON users.id = articles.author
        WHERE 
            articles.id = ` + strconv.Itoa(id))

	if err != nil {
		glog.Errorln("Database:", err)
		return // ret
	}

	defer rows.Close()

	if rows.Next() {
		var id, tag, authorID, hits, parentID, childrenCount, revision int
		var title, content, author, parentTitle string
		var createdAt, modifiedAt time.Time
		var deleted, stayTop, locked bool

		rows.Scan(&id, &title, &tag, &content, &authorID,
			&author, &createdAt, &modifiedAt, &deleted, &locked,
			&stayTop, &hits, &parentID, &childrenCount, &revision,
			&parentTitle)

		if !noEscape {
			content = html.UnescapeString(content)
		} else {
			title = html.UnescapeString(title)
		}

		var _tag string
		if tag >= 100000 {
			_tag = strconv.Itoa(tag) // conf.GlobalServerConfig.GetIndexTag(tag)
		} else {
			_tag = conf.GlobalServerConfig.GetTags()[tag]
		}

		ret = Article{id, title, _tag, author, authorID, content,
			int(createdAt.Unix()), int(modifiedAt.Unix()),
			deleted, locked, stayTop, hits, parentID, parentTitle, childrenCount, false, false, revision}

		if !user.CanView(tag) {
			// content = "[ Restricted Contents to '" + user.Group + "' Group ]"
			ret.IsRestricted = true
			ret.Content = ""
		}

		if tag >= 100000 {
			// log.Println(authorID, tag, user.ID)
			if user.ID != authorID && user.ID != tag-100000 {
				ret.Content = ""
				ret.Title = "---"
				ret.IsMessage = true
			}
		}
	}

	return //ret
}

func InvertArticleState(id int, state string) string {
	_, err := Gdb.Exec(fmt.Sprintf("UPDATE articles SET %s = NOT %s WHERE id = %d", state, state, id))

	if err == nil {
		Gcache.Clear()
		return "ok"
	} else {
		glog.Errorln("Database:", err, id, state)
		return "Err::DB::Update_Failure"
	}
}

func GenerateRSS(atom bool, page int) string {
	now := time.Now()
	feed := &feeds.Feed{
		Title:       conf.GlobalServerConfig.Title,
		Link:        &feeds.Link{Href: conf.GlobalServerConfig.Host},
		Description: conf.GlobalServerConfig.Description,
		Author:      &feeds.Author{Name: conf.GlobalServerConfig.Author, Email: conf.GlobalServerConfig.Email},
		Created:     now,
	}

	feed.Items = make([]*feeds.Item, 0)

	a, _ := GetArticles(page, "", "")

	for _, v := range a {
		feed.Items = append(feed.Items, &feeds.Item{
			Title:       v.Title,
			Link:        &feeds.Link{Href: conf.GlobalServerConfig.Host + "/article/" + strconv.Itoa(v.ID)},
			Author:      &feeds.Author{Name: v.Author},
			Created:     time.Unix(int64(v.Timestamp), 0),
			Description: v.Content,
		})
	}

	if atom {
		ret, err := feed.ToAtom()
		if err != nil {
			glog.Errorln("Atom:", err)
		}

		return ret

	} else {

		ret, err := feed.ToRss()
		if err != nil {
			glog.Errorln("RSS:", err)
		}

		return ret
	}
}
