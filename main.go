package main

import (
	"./auth"
	"./conf"
	"./models"

	"github.com/golang/glog"
	"github.com/julienschmidt/httprouter"
	_ "github.com/lib/pq"

	"flag"
	"fmt"
	"math/rand"
	"net/http"
	"os"
	"os/exec"
	"os/signal"
	"reflect"
	"regexp"
	"strconv"
	"strings"
	"syscall"
	"time"
)

var configPath = flag.String("c", "./config.json", "Load config from file")
var logPath = flag.String("l", "./log", "Log saving directory for glog, alias of '-log_dir'")
var debugMode = flag.Bool("d", false, "Debug mode")
var debugPort = flag.Int("debug-port", 731, "Debug server port")
var testHash = flag.String("test-hash", "", "Make a test hash")

func main() {
	flag.Parse()

	hostname, _ := exec.Command("hostname").Output()
	models.ServerHostname = strings.Replace(string(hostname), "\n", "", -1)

	conf.LoadConfig(*configPath, nil)

	if *debugMode {
		flag.Lookup("logtostderr").Value.Set("true")
	} else {
		flag.Lookup("log_dir").Value.Set(*logPath)
	}

	glog.Infoln("Load config:", *configPath)
	conf.GlobalServerConfig.ConfigPath = *configPath

	if *testHash != "" {
		fmt.Println("Test hash result:", auth.MakeHash(*testHash))
		return
	}

	models.LoadTemplates()

	sigs := make(chan os.Signal, 1)
	signal.Notify(sigs, syscall.SIGHUP)
	go func() {
		for {
			sig := <-sigs
			glog.Infoln("Config reloading", sig)
			conf.LoadConfig(*configPath, auth.Gdb)
		}
	}()

	go func() {
		re := regexp.MustCompile(`((\d|\.)+)\,\s((\d|\.)+)\,\s((\d|\.)+)`)

		for {

			buf, _ := exec.Command("uptime").Output()
			models.ServerLoad = (re.FindAllStringSubmatch(string(buf), -1)[0][1])
			time.Sleep(1 * time.Minute)
		}
	}()

	mux := http.NewServeMux()
	models.ServerStartUp = time.Now()

	auth.ConnectDatabase("postgres", conf.GlobalServerConfig.Connect)
	defer auth.Gdb.Close()
	auth.Salt = conf.GlobalServerConfig.Salt

	auth.Gdb.QueryRow("SELECT version()").Scan(&models.DatabaseVersion)
	conf.GlobalServerConfig.InitTags(auth.Gdb)

	// Access deamon: Log abnormal rapid accessing actions and ban it
	go auth.AccessDaemon()
	go auth.ArticleCounter()

	// PAGE: Serve robots.txt for search engines
	mux.Handle("/robots.txt", http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte(`User-agent: *
Disallow: /playground
Disallow: /account
Disallow: /account/register
Disallow: /tag/`))
	}))

	mux.Handle("/loaderio-d3ff4956e2d53b2ce98c21a717554dbc.html", http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte("loaderio-d3ff4956e2d53b2ce98c21a717554dbc"))
	}))
	mux.Handle("/images/", http.StripPrefix("/images/", http.FileServer(http.Dir("./images"))))
	mux.Handle("/thumbs/", http.StripPrefix("/thumbs/", http.FileServer(http.Dir("./thumbs"))))
	mux.Handle("/assets/", http.StripPrefix("/assets/", http.FileServer(http.Dir("./assets"))))

	router := httprouter.New()

	glog.Infoln("Installing routers...")
	_start := time.Now()

	mhd := reflect.TypeOf(models.ModelHandlerDummy)
	mhv := reflect.ValueOf(models.ModelHandlerDummy)
	re := regexp.MustCompile(`([A-Z]+)`)

	for i := 0; i < mhd.NumMethod(); i++ {
		methodName := mhd.Method(i).Name
		handler := mhv.MethodByName(methodName).Interface().(func(w http.ResponseWriter, r *http.Request, ps httprouter.Params))
		method := strings.Split(methodName, "_")[0]

		routerPath := re.ReplaceAllStringFunc(methodName[len(method):],
			func(s string) string {
				return ":" + strings.ToLower(s)
			})

		routerPath = strings.Replace(routerPath, "_", "/", -1)
		glog.Infoln(fmt.Sprintf("%5s -> %s", method, routerPath))

		// if *accessLog {
		router.Handle(method, routerPath,
			func(w http.ResponseWriter, r *http.Request, ps httprouter.Params) {
				if conf.GlobalServerConfig.AccessLogging {
					referer := strings.Replace(r.Referer(), conf.GlobalServerConfig.Host, "", -1)
					referer = strings.Replace(referer, conf.GlobalServerConfig.DebugHost, "", -1)

					ip := auth.GetIP(r)
					url := strings.Split(r.URL.String(), "?")[0]

					info := ip
					if cookie, err := r.Cookie("uid"); err == nil {
						cookies := strings.Split(cookie.Value, ":")

						if len(cookies) >= 2 {
							info = cookies[0] + "." + cookies[1] + "." + info
						} else {
							info = "0.invalid." + info
						}
					} else {
						info = "guest." + info
					}

					glog.Infoln(info, referer, "->", url)
				}

				handler(w, r, ps)
			})
	}

	mux.Handle("/", router)
	glog.Infoln("Routers installed in", time.Now().Sub(_start).Nanoseconds()/1e6, "ms")

	// _start = time.Now()
	s1 := rand.NewSource(time.Now().UnixNano())
	rand.New(s1)
	// tags := conf.GlobalServerConfig.GetComplexTags()

	// for i := 0; i < 1000; i++ {
	// 	go func(idx int) {
	// 		// glog.Infoln(models.Article(nil,
	// 		// 	auth.DummyUsers[_rand.Intn(5)],
	// 		// 	0,
	// 		// 	tags[_rand.Intn(5)].Name,
	// 		// 	"test"+strconv.Itoa(idx),
	// 		// 	auth.MakeHash()))
	// 		glog.Infoln(models.UpdateArticle(
	// 			auth.DummyUsers[_rand.Intn(5)],
	// 			_rand.Intn(60)+195,
	// 			tags[_rand.Intn(5)].Name,
	// 			"new"+strconv.Itoa(idx),
	// 			"new"+auth.MakeHash()))
	// 	}(i)
	// }
	// glog.Infoln("Routers installed in", time.Now().Sub(_start).Nanoseconds()/1e6, "ms")

	if *debugMode {

		conf.GlobalServerConfig.MainCSS = "main.css"
		conf.GlobalServerConfig.MainJS = "main.js"

		glog.Infoln("Start debug server on", *debugPort)
		glog.Fatalln(http.ListenAndServe(":"+strconv.Itoa(*debugPort), mux))
	} else {
		glog.Infoln("Start HTTP server on", conf.GlobalServerConfig.Listen)
		glog.Fatalln(http.ListenAndServe("127.0.0.1:"+conf.GlobalServerConfig.Listen, mux))
	}
	// }

}
