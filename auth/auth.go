package auth

import (
	"../conf"
	"bytes"
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"crypto/sha1"
	"database/sql"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"github.com/dchest/captcha"
	"github.com/golang/glog"
	"golang.org/x/crypto/openpgp"
	"io"
	"io/ioutil"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"time"
)

const (
	// commonPrefix          = "auth/"
	// accountHTML           = commonPrefix + "account.html"
	stdTimeFormat         = "2006-01-02 15:04:05"
	maxRetryOpportunities = 2
	cooldownTime          = 30
)

var connString string
var databaseType string

var Salt string
var RecaptchaPrivateKey string
var RecaptchaPublicKey string
var Hostname string

var Gdb *sql.DB
var Gcache *Cache

// var Guser *Cache

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
	host := GetIP(r)
	mapIPTime.Lock()
	defer mapIPTime.Unlock()

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

func ServeLoginPhase1(w http.ResponseWriter, r *http.Request) {
	if !LogIP(r) {
		w.Write([]byte("Err::Router::Frequent_Access"))
		return
	}

	if !CheckCSRF(r) {
		w.Write([]byte("Err::CSRF::CSRF_Failure"))
		return
	}

	u := CleanString(r.FormValue("username"))

	if u == "" {
		w.Write([]byte("Err::Login::Empty_Username"))
		return
	}

	var pk string
	var pass string
	var id int

	if Gdb.QueryRow("select id, password, public_key_file from users where username='"+u+"'").
		Scan(&id, &pass, &pk) == nil {

		_start := time.Now()
		var payload struct {
			Key1 string
			Key2 string
		}

		password := MakeHash(u, Salt, time.Now().UnixNano())[:16]
		// fmt.Sprintf("%x", sha1.Sum([]byte(Salt+u+strconv.Itoa(int(time.Now().UnixNano()))+Salt)))[:16]

		if pk != "" {
			_, err := Gdb.Exec("UPDATE users SET password = '" + password + "' WHERE id = " + strconv.Itoa(id))
			if err != nil {
				w.Write([]byte("Err::DB::Update_Failure"))
				return
			}

			_password, err := gpgEncrypt(password, "./public_keys/"+pk)
			_wrapped := `-----BEGIN PGP MESSAGE-----
Version: GnuPG v2` + "\n" + _password + "\n" + `-----END PGP MESSAGE-----`
			glog.Infoln("Finish encrypting in", int(time.Now().Sub(_start).Nanoseconds()/1e6), "ms")

			payload.Key1 = _wrapped
			payload.Key2 = _password

			if err == nil {
				buf, _ := json.Marshal(&payload)
				w.Write(buf)
				return
			}

			w.Write([]byte("Err::IO::File_IO_Failure"))
		} else {
			w.Write([]byte("Err::Login::No_Public_Key"))
		}
	} else {
		w.Write([]byte("Err::DB::Select_Failure"))
	}

}

func ServeLoginPhase2(w http.ResponseWriter, r *http.Request) {
	if !LogIP(r) {
		w.Write([]byte("Err::Router::Frequent_Access"))
		return
	}

	if !CheckCSRF(r) {
		w.Write([]byte("Err::CSRF::CSRF_Failure"))
		return
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
		w.Write([]byte("Err::Login::Empty_Username"))
		return
	}

	if p == "" {
		w.Write([]byte("Err::Login::Empty_Password"))
		return
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

		// Gdb.QueryRow("select last_login_ip, last_login_date, retry, lock_date from users where username='"+u+"'").
		// 	Scan()
		mins := int(time.Now().Sub(lockDate).Minutes())
		if mins < cooldownTime {
			w.Write([]byte("Err::Login::Cooldown_" + strconv.Itoa(cooldownTime-mins) + "min"))
			return
		}

		if pass == p {
			new_session_id := MakeHash()
			// fmt.Sprintf("%x", sha1.Sum([]byte(u+strconv.Itoa(int(time.Now().UnixNano()))+Salt)))

			userToken := fmt.Sprintf("%d:%s:%s:%s", id, u, new_session_id, MakeHash(u, new_session_id))
			http.SetCookie(w, &http.Cookie{
				Name:    "uid",
				Value:   userToken,
				Expires: exp, HttpOnly: true, Path: "/"})

			_, err := Gdb.Exec("update users set last_login_date='" + time.Now().Format(stdTimeFormat) +
				"', last_last_login_date='" + lastLogin.Format(stdTimeFormat) +
				"', last_login_ip='" + GetIP(r) +
				"', last_last_login_ip='" + lastLoginIP +
				"', session_id='" + new_session_id +
				"', retry=0 where id=" + strconv.Itoa(id))

			if err != nil {
				glog.Errorln("Database:", err)
			}

			w.Write([]byte("ok " + strconv.Itoa(id)))
			return
			// finish a successful login procedure
		}

		if retry > maxRetryOpportunities {
			Gdb.Exec("update users set retry=0, lock_date='" +
				time.Now().Format(stdTimeFormat) + "' where id=" + strconv.Itoa(id))
			w.Write([]byte("Err::Login::Account_Locked"))

		} else {
			Gdb.Exec("update users set retry=retry+1 where id=" + strconv.Itoa(id))
			w.Write([]byte("Err::Login::Retry_" + strconv.Itoa(retry)))
		}

		return
	} else {
		glog.Errorln("Database:", err)
		w.Write([]byte("Err::DB::Select_Failure"))
	}

}

func ServeRegister(w http.ResponseWriter, r *http.Request) {
	if !LogIP(r) {
		w.Write([]byte("Err::Router::Frequent_Access"))
		return
	}

	if !CheckCSRF(r) {
		w.Write([]byte("Err::CSRF::CSRF_Failure"))
		return
	}

	if !conf.GlobalServerConfig.AllowRegistration {
		w.Write([]byte("Err::Regr::Registration_Closed"))
		return
	}

	r.ParseMultipartForm(1024)
	u := CleanString(r.FormValue("username"))
	nk := CleanString(r.FormValue("nickname"))

	if !captcha.VerifyString(r.FormValue("captcha-challenge"), r.FormValue("captcha-answer")) {
		w.Write([]byte("Err::Regr::Challenge_Failed"))
		return
	}

	if len(u) < 4 {
		w.Write([]byte("Err::Regr::Username_Too_Short"))
		return
	}

	if len(nk) < 4 {
		w.Write([]byte("Err::Regr::Nickname_Too_Short"))
		return
	}

	var _id int
	Gdb.QueryRow("select id from users where username='" + u + "' or nickname='" + nk + "'").Scan(&_id)

	if _id != 0 {
		w.Write([]byte("Err::Regr::Username_Nickname_Existed"))
		return
	}

	if r.FormValue("use-simple-password") == "true" {
		sp := CleanString(r.FormValue("simple-password"))
		if len(sp) < 4 {
			w.Write([]byte("Err::Regr::Password_Too_Short"))
			return
		}

		_, err := Gdb.Exec("insert into users (username, nickname, signup_date, password, public_key_file, retry) values ('" + u +
			"', '" + nk +
			"', '" + time.Now().Format(stdTimeFormat) +
			"', '" + sp +
			"', '', 0)")

		if err != nil {
			glog.Errorln("Database:", err)
			w.Write([]byte("Err::DB::Insert_Failure"))
			return
		}

		w.Write([]byte("ok"))
	} else {

		in, header, err := r.FormFile("public_key")
		if err != nil {
			w.Write([]byte("Err::IO::File_IO_Failure"))
			return
		}
		defer in.Close()

		ext := filepath.Ext(header.Filename)

		hashBuf, _ := ioutil.ReadAll(in)
		fn := fmt.Sprintf("%x", sha1.Sum(hashBuf)) + ext

		if _, err := os.Stat("./public_keys/" + fn); err == nil {
			w.Write([]byte("Err::Regr::Public_Key_Existed"))
			return
		}

		out, err := os.Create("./public_keys/" + fn)
		if err != nil {
			w.Write([]byte("Err::IO::File_IO_Failure"))
			return
		}
		// io.Copy(out, in)
		out.Write(hashBuf)
		defer out.Close()

		_, err = openpgp.ReadKeyRing(out)

		if err != nil {
			glog.Errorln("PK:", err)
			w.Write([]byte("Err::Regr::Invalid_Public_Key"))
			return
		}
		_, err = Gdb.Exec("insert into users (username, nickname, signup_date, password, public_key_file, retry) values ('" + u +
			"', '" + nk +
			"', '" + Time.Now() +
			"', '" +
			"', '" + fn +
			"', 0)")

		if err != nil {
			glog.Errorln("Database:", err)
			w.Write([]byte("Err::DB::Insert_Failure"))
			return
		}

		w.Write([]byte("ok"))
	}
}

func ServeLogout(w http.ResponseWriter, r *http.Request) {
	u := GetUser(r)

	if !CheckCSRF(r) {
		w.Write([]byte("Err::CSRF::CSRF_Failure"))
		return
	}

	if u.Name != "" {
		_, reer := Gdb.Exec("update users set session_id='" + MakeHash(u.Name) + "' where id=" + strconv.Itoa(u.ID))
		if reer == nil {
			w.Write([]byte("ok"))
			return
		}
	}

	w.Write([]byte("Err::Privil::Invalid_User"))
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
		// Guser = NewCache(10)
		Gcache.Start()
		// Guser.Start()

		return nil
	}
}
