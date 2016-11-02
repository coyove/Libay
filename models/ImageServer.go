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
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"time"
)

var uploadDeamon struct {
	sync.Mutex

	Map          map[int]int
	MapThreshold map[int]int
}

func UploadDeamon() {
	for {
		uploadDeamon.Lock()
		if uploadDeamon.Map == nil {
			uploadDeamon.Map = make(map[int]int)
			uploadDeamon.MapThreshold = make(map[int]int)
		}

		for k, v := range uploadDeamon.Map {
			uploadDeamon.Map[k] = v - conf.GlobalServerConfig.ImagePointsDecline
			if v <= 0 {
				delete(uploadDeamon.Map, k)
				delete(uploadDeamon.MapThreshold, k)
			}
		}

		uploadDeamon.Unlock()
		time.Sleep(1 * time.Minute)
	}
}

func calcImagePath(fn string, ext string, id int) (string, string, string) {
	buf := auth.MakeHashRaw(fn)
	idbuf := make([]byte, 4)

	binary.BigEndian.PutUint32(idbuf, uint32(id))
	hash := auth.To60(binary.BigEndian.Uint64(append(buf[:4], idbuf...)))

	storePath := strconv.Itoa(id/100) + "/" + strconv.Itoa(id) + "/" + string(hash[0]) + "/"
	url := hash
	finalFilename := storePath + hash[1:]

	if ext == "" {
		ext = ".jpg"
	} else {
		ext = strings.ToLower(ext)
	}

	return storePath, finalFilename + ext, url + ext
}

func reverseCacheDeletion(id string) {
	if conf.GlobalServerConfig.ReverseCache != "" {
		url := conf.GlobalServerConfig.ReverseCache + "/reverse/cache/" + id

		client := &http.Client{}
		req, _ := http.NewRequest("GET", url, nil)
		req.Header.Add("X-Salt", conf.GlobalServerConfig.Salt)
		res, err := client.Do(req)
		if err != nil {
			res.Body.Close()
		}
	}
}

func getRealPath(url string) (string, int) {
	imageDots := strings.Split(url, ".")
	cookie := imageDots[0]

	if len(imageDots) != 2 || len(cookie) < 6 {
		return "", 0
	}

	prefix := "./images/"
	if cookie[:6] == "small-" {
		cookie = cookie[6:]
		prefix = "./thumbs/"
	}

	id := int(auth.From60(cookie) & 4294967295)

	return prefix + strconv.Itoa(id/100) + "/" +
		strconv.Itoa(id) + "/" + string(cookie[0]) + "/" + cookie[1:] + "." + imageDots[1], id
}

func (th ModelHandler) POST_upload(w http.ResponseWriter, r *http.Request, ps httprouter.Params) {
	u := auth.GetUser(r)
	uid := strconv.Itoa(u.ID)
	ava := r.FormValue("avatar")
	hide := r.FormValue("hide")
	r18 := r.FormValue("r18")
	tag := auth.CleanString("", r.FormValue("tag"))

	if !u.CanPostImages() {
		if ava == "true" && u.ID > 0 {
			// Even user cannot upload images, he can set his own avatar
		} else {
			Return(w, `{"Error": true, "R": "Cannot_Upload"}`)
			return
		}
	}

	uploadDeamon.Lock()

	if uploadDeamon.MapThreshold[u.ID] == 0 {
		uploadDeamon.MapThreshold[u.ID] = conf.GlobalServerConfig.ImagePointsThreshold
	}

	if uploadDeamon.Map[u.ID] > uploadDeamon.MapThreshold[u.ID] && u.Group != "admin" {
		uploadDeamon.MapThreshold[u.ID] = 1
		Return(w, fmt.Sprintf(`{"Error": true, "R": "Over_Quota_Wait_%dmin"}`,
			uploadDeamon.Map[u.ID]/conf.GlobalServerConfig.ImagePointsDecline))
		uploadDeamon.Unlock()

		return
	} else {
		uploadDeamon.Map[u.ID]++
	}

	uploadDeamon.Unlock()

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
	payload.Link = conf.GlobalServerConfig.ImageHost + "/" + url
	payload.Thumbnail = conf.GlobalServerConfig.ImageHost + "/small-" + url

	alreadyUploaded := false
	if _, err := os.Stat("./images/" + path); os.IsNotExist(err) {
		out, err := os.Create("./images/" + path)
		if err != nil {
			os.Remove("./images/" + path)
			Return(w, `{"Error": true, "R": "IO_Failure"}`)
			return
		}
		out.Write(hashBuf)
		out.Close()
	} else {
		alreadyUploaded = true
	}

	if _, err := os.Stat("./thumbs/" + path); os.IsNotExist(err) {
		better := conf.GlobalServerConfig.GetPrivilege(u.Group, "ImageMagick")

		var err error = nil
		if !better {
			err = auth.ResizeImage(hashBuf, "./thumbs/"+path, 250, 250,
				auth.RICompressionLevel.DefaultCompression)
			if err != nil {
				glog.Errorln("Generating thumbnail failed 1: "+path, err)
			}
		}

		if better || err != nil {
			cmd := exec.Command("sh", "-c", "convert ./images/"+path+
				"[0] -quality 90 -thumbnail '250x250>' ./thumbs/"+path)
			err = cmd.Start()
			err = cmd.Wait()

			if err != nil {
				glog.Errorln("Generating thumbnail failed 2: "+path, err)
				Return(w, `{"Error": true, "R": "Thumbnail_Failure"}`)
				return
			}
		}
	}

	if !alreadyUploaded {
		imageSize := strconv.Itoa(len(hashBuf))

		filename := auth.CleanString(header.Filename)
		if tag != "" {
			filename = strings.Replace(tag, "$", "*"+filename, -1)
		} else {
			filename = "*" + filename
		}

		_, err = auth.Gdb.Exec(`
		INSERT INTO images (image, path, filename, uploader, ts, hide, r18, size) 
		VALUES (
			'` + url + `', 
			'` + path + `', 
			'` + filename + `',
			` + uid + `, 
			` + strconv.Itoa(int(time.Now().UnixNano()/1e6)) + `,
			` + strconv.FormatBool(hide == "true") + `,
			` + strconv.FormatBool(r18 == "true") + `,
			` + imageSize + `
		);
		UPDATE user_info SET image_usage = image_usage + ` + imageSize + ` WHERE id = ` + uid)

		payload.Error = err != nil

		if err != nil {
			glog.Errorln("Database:", err)
		}
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

	auth.Gcache.Remove(`\S+-` + uid + `-img(true|false)`)
	auth.Guser.Remove(u.ID)
	reverseCacheDeletion(uid)

	if r.FormValue("direct") != "direct" {
		Return(w, payload)
	} else {
		http.ServeFile(w, r, "."+payload.Link)
	}
}

func ServeImage(w http.ResponseWriter, r *http.Request) {
	url := r.URL.RequestURI()[1:]
	origin := strings.ToLower(r.Header.Get("origin"))

	if origin == conf.GlobalServerConfig.Host ||
		origin == conf.GlobalServerConfig.DebugHost {
		w.Header().Add("Access-Control-Allow-Origin", origin)
	}

	if url == "upload" {
		ModelHandlerDummy.POST_upload(w, r, nil)
		return
	} else if url == "alter/images" {
		ModelHandlerDummy.POST_alter_images(w, r, nil)
		return
	} else if url == "search/image" {
		ModelHandlerDummy.POST_search_image(w, r, nil)
		return
	} else if url == "cache" {
		ci := auth.Gimage.GetLowLevelCache()
		buf, _ := exec.Command("sh", "-c", "df | awk 'NR==2 {print $5}'").Output()

		caches := []string{"Disk usage: " + string(buf)}

		for k, v := range ci {
			_, sec, hits := auth.Gcache.Info(v)

			if sec < 0 {
				caches = append(caches, fmt.Sprintf("Hits: %5d, waits purging: %s", hits, k))
			} else {
				caches = append(caches, fmt.Sprintf("Hits: %5d, expire in %2ds: %s", hits, sec, k))
			}
		}

		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		Return(w, "<pre>"+strings.Join(caches, "<br>")+"</pre>")
		return
	}

	write := func(mime string, buf []byte, etag string) {
		w.Header().Add("Content-Type", mime)
		w.Header().Add("Cache-Control", "public, max-age=31536000")
		w.Header().Add("Expires", time.Now().Add(8760*time.Hour).Format(time.RFC1123))
		w.Header().Add("ETag", etag)
		w.Write(buf)
	}

	path, _ := getRealPath(url)
	if path == "" {
		Return(w, 404)
		return
	}

	if _, err := os.Stat(path); os.IsNotExist(err) {
		Return(w, 404)
		return
	}

	if auth.LogIPnv(r) {
		if len(url) > 6 && url[:6] == "small-" {

		} else {
			auth.IncrImageCounter(url)
		}
	}

	if r.Header.Get("If-None-Match") != "" {
		Return(w, 304)
		return
	}

	if _image, e := auth.Gimage.Get(path); e {
		image := _image.([]interface{})
		write(image[0].(string), image[1].([]byte), image[2].(string))
		return
	}

	buf, err := ioutil.ReadFile(path)
	if err != nil {
		Return(w, 404)
		auth.Gimage.Remove(path)
		return
	}

	etag := fmt.Sprintf("%x", sha1.Sum(buf))[:16]
	mime := ""
	if len(buf) > 512 {
		mime = http.DetectContentType(buf[:512])
	} else {
		mime = http.DetectContentType(buf)
	}

	write(mime, buf, etag)
	auth.Gimage.Add(path, []interface{}{mime, buf, etag},
		conf.GlobalServerConfig.CacheLifetime)
}

func (th ModelHandler) POST_alter_images(w http.ResponseWriter, r *http.Request, ps httprouter.Params) {
	u := auth.GetUser(r)
	id, err := strconv.Atoi(r.FormValue("id"))
	action := r.FormValue("action")

	if u.ID == 0 && action != "flag" {
		Return(w, "Err::Post::Invalid_User")
		return
	}

	if err != nil || id < 0 {
		Return(w, "Err::Post::Invalid_ID")
		return
	}

	ids := []string{}
	for _, v := range strings.Split(r.FormValue("ids"), ",") {
		if _, err := strconv.Atoi(v); err == nil {
			ids = append(ids, v)
			if len(ids) > 1000 {
				Return(w, "Err::Post::Too_Many_IDs")
				return
			}
		}
	}

	if len(ids) == 0 {
		Return(w, "Err::Post::Invalid_IDs")
		return
	}

	tester := "uploader = " + strconv.Itoa(u.ID)
	if conf.GlobalServerConfig.GetPrivilege(u.Group, "EditOthers") &&
		conf.GlobalServerConfig.GetPrivilege(u.Group, "DeleteOthers") {
		tester = "1 = 1"
	}

	switch action {
	case "delete":
		rows, err := auth.Gdb.Query("SELECT path FROM images WHERE " + tester +
			" AND id IN (" + strings.Join(ids, ",") + ")")

		if err != nil {
			Return(w, "Err:DB::Select_Failure")
			return
		}

		for rows.Next() {
			var path string
			rows.Scan(&path)

			os.Remove("./images/" + path)
			os.Remove("./thumbs/" + path)
			auth.Gimage.Remove("./images/" + path)
			auth.Gimage.Remove("./thumbs/" + path)
		}
		jids := strings.Join(ids, ",")
		sql := `UPDATE user_info SET image_usage = image_usage - sub.size FROM
		(SELECT 	MAX(uploader) AS id, SUM(size) AS size 
		FROM 		images 
		WHERE 		id IN (` + jids + `) 
		GROUP BY 	uploader) as sub
		WHERE 		user_info.id = sub.id;
		DELETE FROM images WHERE ` + tester + " AND id IN (" + jids + ")"

		if _, err := auth.Gdb.Exec(sql); err == nil {
			Return(w, "ok")
		} else {
			glog.Errorln("Database:", err)
			Return(w, "Err:DB::Delete_Failure")
			return
		}

	case "archive":
		if _, err := auth.Gdb.Exec(`UPDATE images SET archive = true WHERE ` + tester +
			" AND id IN (" + strings.Join(ids, ",") + ")"); err == nil {
			Return(w, "ok")
		} else {
			glog.Errorln("Database:", err)
			Return(w, "Err:DB::Update_Failure")
			return
		}
	case "invert":
		col := auth.CleanString(r.FormValue("column"))
		_, err := auth.Gdb.Exec("UPDATE images SET " + col + " = NOT " + col + " WHERE " + tester +
			" AND id IN (" + strings.Join(ids, ",") + ")")

		if err != nil {
			Return(w, "Err:DB::Update_Failure")
			return
		}

		Return(w, "ok")
	case "rename":
		filename := auth.CleanString("", r.FormValue("filename"))
		// Note here we don't escape filename, escaping is done at auth.GetGallery()
		_, err := auth.Gdb.Exec("UPDATE images SET filename = '" + filename + "' WHERE " + tester +
			" AND id IN (" + strings.Join(ids, ",") + ")")

		if err != nil {
			Return(w, "Err:DB::Update_Failure")
			return
		}

		Return(w, "ok")
	case "flag":
		_, err := auth.Gdb.Exec("UPDATE images SET requests = requests + 1 WHERE id IN (" + strings.Join(ids, ",") + ")")

		if err != nil {
			Return(w, "Err:DB::Update_Failure")
			return
		}

		Return(w, "ok")
	default:
		Return(w, 503)
		return
	}

	if id == 0 {
		auth.Gcache.Remove(`\S+-\d+-img(true|false)`)
		reverseCacheDeletion("all")
		auth.Guser.Clear()
	} else {
		auth.Gcache.Remove(`\S+-` + strconv.Itoa(id) + `-img(true|false)`)
		reverseCacheDeletion(strconv.Itoa(id))
		auth.Guser.Remove(id)
	}
}

func (th ModelHandler) GET_reverse_cache_CACHE(w http.ResponseWriter, r *http.Request, ps httprouter.Params) {
	if r.Header.Get("X-Salt") != conf.GlobalServerConfig.Salt {
		Return(w, 503)
		return
	}

	cache := ps.ByName("cache")
	if cache == "all" {
		auth.Gcache.Remove(`\S+-\d+-img(true|false)`)
		auth.Guser.Clear()
	} else {
		auth.Gcache.Remove(`\S+-(` + cache + `|0)-img(true|false)`)
		id, _ := strconv.Atoi(cache)
		auth.Guser.Remove(id)
	}
}

func (th ModelHandler) POST_search_image(w http.ResponseWriter, r *http.Request, ps httprouter.Params) {
	data := r.FormValue("url")
	if data == "" {
		Return(w, "Err::Post::Invalid_Query")
		return
	}

	url := strings.Split(data, "/")
	_, id := getRealPath(url[len(url)-1])

	if !auth.LogIP(r) {
		Return(w, "Err::Router::Frequent_Access_"+auth.GetIP(r))
		return
	}

	if id == 0 {
		Return(w, fmt.Sprintf("ok::/gallery/%s/search/%s/page/1", r.FormValue("id"), data))
	} else {
		var ts int
		if err := auth.Gdb.QueryRow(`
		SELECT 	ts 
		FROM 	images 
		WHERE  	uploader = ` + strconv.Itoa(id) + ` AND image = '` + url[len(url)-1] + `'`).
			Scan(&ts); err == nil {
			ts = ts + 1
			Return(w, fmt.Sprintf("ok::/gallery/%d/page/before=%s_%s", id, auth.HashTS(ts), auth.To60(uint64(ts))))
		} else {
			Return(w, "Err::DB::Nothing_Found")
		}
	}
}
