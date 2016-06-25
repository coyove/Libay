package auth

import (
	"../conf"
	_ "database/sql"

	"github.com/golang/glog"
	"html"
	"net/http"
	"strconv"
	"strings"
	"time"
)

type AuthUser struct {
	ID            int
	Name          string
	NickName      string
	LastLoginDate int
	SignUpDate    int
	LastLoginIP   string
	Status        string
	Group         string
	Expired       bool
	Comment       string
	Avatar        string
	ImageUsage    int
	Unread        string
}

func (au *AuthUser) CanPost() bool {
	if au.Status == "locked" {
		return false
	}

	g := conf.GlobalServerConfig.GetPostsAllowedGroups()
	for _, v := range g {
		if v == au.Group {
			return true
		}
	}

	return false
}

func (au *AuthUser) CanPostImages() bool {
	if au.Status == "locked" {
		return false
	}

	g := conf.GlobalServerConfig.GetImagesAllowedGroups()
	for _, v := range g {
		if v == au.Group {
			return true
		}
	}

	return false
}

func (au *AuthUser) CanView(tag int) bool {
	if au.Status == "locked" {
		return false
	}

	_tag := conf.GlobalServerConfig.GetComplexTags()[tag]

	if !_tag.Restricted {
		return true
	}

	for _, v := range _tag.PermittedTo {
		if v == au.Group {
			return true
		}
	}
	return false
}

func CheckCSRF(r *http.Request) bool {
	referer := (r.Header.Get("Referer"))
	if strings.HasPrefix(referer, conf.GlobalServerConfig.DebugHost) ||
		strings.HasPrefix(referer, conf.GlobalServerConfig.Host) {
		return true
	} else {
		glog.Warningln("CSRF error:", GetIP(r), "Referer:", referer)
		return false
	}
}

func GetUser(r *http.Request) (ret AuthUser) {
	uid, _ := r.Cookie("uid")
	invalid := false

	defer func() {
		if invalid {
			glog.Warningln("Invalid cookie:", uid.Value, "IP:", GetIP(r))
		}
	}()

	if uid == nil {
		return
	}

	tmp := strings.Split(uid.Value, ":")
	if len(tmp) != 4 {
		invalid = true
		return
	}

	b_id := CleanString(tmp[0])
	b_username := tmp[1]
	b_session_id := tmp[2]
	b_verify := tmp[3]

	if _, err := strconv.Atoi(b_id); err != nil {
		invalid = true
		return
	}

	if b_verify == MakeHash(b_username, b_session_id) {
		var session_id, nickname, ip string
		var date, signupDate time.Time
		var _id int
		var status, group, comment, avatar, unread string
		var usage int

		if err := Gdb.QueryRow(`
			SELECT
				    users.id,
				    users.session_id,
				    users.nickname,
				    users.last_last_login_date,
				    users.last_last_login_ip,
				    users.signup_date,
				user_info.status,
				user_info.group,
				user_info.comment,
				user_info.avatar,
				user_info.image_usage,
				user_info.unread
			FROM 
				users
			INNER JOIN
				user_info ON user_info.id = users.id
			WHERE 
				users.id = `+b_id).
			Scan(&_id,
				&session_id,
				&nickname,
				&date,
				&ip,
				&signupDate,
				&status,
				&group,
				&comment,
				&avatar,
				&usage,
				&unread); err == nil {

			if session_id != b_session_id {
				return
			}

			comment = html.UnescapeString(comment)
			ret = AuthUser{_id,
				b_username,
				nickname,
				int(date.Unix()),
				int(signupDate.Unix()),
				ip,
				strings.Trim(status, " "),
				strings.Trim(group, " "),
				false,
				comment,
				avatar,
				usage,
				unread}
		} else {
			glog.Errorln("Database:", err)
		}

		return
	}

	invalid = true
	return //AuthUser{}
}
