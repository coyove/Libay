package models

import (
	"../auth"
	"../conf"

	"github.com/golang/glog"
	"github.com/julienschmidt/httprouter"

	"crypto/sha1"
	_ "database/sql"
	"fmt"
	"io/ioutil"
	"net/http"
	"os"
	// "os/exec"
	"path/filepath"
	"strconv"
	// "time"
)

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

	fn := fmt.Sprintf("%x", sha1.Sum(hashBuf))
	dirs := string(fn[0]) + "/" + string(fn[1]) + "/"
	if ava == "true" {
		dirs = "avatar/" + strconv.Itoa(u.ID/100) + "/" + strconv.Itoa(u.ID) + "/"
	}

	fn = dirs + fn[2:]
	ext := filepath.Ext(header.Filename)

	if ext == "" {
		ext = ".jpg"
	}

	fn += ext

	os.MkdirAll("./images/"+dirs, 0777)
	os.MkdirAll("./thumbs/"+dirs, 0777)

	alreadyUploaded := false
	if _, err := os.Stat("./images/" + fn); os.IsNotExist(err) {
		out, err := os.Create("./images/" + fn)
		if err != nil {
			Return(w, `{"Error": true, "R": "IO_Failure"}`)
			return
		}
		out.Write(hashBuf)
		out.Close()
	} else {
		alreadyUploaded = true
	}

	payload.Link = "/images/" + fn
	payload.Thumbnail = "/thumbs/" + fn

	if err := auth.ResizeImage(hashBuf, "./thumbs/"+fn,
		250, 250, auth.RICompressionLevel.DefaultCompression); err != nil {
		glog.Errorln("Generating thumbnail failed: "+fn, err)
		Return(w, `{"Error": true, "R": "Thumbnail_Failure"}`)
		return
	}

	imageSize := strconv.Itoa(len(hashBuf))
	if alreadyUploaded {
		imageSize = "0"
	}

	uid := strconv.Itoa(u.ID)

	_, err = auth.Gdb.Exec(`
		INSERT INTO images (image, uploader) VALUES ('` + fn + "', " + uid + `);
		UPDATE user_info SET image_usage = image_usage + ` + imageSize + ` WHERE id = ` + uid)

	payload.Error = err != nil

	if err != nil {
		glog.Errorln("Database:", err)
	}

	if ava == "true" {
		if _, err := auth.Gdb.Exec(`UPDATE user_info SET avatar = '` + fn + "' WHERE id = " + uid); err == nil {

			// os.Remove("./images/" + oldAvatar)
			// os.Remove("./thumbs/" + oldAvatar)

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
