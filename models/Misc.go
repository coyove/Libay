package models

import (
	"../auth"
	"../conf"

	"github.com/coyove/DynCaptcha"
	"github.com/dchest/captcha"
	"github.com/julienschmidt/httprouter"

	_ "database/sql"
	"encoding/base64"
	"fmt"
	"net/http"
	"strconv"
	"time"
)

func (th ModelHandler) GET_tags(w http.ResponseWriter, r *http.Request, ps httprouter.Params) {
	u := auth.GetUser(r)

	var payload struct {
		Tags       map[int]conf.Tag
		IsLoggedIn bool
		HotTags    []string
		TotalTags  int
	}

	tags := conf.GlobalServerConfig.GetComplexTags()
	rows, err := auth.Gdb.Query("SELECT tag FROM articles WHERE tag <= 65533 ORDER BY modified_at DESC LIMIT 10;")
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
		if tags[k].Visible {
			payload.HotTags = append(payload.HotTags, tags[k].Name)
		}
	}

	payload.Tags = conf.GlobalServerConfig.GetComplexTags()
	payload.TotalTags = len(payload.Tags)
	payload.IsLoggedIn = u.Name != ""

	ServePage(w, r, "tags", payload)
}

func (th ModelHandler) GET_playground(w http.ResponseWriter, r *http.Request, ps httprouter.Params) {
	ServePage(w, r, "404", nil)
}

func (th ModelHandler) GET_(w http.ResponseWriter, r *http.Request, ps httprouter.Params) {

	var payload struct {
		TotalUsers    int
		TotalArticles int
		TotalImages   int

		CanPostImages bool
		IsLoggedIn    bool
	}

	auth.Gdb.QueryRow(`
        SELECT 
            reltuples, 
            (
                SELECT reltuples
                FROM   pg_class 
                WHERE  relname = 'articles'
            ) AS acount,
            (
                SELECT reltuples
                FROM   pg_class 
                WHERE  relname = 'images'
            ) AS acount 
        FROM
            pg_class
        WHERE
            relname = 'users'`).
		Scan(&payload.TotalUsers, &payload.TotalArticles, &payload.TotalImages)

	u := auth.GetUser(r)

	payload.IsLoggedIn = u.ID != 0
	payload.CanPostImages = u.CanPostImages()

	ServePage(w, r, "index", payload)
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

func (th ModelHandler) GET_get_keywords(w http.ResponseWriter, r *http.Request, ps httprouter.Params) {
	Return(w, auth.GetImageKeywords())
}

func randomImage(w http.ResponseWriter, r18 bool) {

	var id, uploader, ts int
	var image string

	err := auth.Gdb.QueryRow(fmt.Sprintf(`
		SELECT id, uploader, ts, image FROM images 
		WHERE id = (SELECT random_image(%v) LIMIT 1);`, r18)).Scan(&id, &uploader, &ts, &image)
	if err != nil {
		w.WriteHeader(503)
		return
	}

	ts = ts + 1
	url := fmt.Sprintf("/gallery/%d/page/before=%s_%s", uploader, auth.HashTS(ts), auth.To60(uint64(ts)))
	img := conf.GlobalServerConfig.ImageHost + "/" + image

	w.Header().Add("X-Image", img)
	w.Write([]byte(fmt.Sprintf(`<a href="%s" target=_blank><img src="%s"/></a>`, url, img)))
}

func (th ModelHandler) GET_random_safe(w http.ResponseWriter, r *http.Request, ps httprouter.Params) {
	randomImage(w, false)
}

func (th ModelHandler) GET_random(w http.ResponseWriter, r *http.Request, ps httprouter.Params) {
	randomImage(w, true)
}
