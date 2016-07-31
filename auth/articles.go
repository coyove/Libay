package auth

import (
	"../conf"

	"github.com/golang/glog"
	"github.com/gorilla/feeds"

	_ "database/sql"
	"encoding/binary"
	"fmt"
	"html"
	"net/http"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"time"
)

type Article struct {
	ID               int
	Title            string
	Tag              string
	Author           string
	AuthorID         int
	OriginalAuthorID int
	OriginalAuthor   string
	Content          string
	Timestamp        int
	ModTimestamp     int
	Deleted          bool
	Locked           bool
	Read             bool
	Hits             int
	ParentID         int
	ParentTitle      string
	Children         int
	Revision         int

	IsRestricted     bool
	IsOthersMessage  bool
	IsMessage        bool
	IsMessageSentout bool
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
	Read         bool
}

type BackForth struct {
	NextPage string
	PrevPage string

	LastWeekPage  string
	LastMonthPage string
	LastYearPage  string

	NextWeekPage  string
	NextMonthPage string
	NextYearPage  string

	Range struct {
		Start int
		End   int
	}
}

var articleCounter struct {
	sync.RWMutex
	articles map[int]int
}

var itoa = strconv.Itoa

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
				query += "UPDATE articles SET hits = hits + " + itoa(c) + " WHERE id=" + itoa(id) + ";"
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

func HashTS(ts int) string {
	buf := MakeHashRaw(ts)

	return To60(uint64(binary.BigEndian.Uint32(buf[:4])))[:3]
}

func ExtractTS(enc string) (string, string, int, bool) {
	switch enc {
	case "1":
		return "DESC", "<", int(time.Now().UnixNano() / 1e6), false
	case "last":
		return "ASC", ">", 0, false
	default:
		matches := tsReg.FindStringSubmatch(enc)
		if len(matches) != 4 {
			return "", "", 0, true
		}

		verify := matches[2]
		direction, ok := IIF{"before": "DESC", "after": "ASC"}[matches[1]]
		if !ok {
			return "", "", 0, true
		}
		ts := From60(matches[3])

		if HashTS(ts) != verify {
			return "", "", 0, true
		} else {
			return direction, IIF{true: "<", false: ">"}[direction == "DESC"], ts, false
		}
	}
}

func (bf *BackForth) Set(prev, next int) {
	make1 := func(t int) string {
		return fmt.Sprintf("before=%s_%s", HashTS(t), To60(uint64(t)))
	}

	make2 := func(t int) string {
		return fmt.Sprintf("after=%s_%s", HashTS(t), To60(uint64(t)))
	}

	bf.PrevPage = make2(prev)
	bf.NextPage = make1(next)

	bf.LastWeekPage = make1(prev - 3600000*24*7)
	bf.LastMonthPage = make1(prev - 3600000*24*30)
	bf.LastYearPage = make1(prev - 3600000*24*365)

	bf.NextWeekPage = make2(prev + 3600000*24*7)
	bf.NextMonthPage = make2(prev + 3600000*24*30)
	bf.NextYearPage = make2(prev + 3600000*24*365)

	bf.Range.Start = next
	bf.Range.End = prev
}

func GetArticles(enc string, filter string, filterType string) (ret []Article, nav BackForth) {
	ret = make([]Article, 0)

	direction, compare, ts, invalid := ExtractTS(enc)
	if invalid {
		return
	}

	nav.Set(ts, ts)

	cacheKey := fmt.Sprintf("%s-%s-%s", enc, filter, filterType)
	if v, e := Gcache.Get(cacheKey); e {
		_v := v.([]interface{})
		return _v[0].([]Article), _v[1].(BackForth)
	}

	defer func() {
		Gcache.Add(cacheKey, []interface{}{ret, nav}, conf.GlobalServerConfig.CacheLifetime)
	}()

	onlyTag := "articles.deleted = false"
	orderByDate := "modified_at"
	if filterType == "reply" {
		// Replies should be sorted by CREATION date not by MODIFICATION date
		orderByDate = "created_at"
	}
	orderBy := orderByDate + " " + direction

	switch filterType {
	case "ua":
		if _, err := strconv.Atoi(filter); err == nil {
			/*
			   filter = user's ID
			   Show the ICEBERG and filter out all the hidden articles.
			*/
			onlyTag += " AND (articles.author = " + filter +
				" OR articles.original_author = " + filter + ") " +
				conf.GlobalServerConfig.GetSQL()
		} else {
			return
		}
	case "tag":
		/*
		   filter = tag name
		   Show the articles with the specific tag name
		   GetTagIndex converts name to index, returns -1 if name is not found
		*/
		_index := conf.GlobalServerConfig.GetTagIndex(filter)
		onlyTag += " AND articles.tag = " + itoa(_index)
	case "owa":
		/*
		   filter = user-id:tag-id-1:tag-id-2:....:tag-id-n
		   "owa" means showing all articles, authentication was made in models/Page.go.
		   Loggedin user can view his own articles, users with "ViewOtherTrash" privilege can
		   view others' articles.
		*/
		_arr := strings.Split(filter, ":")
		if _arr[0] == "0" {
			/*
			   "0" means accessing all articles.
			*/
			onlyTag = "1 = 1"
		} else {
			// Both the original author and the current author will see it
			onlyTag = "(articles.author = " + _arr[0] +
				" OR articles.original_author = " + _arr[0] + ")"
		}
		for i := 1; i < len(_arr); i++ {
			_id, err := strconv.Atoi(_arr[i])
			if err != nil {
				return
			}

			if _id == conf.GlobalServerConfig.MessageArea {
				onlyTag += " AND articles.tag <= 65536"
			} else {
				onlyTag += " AND articles.tag != " + _arr[i]
			}
		}
	case "reply":
		if _, err := strconv.Atoi(filter); err == nil {
			/*
				filter = ID of the parent article, all replies are children of it
			*/
			onlyTag += " AND articles.parent = " + filter
		} else {
			return
		}
	default:
		onlyTag += conf.GlobalServerConfig.GetSQL()
	}

	_start := time.Now()
	rows, err := Gdb.Query(`
        SELECT
            articles.id, 
            articles.title, 
            articles.tag as tag, 
            articles.author, 
      COALESCE(users.nickname, 'user' || articles.author::TEXT),
            articles.preview,
            articles.created_at,
            articles.modified_at,
            articles.deleted,
            articles.hits,
            articles.children
        FROM
            articles 
        LEFT JOIN 
            users ON users.id = articles.author
        WHERE
            ` + orderByDate + compare + itoa(ts) + ` AND
            (` + onlyTag + `)
        ORDER BY
            ` + orderBy + ` 
        LIMIT ` + itoa(conf.GlobalServerConfig.ArticlesPerPage))

	if err != nil {
		glog.Errorln("Database:", err)
		return
	}

	GarticleTimer.Push(time.Now().Sub(_start).Nanoseconds())

	defer rows.Close()

	for rows.Next() {
		var id, tag, authorID, hits, childrenCount int
		var title, author, preview string
		var createdAt, modifiedAt int
		var deleted bool

		rows.Scan(&id, &title, &tag, &authorID, &author, &preview,
			&createdAt, &modifiedAt,
			&deleted,
			&hits, &childrenCount)

		_tag := conf.GlobalServerConfig.GetIndexTag(tag)
		if tag <= 65536 {
			if conf.GlobalServerConfig.GetComplexTags()[tag].Restricted {
				preview = ""
			}
		}

		ret = append(ret, Article{
			ID:           id,
			Title:        title,
			Tag:          _tag,
			Author:       author,
			AuthorID:     authorID,
			Content:      preview,
			Timestamp:    createdAt,
			ModTimestamp: modifiedAt,
			Deleted:      deleted,
			Hits:         hits,
			Children:     childrenCount,
		})
	}

	if direction == "ASC" {
		for i, j := 0, len(ret)-1; i < j; i, j = i+1, j-1 {
			ret[i], ret[j] = ret[j], ret[i]
		}
	}

	if len(ret) > 0 {
		first := ret[0]
		last := ret[len(ret)-1]

		nav.Set(first.ModTimestamp, last.ModTimestamp)
	}

	return
}

func GetMessages(enc string, userID int, lookupID int) (ret []Message, nav BackForth) {
	ret = make([]Message, 0)

	direction, compare, ts, invalid := ExtractTS(enc)
	if invalid {
		return
	}

	nav.Set(ts, ts)

	onlyTag := fmt.Sprintf(" AND articles.tag >= 100000 AND (articles.author = %d OR articles.tag = %d) ",
		userID, userID+100000)

	if userID != lookupID {
		onlyTag += fmt.Sprintf(" AND (articles.author = %d OR articles.tag = %d) ", lookupID, lookupID+100000)
	}

	messageLimit := int(time.Now().UnixNano()/1e6 - 3600000*24*365)

	_start := time.Now()
	rows, err := Gdb.Query(`
        SELECT
            articles.id, 
            articles.title, 
            articles.preview,
            articles.read,
            articles.tag as tag, 
         COALESCE(u2.nickname, 'user' || (articles.tag - 100000)::TEXT), 
            articles.author, 
      COALESCE(users.nickname, 'user' || articles.author::TEXT),
            articles.created_at
        FROM
            articles 
        LEFT JOIN
            users ON users.id = articles.author
        LEFT JOIN 
            users AS u2 ON u2.id = articles.tag - 100000
        WHERE 
            created_at ` + compare + itoa(ts) + ` AND
            created_at > ` + itoa(messageLimit) + ` AND
            (articles.deleted = false ` + onlyTag + ` )
        ORDER BY
            created_at ` + direction + ` 
        LIMIT ` + itoa(conf.GlobalServerConfig.ArticlesPerPage))

	if err != nil {
		glog.Errorln("Database:", err)
		return
	}

	GmessageTimer.Push(time.Now().Sub(_start).Nanoseconds())

	defer rows.Close()

	for rows.Next() {
		var id, senderID, receiverID, createdAt int
		var title, senderName, receiverName, preview string
		var read bool

		rows.Scan(&id, &title, &preview, &read,
			&receiverID, &receiverName,
			&senderID, &senderName,
			&createdAt)

		ret = append(ret, Message{id, title, preview,
			receiverID - 100000, receiverName,
			senderID, senderName,
			senderID == userID,
			createdAt, read})
	}

	if direction == "ASC" {
		for i, j := 0, len(ret)-1; i < j; i, j = i+1, j-1 {
			ret[i], ret[j] = ret[j], ret[i]
		}
	}

	if len(ret) > 0 {
		nav.Set(ret[0].Timestamp, ret[len(ret)-1].Timestamp)
	}

	return
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
        UPDATE articles
        SET    read = true
        WHERE  
            id = ` + itoa(id) + ` 
        AND tag = ` + itoa(user.ID+100000) + `;

        SELECT 
            articles.id, 
            articles.title, 
            articles.tag, 
            articles.content, 
            articles.author, 
      COALESCE(users.nickname, 'user' || articles.author::TEXT),
            articles.original_author,
         COALESCE(ou.nickname, 'user' || articles.original_author::TEXT),
            articles.created_at,
            articles.modified_at,
            articles.deleted,
            articles.locked,
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
        LEFT JOIN 
            users ON users.id = articles.author
        LEFT JOIN 
            users as ou ON ou.id = articles.original_author
        WHERE 
            articles.id = ` + itoa(id))

	if err != nil {
		glog.Errorln("Database:", err)
		return // ret
	}

	defer rows.Close()

	if rows.Next() {
		var id, tag, authorID, originalAuthorID, hits, parentID, childrenCount, revision int
		var title, content, author, originalAuthor, parentTitle string
		var createdAt, modifiedAt int
		var deleted, locked bool

		rows.Scan(&id, &title, &tag, &content, &authorID, &author,
			&originalAuthorID, &originalAuthor,
			&createdAt, &modifiedAt, &deleted, &locked,
			&hits, &parentID, &childrenCount, &revision,
			&parentTitle)

		if !noEscape {
			content = html.UnescapeString(content)
			// No need to unescape title here
			// title = html.UnescapeString(title)
		}

		var _tag string
		_tag = conf.GlobalServerConfig.GetIndexTag(tag)

		ret = Article{
			ID:               id,
			Title:            title,
			Tag:              _tag,
			Author:           author,
			AuthorID:         authorID,
			OriginalAuthor:   originalAuthor,
			OriginalAuthorID: originalAuthorID,
			Content:          content,
			Timestamp:        createdAt,
			ModTimestamp:     modifiedAt,
			Deleted:          deleted,
			Locked:           locked,
			Hits:             hits,
			ParentID:         parentID,
			ParentTitle:      parentTitle,
			Children:         childrenCount,
			Revision:         revision,
		}

		if !user.CanView(tag) {
			// content = "[ Restricted Contents to '" + user.Group + "' Group ]"
			ret.IsRestricted = true
			ret.Content = ""
		}

		if tag >= 100000 {
			ret.IsMessage = true

			if user.ID == authorID {
				ret.IsMessageSentout = true
			} else if user.ID == tag-100000 {
				ret.IsMessageSentout = false
			} else {
				ret.IsOthersMessage = true
			}
		}
	}

	return //ret
}

func InvertArticleState(user AuthUser, id int, state string) string {
	var _tag, author, oauthor int

	err := Gdb.QueryRow(`SELECT tag, author, original_author FROM articles WHERE id = `+itoa(id)).
		Scan(&_tag, &author, &oauthor)

	if err != nil {
		glog.Errorln("Database:", err, id, state)
		return "Err::DB::Select_Failure"
	}

	if _tag >= 100000 && state == "deleted" {
		// Sender wants to delete the message, after deletion, sender = anonymous
		if user.ID == author {
			_, err = Gdb.Exec(`UPDATE articles SET author = 0 WHERE id = ` + itoa(id))
		}

		// Receiver wants to delete the message, after deletion, receiver = anonymous
		if user.ID == _tag-100000 {
			_, err = Gdb.Exec(`UPDATE articles SET tag = 100000 WHERE id = ` + itoa(id))
		}

		if err == nil {
			glog.Infoln(user.Name, user.NickName, "deleted", id)
			Gcache.Remove(`\d+-` + itoa(id) + `-(true|false)`)
			return "ok"
		} else {
			return "Err::DB::Update_Failure"
		}
	}

	tag := conf.GlobalServerConfig.GetIndexTag(_tag)

	_, err = Gdb.Exec(fmt.Sprintf(`UPDATE articles SET %s = NOT %s WHERE id = %d;`, state, state, id))

	if err == nil {
		pattern := fmt.Sprintf(`(\d+-(%s)-tag|\d+-(%d|%d)-ua|\d+-(%d|0).*-owa|\d+-(%d|0).*-owa|\d+--|\d+-%d-(true|false))`,
			regexp.QuoteMeta(tag),
			author, oauthor,
			author, oauthor,
			id,
		)

		Gcache.Remove(pattern)
		glog.Infoln(user.Name, user.NickName, "inverted", state, "of", id)
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

	a, _ := GetArticles("1", "", "")

	for _, v := range a {
		feed.Items = append(feed.Items, &feeds.Item{
			Title:       v.Title,
			Link:        &feeds.Link{Href: conf.GlobalServerConfig.Host + "/article/" + itoa(v.ID)},
			Author:      &feeds.Author{Name: v.Author},
			Created:     time.Unix(int64(v.Timestamp)/1000, 0),
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
