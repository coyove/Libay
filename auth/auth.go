package auth

import (
	"../conf"

	"github.com/dchest/captcha"
	"github.com/golang/glog"
	"golang.org/x/crypto/openpgp"

	"bytes"
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	// "crypto/sha1"
	"database/sql"
	"encoding/base64"
	// "encoding/json"
	"errors"
	"fmt"
	"io"
	"io/ioutil"
	"net"
	"net/http"
	"os"
	// "path/filepath"
	"strconv"
	"strings"
	"sync"
	"time"
)

const (
	stdTimeFormat = "2006-01-02 15:04:05"
)

var connString string
var databaseType string

var Salt string
var RecaptchaPrivateKey string
var RecaptchaPublicKey string
var Hostname string

var Gdb *sql.DB

var Gcache *Cache
var Guser *Cache
var Gimage *Cache

var GarticleTimer *FixedQueue
var GmessageTimer *FixedQueue
var GuserTimer *FixedQueue

func AESEncrypt(key, text []byte) ([]byte, error) {
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, err
	}
	b := base64.StdEncoding.EncodeToString(text)
	ciphertext := make([]byte, aes.BlockSize+len(b))
	iv := ciphertext[:aes.BlockSize]
	if _, err := io.ReadFull(rand.Reader, iv); err != nil {
		return nil, err
	}
	cfb := cipher.NewCFBEncrypter(block, iv)
	cfb.XORKeyStream(ciphertext[aes.BlockSize:], []byte(b))
	return ciphertext, nil
}

func AESDecrypt(key, text []byte) ([]byte, error) {
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, err
	}
	if len(text) < aes.BlockSize {
		return nil, errors.New("ciphertext too short")
	}
	iv := text[:aes.BlockSize]
	text = text[aes.BlockSize:]
	cfb := cipher.NewCFBDecrypter(block, iv)
	cfb.XORKeyStream(text, text)
	data, err := base64.StdEncoding.DecodeString(string(text))
	if err != nil {
		return nil, err
	}
	return data, nil
}

func gpgEncrypt(secretString string, publicKeyring string) (string, error) {

	keyringFileBuffer, _ := os.Open(publicKeyring)
	defer keyringFileBuffer.Close()
	entityList, err := openpgp.ReadKeyRing(keyringFileBuffer)
	if err != nil {
		return "", err
	}

	// encrypt string
	buf := new(bytes.Buffer)
	w, err := openpgp.Encrypt(buf, entityList, nil, nil, nil)
	if err != nil {
		return "", err
	}
	_, err = w.Write([]byte(secretString))
	if err != nil {
		return "", err
	}
	err = w.Close()
	if err != nil {
		return "", err
	}

	// Encode to base64
	bb, err := ioutil.ReadAll(buf)
	if err != nil {
		return "", err
	}
	encStr := base64.StdEncoding.EncodeToString(bb)

	return encStr, nil
}

var mapIPTime struct {
	sync.RWMutex
	ips map[string]time.Time
}

func AccessDaemon() {
	mapIPTime.ips = make(map[string]time.Time)

	ticker := time.NewTicker(5 * time.Second)
	defer ticker.Stop()
	for {
		select {
		case <-ticker.C:
			mapIPTime.Lock()
			cur := time.Now()
			for k, v := range mapIPTime.ips {
				if cur.After(v.Add(4 * time.Second)) {
					// log.Println("delete", k)
					delete(mapIPTime.ips, k)
				}
			}

			err := Gdb.Ping()
			if err != nil {
				glog.Errorln("Database: lost ping,", err)
			}
			mapIPTime.Unlock()

			// glog.Flush()
		}
	}
}

func GetIP(r *http.Request) string {

	host := r.Header.Get("X-Real-IP")
	if host == "" {
		host = r.Header.Get("X-Forwarded-For")
		if host == "" {
			host, _, _ = net.SplitHostPort(r.RemoteAddr)

			if host == "" {
				host = "unknown"
			}

		}
	}

	tmp := strings.Split(host, ",")
	if len(tmp) > 1 {
		return tmp[0]
	}

	return host
}

func LogIP(r *http.Request) bool {
	if LogIPnv(r) {
		return true
	} else {
		glog.Warningln("Frequent access:", GetIP(r))
		return false
	}
}

func LogIPnv(r *http.Request) bool {
	mapIPTime.Lock()
	defer mapIPTime.Unlock()

	host := GetIP(r)
	if host == "127.0.0.1" || host == "localhost" {
		return true
	}

	if v, has := mapIPTime.ips[host]; has {
		cur := time.Now()
		if cur.After(v.Add(4 * time.Second)) {
			delete(mapIPTime.ips, host)
			return true
		}
		return false
	} else {
		mapIPTime.ips[host] = time.Now()
		return true
	}
}

func ServeLogin(w http.ResponseWriter, r *http.Request) string {
	if !LogIP(r) {
		return "Err::Router::Frequent_Access"
	}

	if !CheckCSRF(r) {
		return "Err::CSRF::CSRF_Failure"
	}

	u := CleanString(r.FormValue("username"))
	p := CleanString(r.FormValue("password"))
	expire, _ := strconv.Atoi(r.FormValue("expire"))

	exp := time.Now().AddDate(0, 0, expire)
	if expire == 0 {
		exp = exp.AddDate(1, 0, 0)
	}

	// log.Println(r.RemoteAddr)

	if u == "" {
		return "Err::Login::Empty_Username"
	}

	if p == "" {
		return "Err::Login::Empty_Password"
	}

	var pass, lastLoginIP string
	var id, retry int
	var lastLogin, lockDate time.Time

	if err := Gdb.QueryRow(`
		SELECT
			id, 
			password,
			COALESCE(last_login_ip, ''), 
			COALESCE(last_login_date, '1970-01-01'::timestamp with time zone), 
			COALESCE(retry, 0),
			COALESCE(lock_date, '1970-01-01'::timestamp with time zone)
		FROM
		 	users 
		WHERE
			username='`+u+"'").
		Scan(&id, &pass, &lastLoginIP, &lastLogin, &retry, &lockDate); err == nil {

		iid := strconv.Itoa(id)

		cooldownTime := conf.GlobalServerConfig.CooldownTime
		mins := int(time.Now().Sub(lockDate).Minutes())
		if mins < cooldownTime {
			return "Err::Login::Cooldown_" + strconv.Itoa(cooldownTime-mins) + "min"
		}

		if pass == MakeHash(p) {
			new_session_id := MakeHash()
			userToken := fmt.Sprintf("%d:%s:%s:%s", id, u, new_session_id, MakeHash(u, new_session_id))

			cookie := &http.Cookie{
				Name:     "uid",
				Value:    userToken,
				HttpOnly: true,
				Path:     "/",
			}
			if expire >= 0 {
				cookie.Expires = exp
			}

			http.SetCookie(w, cookie)

			var unread int
			unreadLimit := int(time.Now().UnixNano()/1e6 - 365*3600000*24)

			_start := time.Now()
			err := Gdb.QueryRow(`
				UPDATE 
                    users 
                SET 
                    last_login_date      = '` + Time.Now() + `', 
                    last_last_login_date = '` + Time.F(lastLogin) + `', 
                    last_login_ip        = '` + GetIP(r) + `', 
                    last_last_login_ip   = '` + lastLoginIP + `', 
                    session_id           = '` + new_session_id + `', 
                    retry                = 0 
                WHERE 
                    id = ` + iid + `;

                SELECT 
                	COUNT(id) 
            	FROM 
            		articles
            	WHERE
            		created_at > ` + strconv.Itoa(unreadLimit) + ` AND
            		(tag = ` + strconv.Itoa(id+100000) + ` AND read = false AND deleted = false)
            	LIMIT 99`).Scan(&unread)

			if err != nil {
				glog.Errorln("Database:", err)
				return "Err::DB::General_Failure"
			}

			GuserTimer.Push(time.Now().Sub(_start).Nanoseconds())

			http.SetCookie(w, &http.Cookie{
				Name:     "unread",
				Value:    strconv.Itoa(unread),
				Expires:  time.Now().Add(1 * time.Minute),
				HttpOnly: false,
			})

			return "ok " + iid
			// finish a successful login procedure
		}

		maxRetryOpportunities := conf.GlobalServerConfig.MaxRetryOpportunities

		if retry >= maxRetryOpportunities {
			Gdb.Exec("UPDATE users SET retry = 0, lock_date = '" + Time.Now() + "' WHERE id = " + iid)
			glog.Errorln("Account locked on", GetIP(r))

			return "Err::Login::Account_Locked"
		} else {
			var hint string
			Gdb.QueryRow(`
				UPDATE users SET retry = retry + 1 WHERE id = ` + iid + `;
				SELECT password_hint FROM users WHERE id = ` + iid).Scan(&hint)

			return "Err::Login::Retry_Opportunities_" +
				strconv.Itoa(maxRetryOpportunities-retry-1) +
				"_Hint_" + Unescape(hint)
		}

	} else {
		glog.Errorln("Database:", err)
	}
	return "Err::DB::Select_Failure"

}

func ServeRegister(w http.ResponseWriter, r *http.Request) string {
	if !LogIP(r) {
		return "Err::Router::Frequent_Access"
	}

	if !CheckCSRF(r) {
		return "Err::CSRF::CSRF_Failure"
	}

	if !conf.GlobalServerConfig.AllowRegistration {
		return "Err::Regr::Registration_Closed"
	}

	r.ParseMultipartForm(1024)

	u := CleanString(r.FormValue("username"))
	nk := CleanString(r.FormValue("nickname"))
	hint := Escape(r.FormValue("hint"))

	if len(hint) > 64 {
		hint = hint[:64]
	}

	if !captcha.VerifyString(r.FormValue("captcha-challenge"), r.FormValue("captcha-answer")) {
		glog.Errorln("Challenge failed by", GetIP(r))
		return "Err::Regr::Challenge_Failed"
	}

	if len(u) < 4 {
		return "Err::Regr::Username_Too_Short"
	}

	if len(nk) < 4 {
		return "Err::Regr::Nickname_Too_Short"
	}

	var _id int
	Gdb.QueryRow("SELECT id FROM users WHERE username = '" + u + "' OR nickname = '" + nk + "'").Scan(&_id)

	if _id != 0 {
		return "Err::Regr::Username_Or_Nickname_Existed"
	}

	sp := CleanString(r.FormValue("simple-password"))
	if len(sp) < 4 {
		return "Err::Regr::Password_Too_Short"
	}

	_, err := Gdb.Exec(`
        INSERT INTO users (username, nickname, password, password_hint) 
        VALUES ('` + u + `', '` + nk + `', '` + MakeHash(sp) + `', '` + hint + `')`)

	if err != nil {
		glog.Errorln("Database:", err)
		return "Err::DB::Insert_Failure"
	}

	return "ok"
}

func ServeLogout(w http.ResponseWriter, r *http.Request) string {
	u := GetUser(r)

	if !CheckCSRF(r) {
		return "Err::CSRF::CSRF_Failure"
	}

	if u.Name != "" {
		_, err := Gdb.Exec("UPDATE users SET session_id = '" + MakeHash() + "' WHERE id = " + strconv.Itoa(u.ID))
		if err == nil {
			Guser.Remove(u.ID)
			return "ok"
		}
	}

	return "Err::Privil::Invalid_User"
}

func ConnectDatabase(t string, conn string) error {
	var err error
	Gdb, err = sql.Open(t, conn)
	if err != nil {
		glog.Fatalln("Connecting to database failed")
		return err
	} else {
		connString = conn
		databaseType = t
		Gdb.SetMaxIdleConns(conf.GlobalServerConfig.MaxIdleConns)
		Gdb.SetMaxOpenConns(conf.GlobalServerConfig.MaxOpenConns)

		Gcache = NewCache(conf.GlobalServerConfig.CacheEntities)
		Guser = NewCache(conf.GlobalServerConfig.CacheEntities)
		Gimage = NewCache(conf.GlobalServerConfig.CacheEntities)
		Gcache.Start()
		Guser.Start()
		Gimage.Start()

		GarticleTimer = NewFixedQueue(20, 60)
		GmessageTimer = NewFixedQueue(20, 120)
		GuserTimer = NewFixedQueue(20, 60)
		return nil
	}
}
