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
	"os/exec"
	"path/filepath"
	"strconv"
	"time"
)

func (th ModelHandler) POST_upload(w http.ResponseWriter, r *http.Request, ps httprouter.Params) {
	u := auth.GetUser(r)

	if !u.CanPostImages() {
		Return(w, `{"Error": true, "R": "P"}`)
		return
	}

	var payload struct {
		Error     bool
		Avatar    string
		Link      string
		Thumbnail string
	}

	r.ParseMultipartForm(int64(1024 * 1024 * 5)) // conf.GlobalServerConfig.MaxImageSize))

	in, header, err := r.FormFile("image")
	ava := r.FormValue("avatar")
	if err != nil {
		Return(w, `{"Error": true, "R": "H"}`)
		return
	}
	defer in.Close()

	if ava == "true" && !auth.CheckCSRF(r) {
		Return(w, `{"Error": true, "R": "C"}`)
		return
	}

	hashBuf, _ := ioutil.ReadAll(in)

	if len(hashBuf) > 1024*1024*conf.GlobalServerConfig.MaxImageSize {
		Return(w, `{"Error": true, "R": "R"}`)
		return
	}

	fn := fmt.Sprintf("%x", sha1.Sum(hashBuf))
	dirs := string(fn[0]) + "/" + string(fn[1]) + "/"
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
			Return(w, `{"Error": true, "R": "I"}`)
			return
		}
		out.Write(hashBuf)
		out.Close()
	} else {
		alreadyUploaded = true
	}

	payload.Link = "/images/" + fn
	payload.Thumbnail = "/thumbs/" + fn

	width, werr := strconv.Atoi(r.FormValue("width"))
	height, herr := strconv.Atoi(r.FormValue("height"))
	needThumb := true

	if werr == nil && herr == nil && width <= 250 && height <= 250 {
		// No need to resize it
		needThumb = false
	}

	writeFakeThumb := func() bool {
		thumb, err := os.Create("./thumbs/" + fn)
		if err != nil {
			Return(w, `{"Error": true, "R": "T"}`)
			return false
		}
		thumb.Write(hashBuf)
		thumb.Close()
		return true
	}

	if needThumb {
		if _, err := os.Stat("./thumbs/" + fn); os.IsNotExist(err) {
			cmd := exec.Command("vipsthumbnail", "./images/"+fn, "-s", "250", "-o", "../../../thumbs/"+fn)
			err = cmd.Start()
			if err != nil {
				glog.Errorln("Resizing image failed:", fn, err)
			} else {
				//				cmd.Wait()
				done := make(chan error)
				go func() { done <- cmd.Wait() }()
				select {
				case <-done:
					// exited
				case <-time.After(3 * time.Second):
					// If we see nothing, just write a fake one
					glog.Warningln("Resizing image timeout: " + fn)
					if _, err := os.Stat("./thumbs/" + fn); os.IsNotExist(err) {
						if !writeFakeThumb() {
							glog.Errorln("Writing fake thumb failed: " + fn)
							return
						}
					}
				}
			}
		}

	} else {
		// Just write the original one as a thumbnail
		// thumb, err := os.Create("./thumbs/" + fn)
		// if err != nil {
		// 	w.Write([]byte("{\"Error\": true}"))
		// 	return
		// }
		// thumb.Write(hashBuf)
		// thumb.Close()
		if !writeFakeThumb() {
			return
		}
	}

	imageSize := strconv.Itoa(len(hashBuf))
	if alreadyUploaded {
		imageSize = "0"
	}

	_, err = auth.Gdb.Exec(`
		INSERT INTO images (image, uploader) VALUES ('` + fn + "', " + strconv.Itoa(u.ID) + `);
		UPDATE user_info SET image_usage = image_usage + ` + imageSize + `WHERE id = ` + strconv.Itoa(u.ID))

	payload.Error = err != nil

	if err != nil {
		glog.Errorln("Database:", err)
	}

	if ava == "true" {
		_, err := auth.Gdb.Exec("update user_info set avatar='" + fn + "' where id=" + strconv.Itoa(u.ID))

		if err == nil {
			payload.Avatar = "ok"
		} else {
			glog.Errorln("Database:", err)
			payload.Avatar = "error"
		}
		// payload.Avatar = auth.SetUserAvatar(u, fn)
	}

	if r.FormValue("direct") != "direct" {
		Return(w, payload)
	} else {
		http.ServeFile(w, r, "."+payload.Link)
	}
}

func (th ModelHandler) POST_upload_file(w http.ResponseWriter, r *http.Request, ps httprouter.Params) {
	u := auth.GetUser(r)
	if !conf.GlobalServerConfig.GetPrivilege(u.Group, "UploadFile") {
		Return(w, `{"Error": true}`)
		return
	}

	if !auth.CheckCSRF(r) {
		Return(w, `{"Error": true}`)
		return
	}

	var payload struct {
		Error bool
		Link  string
	}

	r.ParseMultipartForm(int64(1024 * 1024 * 5)) // conf.GlobalServerConfig.MaxImageSize))

	in, header, err := r.FormFile("file")
	if err != nil {
		Return(w, `{"Error": true}`)
		return
	}
	defer in.Close()
	hashBuf, _ := ioutil.ReadAll(in)

	fn := fmt.Sprintf("%x", sha1.Sum(hashBuf))
	fn += filepath.Ext(header.Filename)

	alreadyUploaded := false
	if _, err := os.Stat("./images/" + fn); os.IsNotExist(err) {
		out, err := os.Create("./images/" + fn)
		if err != nil {
			Return(w, `{"Error": true}`)
			return
		}
		out.Write(hashBuf)
		out.Close()
	} else {
		alreadyUploaded = true
	}

	payload.Link = "/images/" + fn

	imageSize := strconv.Itoa(len(hashBuf))
	if alreadyUploaded {
		imageSize = "0"
	}

	_, err = auth.Gdb.Exec(`
		INSERT INTO images (image, uploader) VALUES ('` + fn + "', " + strconv.Itoa(u.ID) + `);
		UPDATE user_info SET image_usage = image_usage + ` + imageSize + ` WHERE id = ` + strconv.Itoa(u.ID))

	payload.Error = err != nil

	if err != nil {
		glog.Errorln("Database:", err)
	}

	Return(w, payload)
}
