package auth

import (
	"../conf"

	"github.com/golang/glog"

	_ "database/sql"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"time"
)

var DummyUsers = []AuthUser{
	AuthUser{ID: 1, Group: "admin"},
	AuthUser{ID: 2, Group: "user"},
	AuthUser{ID: 3, Group: "user"},
	AuthUser{ID: 4, Group: "user"},
	AuthUser{ID: 5, Group: "user"},
}

var nicknameReverseLookup struct {
	sync.Mutex
	Map map[string]int
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

	if au.Group == "admin" {
		return true
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

func GetUser(vs ...interface{}) (ret AuthUser) {
	r := vs[0].(*http.Request)
	_uid, _ := r.Cookie("uid")
	invalid := false

	defer func() {
		if invalid {
			glog.Warningln("Invalid cookie:", _uid.Value, "IP:", GetIP(r))
		}
	}()

	uid := ""
	if _uid == nil {
		uid = r.FormValue("cduc")
		if uid == "" {
			return
		}
	} else {
		uid = _uid.Value
	}

	tmp := strings.Split(uid, ":")
	if len(tmp) != 4 {
		invalid = true
		return
	}

	b_id := CleanString(tmp[0])
	b_username := tmp[1]
	b_session_id := tmp[2]
	b_verify := tmp[3]

	if id, err := strconv.Atoi(b_id); err != nil {
		invalid = true
		return
	} else {

		if b_verify == MakeHash(b_username, b_session_id) {
			// User has a valid cookie, now test whether it has expired
			user := GetUserByID(id)
			if user.SessionID == b_session_id {
				if ur, err := r.Cookie("unread"); err != nil || ur.Value == "" {
					if len(vs) == 2 {
						unread := 0
						unreadLimit := int(time.Now().UnixNano()/1e6 - 3600000*24)

						err := Gdb.QueryRow(`
		                SELECT COUNT(id) 
		            	FROM   articles
		            	WHERE
		            		created_at > ` + strconv.Itoa(unreadLimit) + ` AND
		            		(tag = ` + strconv.Itoa(user.ID+100000) + ` AND read = false AND deleted = false)
		            	LIMIT 99`).Scan(&unread)

						if err != nil {
							glog.Errorln("Database:", err)
						}

						http.SetCookie(vs[1].(http.ResponseWriter), &http.Cookie{
							Name:     "unread",
							Value:    strconv.Itoa(unread),
							Expires:  time.Now().Add(1 * time.Minute),
							HttpOnly: false,
						})
					}
				}
				return user
			}

			Guser.Remove(id)
			return
		}
	}

	invalid = true
	return
}

func GetIDByNickname(n string) int {
	nicknameReverseLookup.Lock()
	defer nicknameReverseLookup.Unlock()

	if nicknameReverseLookup.Map == nil {
		nicknameReverseLookup.Map = make(map[string]int)
	}

	if id, e := nicknameReverseLookup.Map[n]; e {
		return id
	} else {
		if err := Gdb.QueryRow(`
            SELECT id
            FROM   users
            WHERE  users.nickname = '` + n + `'`).Scan(&id); err == nil {

			nicknameReverseLookup.Map[n] = id
			return id
		} else {
			glog.Errorln("Database:", err)
			return 0
		}
	}
}

func GetUserByID(id int) (ret AuthUser) {
	var session_id, nickname, username, ip, status, group, avatar, galleryVisible string
	var date, signupDate time.Time
	var _id, usage, index int

	if v, ok := Guser.Get(id); ok {
		ret = v.(AuthUser)
		if ret.IndexID != 0 {
			ret.Index = GetArticle(nil, ret, ret.IndexID, false)
		}
		return
	}

	if err := Gdb.QueryRow(`
            SELECT
                    users.id,
                    users.username,
           	        users.session_id, 
                    users.nickname,
                    users.last_last_login_date,
                    users.last_last_login_ip,
                    users.signup_date, 
                user_info.status,
                user_info.group,
                user_info.index,
                user_info.avatar,
                user_info.image_usage,
                user_info.g_visible
            FROM 
                users
            INNER JOIN
                user_info ON user_info.id = users.id
            WHERE 
                users.id = `+strconv.Itoa(id)).
		Scan(&_id,
		&username,
		&session_id,
		&nickname,
		&date,
		&ip,
		&signupDate,
		&status,
		&group,
		&index,
		&avatar,
		&usage,
		&galleryVisible); err == nil {

		ret = AuthUser{
			ID:             _id,
			Name:           username,
			NickName:       nickname,
			LastLoginDate:  int(date.Unix()),
			SignUpDate:     int(signupDate.Unix()),
			LastLoginIP:    ip,
			Status:         strings.Trim(status, " "),
			Group:          strings.Trim(group, " "),
			Avatar:         conf.GlobalServerConfig.ImageHost + "/" + avatar,
			AvatarThumb:    conf.GlobalServerConfig.ImageHost + "/small-" + avatar,
			ImageUsage:     usage,
			SessionID:      session_id,
			GalleryVisible: galleryVisible,
			IndexID:        index,
		}

		if index != 0 {
			ret.Index = GetArticle(nil, ret, index, false)
		}

		Guser.Add(_id, ret, conf.GlobalServerConfig.CacheLifetime)

		nicknameReverseLookup.Lock()
		if nicknameReverseLookup.Map == nil {
			nicknameReverseLookup.Map = make(map[string]int)
		}

		nicknameReverseLookup.Map[nickname] = _id
		nicknameReverseLookup.Unlock()
	} else {
		// glog.Errorln("Database:", err)
	}

	return
}
