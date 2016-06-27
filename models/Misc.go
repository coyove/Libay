package models

import (
	"../auth"
	"../conf"

	"github.com/coyove/DynCaptcha"
	"github.com/dchest/captcha"
	"github.com/julienschmidt/httprouter"

	_ "database/sql"
	"encoding/base64"
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
	payload.IsLoggedIn = u.Name != ""

	ServePage(w, "tags", payload)
}

func (th ModelHandler) GET_playground(w http.ResponseWriter, r *http.Request, ps httprouter.Params) {
	// type Image struct {
	// 	Name string
	// 	Date int
	// }

	// var payload struct {
	// 	MaxSize              int
	// 	AllowAnonymousUpload bool
	// 	HistoryImages        []Image
	// }
	// payload.MaxSize = conf.GlobalServerConfig.MaxImageSize
	// payload.AllowAnonymousUpload = (&auth.AuthUser{}).CanPostImages()
	// // payload.HistoryImages = auth.GetImages()
	// rows, err := auth.Gdb.Query(`
	//        SELECT
	//            MAX(id) AS id,
	//            image,
	//            MAX(date) AS date
	//        FROM
	//            images
	//        WHERE
	//            uploader = 0
	//        GROUP BY
	//            image
	//        ORDER BY
	//            id DESC
	//        LIMIT ` + strconv.Itoa(conf.GlobalServerConfig.PlaygroundMaxImages))

	// payload.HistoryImages = make([]Image, 0)

	// if err == nil {
	// 	defer rows.Close()

	// 	for rows.Next() {
	// 		var img string
	// 		var id int
	// 		var t time.Time
	// 		rows.Scan(&id, &img, &t)

	// 		payload.HistoryImages = append(payload.HistoryImages, Image{img, int(t.Unix())})
	// 	}
	// }

	// ServePage(w, "playground", payload)
	ServePage(w, "404", nil)
}

// PAGE: Serve about page
func (th ModelHandler) GET_about(w http.ResponseWriter, r *http.Request, ps httprouter.Params) {
	var payload struct {
		TotalUsers    int
		TotalArticles int
	}

	auth.Gdb.QueryRow(`
        SELECT 
            COUNT(id), 
            (SELECT
                COUNT(id)
            FROM
                articles
            WHERE tag < 100000) AS acount 
        FROM
            user_info`).
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
