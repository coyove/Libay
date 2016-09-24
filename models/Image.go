package models

import (
	"../auth"
	"../conf"

	"github.com/golang/glog"
	"github.com/julienschmidt/httprouter"

	"crypto/sha1"
	_ "database/sql"
	"encoding/binary"
	"fmt"
	"io/ioutil"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"
)

func calcImagePath(fn string, ext string, id int) (string, string, string) {
	buf := auth.MakeHashRaw(fn)
	hash := auth.To60(uint64(binary.BigEndian.Uint32(buf[:4])))

	storePath := strconv.Itoa(id/100) + "/" + strconv.Itoa(id) + "/" + string(hash[0]) + "/"
	url := auth.To60(uint64(id)) + "/" + hash
	finalFilename := storePath + hash[1:]

	if ext == "" {
		ext = ".jpg"
	}

	return storePath, finalFilename + ext, url + ext
}

func (th ModelHandler) POST_upload(w http.ResponseWriter, r *http.Request, ps httprouter.Params) {
	u := auth.GetUser(r)
	ava := r.FormValue("avatar")

	if !auth.CheckCSRF(r) {
		Return(w, `{"Error": true, "R": "CSRF"}`)
		return
	}

	if !u.CanPostImages() {
		if ava == "true" && u.ID > 0 {
			// Even user cannot upload images, he can set his own avatar
		} else {
			Return(w, `{"Error": true, "R": "Cannot_Upload"}`)
			return
		}
	}

	var payload struct {
		Error     bool
		Avatar    string
		Link      string
		Thumbnail string
	}

	r.ParseMultipartForm(int64(1024 * 1024 * 5))

	in, header, err := r.FormFile("image")
	if err != nil {
		Return(w, `{"Error": true, "R": "HTTP_Form_Failure"}`)
		return
	}
	defer in.Close()
	hashBuf, _ := ioutil.ReadAll(in)

	if len(hashBuf) > 1024*1024*conf.GlobalServerConfig.MaxImageSize {
		Return(w, `{"Error": true, "R": "Image_Too_Large"}`)
		return
	}

	detectBuf := hashBuf
	if len(detectBuf) > 512 {
		detectBuf = detectBuf[:512]
	}

	switch http.DetectContentType(detectBuf) {
	case "image/jpeg", "image/jpg", "image/gif", "image/png":
	default:
		Return(w, `{"Error": true, "R": "Invalid_Image"}`)
		return
	}

	dir, path, url := calcImagePath(fmt.Sprintf("%x", sha1.Sum(hashBuf)), filepath.Ext(header.Filename), u.ID)

	os.MkdirAll("./images/"+dir, 0777)
	os.MkdirAll("./thumbs/"+dir, 0777)

	alreadyUploaded := false
	if _, err := os.Stat("./images/" + path); os.IsNotExist(err) {
		out, err := os.Create("./images/" + path)
		if err != nil {
			Return(w, `{"Error": true, "R": "IO_Failure"}`)
			return
		}
		out.Write(hashBuf)
		out.Close()
	} else {
		alreadyUploaded = true
	}

	payload.Link = "/img/" + url
	payload.Thumbnail = "/timg/" + url

	if err := auth.ResizeImage(hashBuf, "./thumbs/"+path,
		250, 250, auth.RICompressionLevel.DefaultCompression); err != nil {
		glog.Errorln("Generating thumbnail failed: "+path, err)
		Return(w, `{"Error": true, "R": "Thumbnail_Failure"}`)
		return
	}

	imageSize := strconv.Itoa(len(hashBuf))
	if alreadyUploaded {
		imageSize = "0"
	}

	uid := strconv.Itoa(u.ID)

	_, err = auth.Gdb.Exec(`
		INSERT INTO images (image, path, uploader) VALUES ('` + url + "', '" + path + "', " + uid + `);
		UPDATE user_info SET image_usage = image_usage + ` + imageSize + ` WHERE id = ` + uid)

	payload.Error = err != nil

	if err != nil {
		glog.Errorln("Database:", err)
	}

	if ava == "true" {
		if _, err := auth.Gdb.Exec(`UPDATE user_info SET avatar = '` + url + "' WHERE id = " + uid); err == nil {
			payload.Avatar = "ok"
			auth.Guser.Remove(u.ID)

		} else {
			glog.Errorln("Database:", err)
			payload.Avatar = "error"
		}
	}

	if r.FormValue("direct") != "direct" {
		Return(w, payload)
	} else {
		http.ServeFile(w, r, "."+payload.Link)
	}
}

func ServeImage(w http.ResponseWriter, ps httprouter.Params, prefix string) {
	write := func(mime string, buf []byte) {
		w.Header().Add("Content-Type", mime)
		w.Header().Add("Cache-Control", "public, max-age=7200")
		w.Header().Add("Expires", time.Now().Add(2*time.Hour).Format(time.RFC1123))
		w.Write(buf)
	}

	id := auth.From60(ps.ByName("user"))
	img := ps.ByName("image")
	path := prefix + strconv.Itoa(id/100) + "/" + strconv.Itoa(id) + "/" + string(img[0]) + "/" + img[1:]

	if _image, e := auth.Gimage.Get(path); e {
		image := _image.([]interface{})
		write(image[0].(string), image[1].([]byte))
		return
	}

	buf, err := ioutil.ReadFile(path)
	if err != nil {
		Return(w, 404)
		auth.Gimage.Remove(path)
		return
	}

	mime := ""
	if len(buf) > 512 {
		mime = http.DetectContentType(buf[:512])
	} else {
		mime = http.DetectContentType(buf)
	}

	write(mime, buf)
	auth.Gimage.Add(path, []interface{}{mime, buf}, conf.GlobalServerConfig.CacheLifetime)
}

func (th ModelHandler) GET_img_USER_IMAGE(w http.ResponseWriter, r *http.Request, ps httprouter.Params) {
	ServeImage(w, ps, "./images/")
}

func (th ModelHandler) GET_timg_USER_IMAGE(w http.ResponseWriter, r *http.Request, ps httprouter.Params) {
	ServeImage(w, ps, "./thumbs/")
}

func (th ModelHandler) POST_delete_images(w http.ResponseWriter, r *http.Request, ps httprouter.Params) {
	u := auth.GetUser(r)
	if u.ID == 0 {
		Return(w, "Err::Post::Invalid_User")
		return
	}

	if !auth.CheckCSRF(r) {
		Return(w, "Err::CSRF::CSRF_Failure")
		return
	}

	ids := []string{}
	for _, v := range strings.Split(r.FormValue("ids"), ",") {
		_, err := strconv.Atoi(v)
		if err == nil {
			ids = append(ids, v)
		}
	}

	if len(ids) == 0 {
		Return(w, "Err::Post::Invalid_IDs")
		return
	}

	rows, err := auth.Gdb.Query("SELECT id, path FROM images WHERE uploader = " + strconv.Itoa(u.ID) +
		" AND id IN (" + strings.Join(ids, ",") + ")")

	if err != nil {
		Return(w, "Err:DB::Select_Failure")
		return
	}

	for rows.Next() {
		var id int
		var path string

		rows.Scan(&id, &path)

		os.Remove("./images/" + path)
		os.Remove("./thumbs/" + path)
		auth.Gimage.Remove("./images/" + path)
		auth.Gimage.Remove("./thumbs/" + path)
	}

	sql := "DELETE FROM images WHERE uploader = " + strconv.Itoa(u.ID) +
		" AND id IN (" + strings.Join(ids, ",") + ")"

	if _, err := auth.Gdb.Exec(sql); err == nil {
		Return(w, "ok")
	} else {
		glog.Errorln("Database:", err)
		Return(w, "Err:DB::Delete_Failure")
	}
}
