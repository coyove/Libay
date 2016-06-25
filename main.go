package main

import (
	"./auth"
	"./conf"
	"./models"
	"crypto/sha1"
	"encoding/json"
	"fmt"
	"github.com/golang/glog"
	"github.com/julienschmidt/httprouter"
	"github.com/kardianos/osext"
	_ "github.com/lib/pq"
	// "io"
	"flag"
	// "go/ast"
	// "go/parser"
	// "go/token"
	// "html"
	"io/ioutil"
	"math/rand"
	"net/http"
	"os"
	"os/exec"
	"os/signal"
	"reflect"
	"regexp"
	// "runtime"
	"strconv"
	"strings"
	"syscall"
	// "text/template"
	"time"
)

var configPath = flag.String("c", "./config.json", "Load config from file")
var logPath = flag.String("l", "./log", "Log directory")
var debugMode = flag.Bool("d", false, "Debug mode")

func main() {

	flag.Parse()
	filename, _ := osext.Executable()
	exebuf, _ := ioutil.ReadFile(filename)
	models.ServerChecksum = fmt.Sprintf("%x", sha1.Sum(exebuf))[:8]

	conf.LoadConfig(*configPath, nil)

	if *debugMode {
		flag.Lookup("logtostderr").Value.Set("true")
	} else {
		flag.Lookup("log_dir").Value.Set(*logPath)
	}

	glog.Infoln("Load config:", *configPath)

	conf.GlobalServerConfig.ConfigPath = *configPath
	configBuf, _ := json.Marshal(conf.GlobalServerConfig)

	models.ConfigChecksum = fmt.Sprintf("%x", sha1.Sum(configBuf))[:8]
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
			tmp, _ := strconv.ParseFloat(models.ServerLoad, 64)
			models.ServerLoadi += tmp
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

	// HANDLER: Login phase 1
	//          Client posts a username and server returns an exchange session key
	mux.Handle("/login/phase2", &auth.LoginPhase2Handler{})

	// HANDLER: Login phase 2
	//			Client posts a username and a password, server returns a token
	mux.Handle("/login/phase1", &auth.LoginPhase1Handler{})

	// HANDLER: Register
	mux.Handle("/register", &auth.RegisterHandler{})

	// HANDLER: Logout
	mux.Handle("/logout", &auth.LogoutHandler{})

	// Begin statis files handling
	mux.Handle("/images/", http.StripPrefix("/images/", http.FileServer(http.Dir("./images"))))
	mux.Handle("/thumbs/", http.StripPrefix("/thumbs/", http.FileServer(http.Dir("./thumbs"))))
	mux.Handle("/assets/", http.StripPrefix("/assets/", http.FileServer(http.Dir("./assets"))))
	// End statis files handling

	router := httprouter.New()

	// for i := 1; i < 1000; i++ {
	s1 := rand.NewSource(time.Now().UnixNano())
	rand.New(s1)

	glog.Infoln("Installing routers...")
	_start := time.Now()

	// if files, err := ioutil.ReadDir("./models"); err == nil {
	// for i, file := range files {
	// 	routers := AddRouter(router, "./models/"+file.Name())
	// 	var _pad, pad string
	// 	if i < len(files)-1 {
	// 		_pad = "  │"
	// 		glog.Infof("  ├─ %s(%d):", file.Name(), len(routers))
	// 	} else {
	// 		_pad = "   "
	// 		glog.Infof("  └─ %s(%d):", file.Name(), len(routers))
	// 	}
	// 	for i, v := range routers {
	// 		if i == 0 && len(routers) > 1 {
	// 			pad = "├"
	// 		} else if i == len(routers)-1 {
	// 			pad = "└"
	// 		} else {
	// 			pad = "├"
	// 		}

	// 		glog.Infoln(_pad+"  "+pad+"─", v)
	// 	}

	// }

	mhd := reflect.TypeOf(models.ModelHandlerDummy)
	mhv := reflect.ValueOf(models.ModelHandlerDummy)
	for i := 0; i < mhd.NumMethod(); i++ {
		methodName := mhd.Method(i).Name
		handler := mhv.MethodByName(methodName).Interface()
		method := strings.Split(methodName, "_")[0]

		routerPath := regexp.MustCompile(`([A-Z]+)`).ReplaceAllStringFunc(methodName[len(method):],
			func(s string) string {
				return ":" + strings.ToLower(s)
			})

		routerPath = strings.Replace(routerPath, "_", "/", -1)
		glog.Infoln(fmt.Sprintf("%5s -> %s", method, routerPath))

		router.Handle(method, routerPath, handler.(func(w http.ResponseWriter, r *http.Request, ps httprouter.Params)))
	}

	mux.Handle("/", router)
	glog.Infoln("Routers loaded in", time.Now().Sub(_start).Nanoseconds()/1e6, "ms")

	if *debugMode {
		// for i, t := range conf.GlobalServerConfig.GetComplexTags() {
		// 	buf, _ := json.Marshal(t.PermittedTo)
		// rows, _ := auth.Gdb.Query("select id, content from articles where id < 112")
		// for rows.Next() {
		// 	var tmp string
		// 	var id int
		// 	rows.Scan(&id, &tmp)
		// 	tmp = html.UnescapeString(tmp)
		// 	tmp = models.ExtractContent(tmp)
		// 	tmp = html.EscapeString(tmp)
		// 	auth.Gdb.Exec("update articles set preview='" + tmp + "' where id=" + strconv.Itoa(id))
		// }

		// 	log.Println(err)
		// }

		glog.Infoln("Debug server on 731")
		glog.Fatalln(http.ListenAndServe(":731", mux))
	} else {
		glog.Infoln("Start HTTP Server:", conf.GlobalServerConfig.Listen)
		glog.Fatalln(http.ListenAndServe("127.0.0.1:"+conf.GlobalServerConfig.Listen, mux))
	}
	// }

}

// func AddRouter(r *httprouter.Router, p string) []string {
// 	buf, _ := ioutil.ReadFile(p)
// 	fset := token.NewFileSet()
// 	f, err := parser.ParseFile(fset, "", string(buf), 0)
// 	if err != nil {
// 		panic(err)
// 	}
// 	// ast.Print(fset, f)

// 	ret := make([]string, 0)
// 	ast.Inspect(f, func(n ast.Node) bool {
// 		var s string
// 		switch x := n.(type) {
// 		case *ast.FuncDecl:
// 			s = x.Name.Name
// 			if x.Recv != nil && len(x.Recv.List) > 0 {
// 				t := fmt.Sprintf("%s", reflect.ValueOf(x.Recv.List[0].Type).Elem().Field(1))

// 				if t == "__ModelHandler" {
// 					method := strings.Split(s, "_")[0]

// 					router := regexp.MustCompile(`([A-Z]+)`).ReplaceAllStringFunc(s[len(method):], func(s string) string {
// 						return ":" + strings.ToLower(s)
// 					})

// 					router = strings.Replace(router, "_", "/", -1)
// 					handler := reflect.ValueOf(&models.ModelHandlerDummy).MethodByName(s).Interface()

// 					r.Handle(method, router, handler.(func(w http.ResponseWriter, r *http.Request, ps httprouter.Params)))

// 					ret = append(ret, fmt.Sprintf("%5s %s", method, router))
// 				}
// 			}
// 		}
// 		return true
// 	})

// 	return ret
// }
