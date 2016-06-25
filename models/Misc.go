package models

import (
	"../auth"
	"../conf"
	// "crypto/sha1"
	_ "database/sql"
	"encoding/base64"
	// "fmt"
	// "fmt"
	"github.com/coyove/DynCaptcha"
	"github.com/dchest/captcha"
	"github.com/julienschmidt/httprouter"
	"net/http"
	"strconv"
	// "strings"
	"time"
)

func (th ModelHandler) GET_tags(w http.ResponseWriter, r *http.Request, ps httprouter.Params) {
	u := auth.GetUser(r)

	var payload struct {
		Tags       map[int]conf.Tag
		IsLoggedIn bool
		HotTags    []string
	}

	rows, err := auth.Gdb.Query("select tag from articles where tag <= 65533 order by modified_at desc limit 10;")
	m := make(map[int]bool)

	payload.HotTags = make([]string, 0)

	if err == nil {
		defer rows.Close()
		for rows.Next() {
			var t int
			rows.Scan(&t)
			m[t] = true
		}
	}

	for k, _ := range m {
		payload.HotTags = append(payload.HotTags, conf.GlobalServerConfig.GetIndexTag(k))
	}

	payload.Tags = conf.GlobalServerConfig.GetComplexTags()
	payload.IsLoggedIn = u.Name != ""

	ServePage(w, "tags", payload)
}

func (th ModelHandler) GET_playground(w http.ResponseWriter, r *http.Request, ps httprouter.Params) {
	type Image struct {
		Name string
		Date int
	}

	var payload struct {
		MaxSize              int
		AllowAnonymousUpload bool
		HistoryImages        []Image
	}
	payload.MaxSize = conf.GlobalServerConfig.MaxImageSizeGuest
	payload.AllowAnonymousUpload = conf.GlobalServerConfig.AllowAnonymousUpload
	// payload.HistoryImages = auth.GetImages()
	rows, err := auth.Gdb.Query(`select max(id) as id, image, max(date) as date from images 
		where uploader=0 group by image order by id desc limit ` +
		strconv.Itoa(conf.GlobalServerConfig.PlaygroundMaxImages))

	payload.HistoryImages = make([]Image, 0)

	if err == nil {
		defer rows.Close()

		for rows.Next() {
			var img string
			var id int
			var t time.Time
			rows.Scan(&id, &img, &t)

			payload.HistoryImages = append(payload.HistoryImages, Image{img, int(t.Unix())})
		}
	}

	ServePage(w, "playground", payload)
}

// PAGE: Serve about page
func (th ModelHandler) GET_about(w http.ResponseWriter, r *http.Request, ps httprouter.Params) {
	var payload struct {
		TotalUsers    int
		TotalArticles int
	}

	auth.Gdb.QueryRow(`select count(id), (select count(id) from articles where tag < 100000) as acount from user_info`).
		Scan(&payload.TotalUsers, &payload.TotalArticles)

	ServePage(w, "about", payload)
}

func (th ModelHandler) GET_status(w http.ResponseWriter, r *http.Request, ps httprouter.Params) {
	http.ServeFile(w, r, "./assets/test.png")
}

func (th ModelHandler) GET_dyncaptcha(w http.ResponseWriter, r *http.Request, ps httprouter.Params) {
	// _start := time.Now()
	buf, ans := DynCaptcha.New(0)
	txt := strconv.Itoa(int(time.Now().UnixNano())) + ":" + strconv.Itoa(ans)
	enc, _ := auth.AESEncrypt([]byte(auth.Salt), []byte(txt))

	w.Header().Add("Content-type", "image/gif")
	w.Header().Add("Challenge", base64.StdEncoding.EncodeToString(enc))
	w.Write(buf)
}

func (th ModelHandler) GET_new_captcha(w http.ResponseWriter, r *http.Request, ps httprouter.Params) {
	cid := captcha.New()
	//
	w.Write([]byte(cid))
}

func (th ModelHandler) GET_get_captcha_CAPTCHA(w http.ResponseWriter, r *http.Request, ps httprouter.Params) {

	captcha.WriteImage(w, ps.ByName("captcha"), 200, 50)
	// captcha.WriteImage(w, cid, 200, 50)
}
