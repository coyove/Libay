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
	"crypto/sha1"
	"database/sql"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
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
var Guser *Cache

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

func ServeLoginPhase1(w http.ResponseWriter, r *http.Request) string {
	if !LogIP(r) {
		return "Err::Router::Frequent_Access"
	}

	if !CheckCSRF(r) {
		return "Err::CSRF::CSRF_Failure"
	}

	u := CleanString(r.FormValue("username"))

	if u == "" {
		return "Err::Login::Empty_Username"
	}

	var pk string
	var pass string
	var id int

	if Gdb.QueryRow("SELECT id, password, public_key_file FROM users WHERE username = '"+u+"'").
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
				return "Err::DB::Update_Failure"
			}

			_password, err := gpgEncrypt(password, "./public_keys/"+pk)
			_wrapped := `-----BEGIN PGP MESSAGE-----
Version: GnuPG v2` + "\n" + _password + "\n" + `-----END PGP MESSAGE-----`
			glog.Infoln("Finish encrypting in", int(time.Now().Sub(_start).Nanoseconds()/1e6), "ms")

			payload.Key1 = _wrapped
			payload.Key2 = _password

			if err == nil {
				buf, _ := json.Marshal(&payload)
				return string(buf)
			}

			return "Err::IO::File_IO_Failure"
		} else {
			return "Err::Login::No_Public_Key"
		}
	}

	return "Err::DB::Select_Failure"

}

func ServeLoginPhase2(w http.ResponseWriter, r *http.Request) string {
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

		// Gdb.QueryRow("select last_login_ip, last_login_date, retry, lock_date from users where username='"+u+"'").
		// 	Scan()
		mins := int(time.Now().Sub(lockDate).Minutes())
		if mins < cooldownTime {
			return "Err::Login::Cooldown_" + strconv.Itoa(cooldownTime-mins) + "min"
		}

		if pass == p {
			new_session_id := MakeHash()
			// fmt.Sprintf("%x", sha1.Sum([]byte(u+strconv.Itoa(int(time.Now().UnixNano()))+Salt)))

			userToken := fmt.Sprintf("%d:%s:%s:%s", id, u, new_session_id, MakeHash(u, new_session_id))
			http.SetCookie(w, &http.Cookie{
				Name:    "uid",
				Value:   userToken,
				Expires: exp, HttpOnly: true, Path: "/"})

			var unread int
			unreadLimit := int(time.Now().UnixNano()/1e6 - 365*3600000*24)

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
                    id = ` + strconv.Itoa(id) + `;

                SELECT 
                	COUNT(id) 
            	FROM 
            		articles
            	WHERE
            		created_at > ` + strconv.Itoa(unreadLimit) + ` AND
            		(tag = ` + strconv.Itoa(id+100000) + ` AND read = false AND deleted = false)
            	LIMIT 20`).Scan(&unread)

			if err != nil {
				glog.Errorln("Database:", err)
			}

			return "ok " + strconv.Itoa(id) + " " + strconv.Itoa(unread)
			// finish a successful login procedure
		}

		if retry > maxRetryOpportunities {
			Gdb.Exec("UPDATE users SET retry = 0, lock_date = '" + Time.Now() + "' WHERE id = " + strconv.Itoa(id))
			glog.Errorln("Account locked by", GetIP(r))
			return "Err::Login::Account_Locked"
		} else {
			Gdb.Exec("UPDATE users SET retry = retry + 1 WHERE id = " + strconv.Itoa(id))
			return "Err::Login::Retry_" + strconv.Itoa(retry)
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
		return "Err::Regr::Username_Nickname_Existed"
	}

	if r.FormValue("use-simple-password") == "true" {
		sp := CleanString(r.FormValue("simple-password"))
		if len(sp) < 4 {
			return "Err::Regr::Password_Too_Short"
		}

		_, err := Gdb.Exec(`
            INSERT INTO users 
                (username, nickname, signup_date, password, public_key_file, retry) 
            VALUES (
                '` + u + `', 
                '` + nk + `', 
                '` + Time.Now() + `', 
                '` + sp + `', 
                '', 
                0)`)

		if err != nil {
			glog.Errorln("Database:", err)
			return "Err::DB::Insert_Failure"
		}
	} else {

		in, header, err := r.FormFile("public_key")
		if err != nil {
			return "Err::IO::File_IO_Failure"
		}
		defer in.Close()

		ext := filepath.Ext(header.Filename)

		hashBuf, _ := ioutil.ReadAll(in)
		fn := fmt.Sprintf("%x", sha1.Sum(hashBuf)) + ext

		if _, err := os.Stat("./public_keys/" + fn); err == nil {
			return "Err::Regr::Public_Key_Existed"
		}

		out, err := os.Create("./public_keys/" + fn)
		if err != nil {
			return "Err::IO::File_IO_Failure"
		}
		// io.Copy(out, in)
		out.Write(hashBuf)
		defer out.Close()

		_, err = openpgp.ReadKeyRing(out)

		if err != nil {
			glog.Errorln("PK:", err)
			return "Err::Regr::Invalid_Public_Key"
		}
		_, err = Gdb.Exec(`
            INSERT INTO users 
                (username, nickname, signup_date, password, public_key_file, retry) 
            VALUES (
                '` + u + `', 
                '` + nk + `', 
                '` + Time.Now() + `', 
                '', 
                '` + fn + `', 
                0)`)

		if err != nil {
			glog.Errorln("Database:", err)
			return "Err::DB::Insert_Failure"
		}
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
		Gcache.Start()
		Guser.Start()

		return nil
	}
}
