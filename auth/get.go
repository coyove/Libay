package auth

import (
	"../conf"

	"github.com/golang/glog"

	_ "database/sql"
	"fmt"
	"html"
	"net/http"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"time"
)

var articleCounter struct {
	sync.RWMutex
	articles map[int]int
	images   map[string]int
}

func IncrArticleCounter(id int) {
	articleCounter.Lock()
	articleCounter.articles[id]++
	articleCounter.Unlock()
}

func IncrImageCounter(img string) {
	articleCounter.Lock()
	articleCounter.images[img]++
	articleCounter.Unlock()
}

func ArticleCounter() {
	ticker := time.NewTicker(5 * time.Minute)
	articleCounter.articles = make(map[int]int)
	articleCounter.images = make(map[string]int)

	defer ticker.Stop()
	for {
		select {
		case <-ticker.C:
			articleCounter.Lock()
			var query string = ""
			for id, c := range articleCounter.articles {
				query += "UPDATE articles SET hits = hits + " + itoa(c) + " WHERE id = " + itoa(id) + ";"
				delete(articleCounter.articles, id)
			}

			for img, c := range articleCounter.images {
				query += "UPDATE images SET hits = hits + " + itoa(c) + " WHERE image = '" + img + "';"
				delete(articleCounter.images, img)
			}

			now := time.Now()
			if now.Hour()%2 == 0 && now.Minute() >= 7 && now.Minute() < 12 {
				query += `
				UPDATE tags SET children = sub.count FROM (
					SELECT COUNT(id) AS count, MAX(tag) AS tag FROM articles 
					WHERE tag <= 65536 GROUP BY tag) AS sub 
				WHERE tags.id = sub.tag;`
			}

			if _, err := Gdb.Exec(query); err != nil {
				glog.Errorln("Database:", err)
			}

			conf.GlobalServerConfig.InitTags(Gdb)

			articleCounter.Unlock()
		}
	}
}

func GetArticles(enc, filter, filterType, searchPattern string) (ret []Article, nav BackForth) {

	ret = make([]Article, 0)

	direction, compare, ts, invalid := ExtractTS(enc)
	if invalid {
		return
	}

	nav.Set(ts, ts)

	cacheKey := fmt.Sprintf("%s-%s-%s%s", enc, filter, filterType, searchPattern)
	if v, e := Gcache.Get(cacheKey); e {
		_v := v.([]interface{})
		return _v[0].([]Article), _v[1].(BackForth)
	}

	defer func() {
		Gcache.Add(cacheKey, []interface{}{ret, nav}, conf.GlobalServerConfig.CacheLifetime)
	}()

	// searchPattern escaping should be done elsewhere
	searchStat := "1 = 1"
	if searchPattern != "" {
		searchStat = "(vector @@ to_tsquery('" + conf.GlobalServerConfig.Zhparser + "', '" + searchPattern + `')
			OR title LIKE '%` + searchPattern + `%' OR raw LIKE '%` + searchPattern + `%')`
	}

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
	sql := `
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
            ` + orderByDate + compare + itoa(ts) + ` AND ` + searchStat + ` AND
            (` + onlyTag + `)
        ORDER BY
            ` + orderBy + ` 
        LIMIT ` + itoa(conf.GlobalServerConfig.ArticlesPerPage)

	rows, err := Gdb.Query(sql)
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

		if filterType == "reply" {
			nav.Set(first.Timestamp, last.Timestamp)
		} else {
			nav.Set(first.ModTimestamp, last.ModTimestamp)
		}
	}

	return
}

func GetMessages(enc string, userID, lookupID int) (ret []Message, nav BackForth) {
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

func GetGallery(enc string, user, galleryUser AuthUser, searchPattern string) (ret []Image, nav BackForth) {
	ret = make([]Image, 0)

	direction, compare, ts, invalid := ExtractTS(enc)
	if invalid {
		return
	}

	nav.Set(ts, ts)

	isSelf := user.ID == galleryUser.ID || conf.GlobalServerConfig.GetPrivilege(user.Group, "ViewOthers")
	if user.ID == 0 {
		isSelf = false
	}

	visible := isSelf || galleryUser.GalleryVisible == "all" || (user.Group != "" &&
		regexp.MustCompile(`(^|\s)`+user.Group+`(\s|$)`).MatchString(galleryUser.GalleryVisible))

	if galleryUser.ID == 0 {
		visible = isSelf
	}

	cacheKey := fmt.Sprintf("%s-%d-img%v%s", enc, galleryUser.ID, visible, searchPattern)
	if v, e := Gcache.Get(cacheKey); e {
		_v := v.([]interface{})
		return _v[0].([]Image), _v[1].(BackForth)
	}

	defer func() {
		Gcache.Add(cacheKey, []interface{}{ret, nav}, conf.GlobalServerConfig.CacheLifetime)
	}()

	showHidden := " AND hide = false"
	if visible {
		showHidden = ""
	}

	tester := ` AND uploader = ` + itoa(galleryUser.ID)
	if galleryUser.ID == 0 {
		tester = ""
	}

	searcher := ""
	if searchPattern != "" {
		for _, word := range strings.Split(searchPattern, " ") {
			searcher += " AND filename LIKE '%" + word + "%'"
		}
	}

	_start := time.Now()
	rows, err := Gdb.Query(`
        SELECT
            images.id, 
            images.image, 
            images.filename, 
            images.uploader, 
            images.ts, 
            images.hits, 
            images.hide,
            images.r18,
            images.size,
            COALESCE(users.nickname, 'user' || images.uploader::TEXT)
        FROM
            images
        LEFT JOIN
            users ON users.id = images.uploader
        WHERE 
            ts ` + compare + itoa(ts) + tester + showHidden + searcher + `
        ORDER BY
            ts ` + direction + ` 
        LIMIT ` + itoa(conf.GlobalServerConfig.ArticlesPerPage))

	if err != nil {
		glog.Errorln("Database:", err)
		return
	}

	GarticleTimer.Push(time.Now().Sub(_start).Nanoseconds())

	defer rows.Close()

	for rows.Next() {
		var id, uploaderID, timestamp, hits, size int
		var path, filename, uploader, shortName string
		var hide, r18 bool
		rows.Scan(&id, &path, &filename, &uploaderID, &timestamp, &hits, &hide, &r18, &size, &uploader)

		shortName = filename
		if len(filename) > 16 {
			shortName = Shorten(filename)
		}

		ret = append(ret, Image{
			id,
			uploaderID,
			uploader,
			conf.GlobalServerConfig.ImageHost + "/" + path,
			conf.GlobalServerConfig.ImageHost + "/small-" + path,
			Escape(filename),
			Escape(shortName),
			timestamp,
			hits,
			hide,
			r18,
			size,
		})
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

	if r != nil && LogIPnv(r) {
		IncrArticleCounter(id)
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
            articles.raw,
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
		var title, content, raw, author, originalAuthor, parentTitle string
		var createdAt, modifiedAt int
		var deleted, locked bool

		rows.Scan(&id, &title, &tag, &content, &raw, &authorID, &author,
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
			TagID:            tag,
			Author:           author,
			AuthorID:         authorID,
			OriginalAuthor:   originalAuthor,
			OriginalAuthorID: originalAuthorID,
			Content:          content,
			Raw:              raw,
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
			ret.IsRestricted = true
			ret.Content = ""
		}

		if tag >= 100000 {
			ret.IsMessage = true

			op := GetUserByID(tag - 100000)
			if op.ID != 0 {
				ret.Tag = op.NickName
			} else {
				ret.Tag = "user" + strconv.Itoa(tag-100000)
			}

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
