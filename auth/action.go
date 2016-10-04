package auth

import (
	"../conf"

	"github.com/golang/glog"

	_ "database/sql"
	"fmt"
	"net/http"
	"regexp"
	"strconv"
	"strings"
	"time"
)

func NewArticle(r *http.Request, user AuthUser, id int, tag string, title string, content string) string {
	_tag, err := strconv.Atoi(tag)
	if err != nil {
		_tag = conf.GlobalServerConfig.GetTagIndex(tag)
	}

	_title := Escape(title)
	_content, _preview, errs := BBCodeToHTML(content)

	if len(errs) > 0 {
		return "Err::Post::BBCode_Error"
	}

	if user.ID == 0 {
		ip := strings.Split(GetIP(r), ".")

		if len(ip) >= 4 {
			_content = "<div>[ IP: " + strings.Join(ip[:3], ".") + ".* ]</div>" + _content
		} else {
			return "Err::Post::Cannot_Get_IP"
		}
	}

	_content = Escape(_content)
	_preview = Escape(_preview)
	_raw := Escape(content)

	if _tag == -1 {
		return "Err::Post::Invalid_Tag"
	}

	if !user.CanView(_tag) {
		return "Err::Post::Restricted_Tag"
	}

	if user.ID == 0 && (_tag != conf.GlobalServerConfig.AnonymousArea &&
		_tag != conf.GlobalServerConfig.ReplyArea) {
		return "Err::Post::Invalid_Tag_For_Anonymous"
	}

	if _tag == conf.GlobalServerConfig.MessageArea {
		if conf.GlobalServerConfig.GetPrivilege(user.Group, "ForbidPM") || user.Status == "locked" {
			return "Err::Post::Cannot_Send_PM"
		}

		_tag = id + 100000
		id = 0
	}

	cooldown := conf.GlobalServerConfig.GetInt(user.Group, "Cooldown")
	_now := time.Now().UnixNano() / 1e6

	sql := `SELECT 
               new_article('%s', %d, '%s', '%s', '%s', %d, %d, %d, %d, %d);`
	sql = fmt.Sprintf(sql, _title, _tag, _content, _raw, _preview, _now, _now, user.ID, id, cooldown)

	var succ int

	if err := Gdb.QueryRow(sql).Scan(&succ); err == nil {

		if succ == 0 {
			// \d+\-%s\-tag -> tag
			// \d+\-%d\-ua -> user
			// \d+\-(%d|0).+\-owa -> owa
			// \d+\-%d\-reply -> reply
			Gcache.Remove(fmt.Sprintf(`(.+-%s-tag|.+-%d-ua|.+-(%d|0).*-owa|.+-%d-reply|.+--|.+-%d-(true|false))`,
				regexp.QuoteMeta(tag),
				user.ID,
				user.ID,
				id,
				id,
			))
			// Gcache.Clear()
			return "ok::" + tag
		} else {
			return "Err::Post::Cooldown_" + strconv.Itoa(cooldown-succ/1e3) + "s"
		}
	} else {
		glog.Errorln("Database:", err)
		return "Err::DB::General_Failure"
	}
}

func UpdateArticle(user AuthUser, id int, tag string, title string, content string) string {
	_tag, err := strconv.Atoi(tag)
	if err != nil {
		_tag = conf.GlobalServerConfig.GetTagIndex(tag)
	}

	_title := Escape(title)
	_content, _preview, errs := BBCodeToHTML(content)

	if len(errs) > 0 {
		return "Err::Post::BBCode_Error"
	}

	_preview = Escape(_preview)
	_content = Escape(_content)
	_raw := Escape(content)

	if _tag == -1 {
		return "Err::Post::Invalid_Tag"
	}

	old := GetArticle(nil, DummyUsers[0], id, true)
	if old.ID == 0 {
		return "Err::DB::Select_Failure"
	}

	if old.Revision >= conf.GlobalServerConfig.MaxRevision {
		old.Locked = true
		Gdb.Exec(`UPDATE articles SET locked = true WHERE id = ` + itoa(id))
	}

	if old.AuthorID != user.ID &&
		old.OriginalAuthorID != user.ID &&
		!conf.GlobalServerConfig.GetPrivilege(user.Group, "EditOthers") {
		return "Err::Privil::Edit_Action_Denied"
	}

	if 0 == user.ID {
		return "Err::Privil::Edit_Action_Denied"
	}

	if old.Locked && !conf.GlobalServerConfig.GetPrivilege(user.Group, "MakeLocked") {
		return "Err::Post::Locked_Article"
	}

	cooldown := conf.GlobalServerConfig.GetInt(user.Group, "Cooldown")

	sql := `SELECT update_article(%d, '%s', %d, %d, '%s', '%s', '%s', %d, 
                                    '%s', %d, '%s', '%s', %d, %d)`

	sql = fmt.Sprintf(sql, id,
		_title, _tag, user.ID, _content, _raw, _preview, time.Now().UnixNano()/1e6,
		old.Title, old.AuthorID, old.Content, old.Raw, old.ModTimestamp, cooldown)

	var succ int

	if err := Gdb.QueryRow(sql).Scan(&succ); err == nil {
		// row.Close()
		if succ == 0 {

			Gcache.Remove(fmt.Sprintf(`(.+-(%s|%s)-tag|.+-(%d|%d)-ua|.+-((%d|0).*|(%d|0).*)-owa|.+--|.+-%d-(true|false)|.+-%d-reply)`,
				regexp.QuoteMeta(tag), regexp.QuoteMeta(old.Tag),
				user.ID, old.OriginalAuthorID,
				user.ID, old.OriginalAuthorID,
				id, old.ParentID,
			))
			return "ok::" + itoa(id)
		} else {
			return "Err::Post::Cooldown_" + itoa(cooldown-succ/1e3) + "s"
		}
	} else {
		glog.Errorln("Database:", err)
		return "Err::DB::General_Failure"
	}
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
		pattern := fmt.Sprintf(`(\S+-(%s)-tag|\S+-(%d|%d)-ua|\S+-(%d|0).*-owa|\S+-(%d|0).*-owa|\S+--|\d+-%d-(true|false))`,
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
