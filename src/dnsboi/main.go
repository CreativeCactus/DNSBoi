package main

import (
	"fmt"
	"io/ioutil"
	"net/http"
	"os"
	"runtime/debug"
	"strconv"
	"strings"
	"time"

	"github.com/gorilla/mux"
	log "github.com/sirupsen/logrus"
)

func init() {
	defer func() {
		// s := debug.Stack()
		if err := recover(); err != nil {
			(&App{Meta: log.Fields{
				"State": "Fatal",
			}}).Recovering(err, "Init")
		}
	}()

	// Log as JSON instead of the default ANSI formatter.
	log.SetFormatter(&log.JSONFormatter{})
	log.SetOutput(os.Stdout)
	log.SetLevel(log.InfoLevel)
}
func main() {
	meta := log.Fields{
		"App":   "DNSBoi",
		"Start": fmt.Sprintf("%d", time.Now().Unix()),
		"PID":   os.Getpid(),
	}

	defer func() {
		//s := debug.Stack()
		if err := recover(); err != nil {
			(&App{Meta: meta}).Recovering(err, "Main")
		}
	}()

	// Config
	Zonefile, ok := os.LookupEnv("ZONEFILE")
	if !ok {
		Zonefile = "zones"
	}
	Port, ok := os.LookupEnv("PORT")
	if !ok {
		Port = ":3353"
	}
	// Setup
	A := NewApp(meta)
	A.Zonefile = Zonefile
	A.DefaultRemotePort = 8000
	A.Log().Info(fmt.Sprintf("Zonefile at %s", A.Zonefile))
	A.Log().Info(fmt.Sprintf("Listening on %s", Port))
	log.Fatal(http.ListenAndServe(Port, A.Router.router))
}

//  .d8b.  d8888b. d8888b.
// d8' `8b 88  `8D 88  `8D
// 88ooo88 88oodD' 88oodD'
// 88~~~88 88~~~   88~~~
// 88   88 88      88
// YP   YP 88      88

// NewApp is responsible for the overall coordination of the App
// (which is passed around in a dependency injection manner)
func NewApp(meta log.Fields) *App {
	A := App{
		// Metadata to include in logs
		Meta:                  meta,
		ServiceDB:             map[string]Service{},
		ServiceErrorThreshold: 5,
	}

	/* HTTP */

	mr := mux.NewRouter()
	R := Router{
		router: mr,
	}
	mr.HandleFunc("/health", A.HealthCheck).Methods("GET")

	mr.Path("/register").Queries("key", "{key}").HandlerFunc(
		A.SetContext(A.LoggerMiddleware(A.RegisterHandler)),
	).Name("Register")

	/* Services */

	setInterval(func(t time.Time) {
		A.Log().Info(fmt.Sprintf("%v:%d services", t, len(A.ServiceDB)))
		A.HealthCheckZones()
		A.PruneZones()
		A.WriteZonesFile()
	}, 15000)

	/* Done */

	A.Router = R
	return &A
}

// RegisterHandler will handle the registration endpoint
func (A *App) RegisterHandler(ctx *HTTPCtx) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		key := r.FormValue("key")
		port, err := strconv.Atoi(r.FormValue("port"))
		if err != nil {
			port = A.DefaultRemotePort
		}

		A.AddServices(&map[string]Service{
			key: Service{
				Name: key,
				IP:   r.RemoteAddr,
				Port: port,
			},
		})

		A.Log(log.Fields{
			"Name": key,
			"IP":   r.RemoteAddr,
			"Port": port,
		}).Info(`Discovery`)
	}
}

// HealthCheck will return a 200, hopefully
func (A *App) HealthCheck(w http.ResponseWriter, r *http.Request) {
	w.WriteHeader(200)
	w.Write([]byte(`{"status":"OK"}`))
}

//  .o88b.  .d88b.  d8888b. d88888b
// d8P  Y8 .8P  Y8. 88  `8D 88'
// 8P      88    88 88oobY' 88ooooo
// 8b      88    88 88`8b   88~~~~~
// Y8b  d8 `8b  d8' 88 `88. 88.
//  `Y88P'  `Y88P'  88   YD Y88888P

func (A *App) AddServices(services *map[string]Service) {
	newDB := map[string]Service{}

	fmt.Println("Adding", len(*services), "services")

	for key, service := range A.ServiceDB {
		if service.Errors < A.ServiceErrorThreshold {
			newDB[key] = service
		}
	}

	for key, service := range *services {
		if service.Errors < A.ServiceErrorThreshold {
			newDB[key] = service
		}
	}

	A.ServiceDB = newDB
}
func (A *App) PruneZones() {
	newDB := map[string]Service{
	// "debug": Service{
	// 	Name: "debug",
	// 	IP:   "127.0.0.1",
	// 	Port: 8000,
	// },
	}

	for key, service := range A.ServiceDB {
		if service.Errors < A.ServiceErrorThreshold {
			newDB[key] = service
		}
	}

	A.ServiceDB = newDB
}
func (A *App) HealthCheckZones() {
	for _, service := range A.ServiceDB {
		res, err := http.Get(fmt.Sprintf(`http://%s:%d/health`, service.IP, service.Port))
		if err != nil || res.StatusCode != 200 {
			service.Errors++
		} else {
			service.Errors = 0
		}
	}
}
func (A *App) WriteZones() string {
	t := time.Now()
	// Serial format for SOA is YYSSSSSSSS where S is seconds into the year
	// Are SOA serials necessarily sequential? I don't think so, but this might
	// cause issues in 2099 on New Year's Eve. If so I appologise.
	// Actually, SOASN is 32bit unsigned, so it's more likely to be a problem
	// some time around New Year's Day 2043, shortly after it ticks over from
	// 4231556926 to 4300000000 (>2^32-1) and the devs are awoken by an issue
	// which seemed to be at least 68443074 seconds away (in theory).
	// Again, I'm sorry. I promise to get back to this in the next 25 years,
	// but right now I really gotta sleep, man.
	serial := fmt.Sprintf("%02d%08d",
		t.Year()%100,
		(t.YearDay()*24+t.Hour())*3600+t.Minute()*60+t.Second(),
	)

	zones := fmt.Sprintf(`$ORIGIN example.net.
@	3600 IN	SOA sns.dns.icann.org. noc.dns.icann.org. (
				%s ; serial
				7200       ; refresh (2 hours)
				3600       ; retry (1 hour)
				1209600    ; expire (2 weeks)
				3600       ; minimum (1 hour)
				)

	3600 IN NS a.iana-servers.net.
	3600 IN NS b.iana-servers.net.

www     IN A     127.0.0.1
		IN AAAA  ::1
`, serial)
	for _, service := range A.ServiceDB {
		zones = fmt.Sprintf(`%s%s`, zones, service.WriteZone())
	}

	fmt.Println(zones)
	fmt.Printf("%d services...\n", len(A.ServiceDB))

	return zones
}
func (A *App) WriteZonesFile() {
	err := ioutil.WriteFile(A.Zonefile, []byte(A.WriteZones()), 0644)
	if err != nil {
		A.Log().Info(fmt.Sprintf("Err:WriteFile:%s:%s", A.Zonefile, err.Error()))
	}
}
func (S Service) WriteZone() string {
	return fmt.Sprintf(`
%s      IN A     %s
		IN AAAA  %s`, S.Name, S.IP, "::1")
}

// d888888b db    db d8888b. d88888b .d8888.
// `~~88~~' `8b  d8' 88  `8D 88'     88'  YP
//    88     `8bd8'  88oodD' 88ooooo `8bo.
//    88       88    88~~~   88~~~~~   `Y8b.
//    88       88    88      88.     db   8D
//    YP       YP    88      Y88888P `8888Y'

// App is the basic reference to the DB and request scope used by API handlers.
type App struct {
	Router                Router
	Meta                  log.Fields
	ServiceDB             map[string]Service
	Zonefile              string
	ServiceErrorThreshold int32
	DefaultRemotePort     int
}
type Service struct {
	Name   string
	IP     string
	Port   int
	Errors int32
}

// Log wraps the common logging pattern of merging app metadata,
// feeding to logrus and calling .Info or .Warn.
// NOTE THAT ONE OF THESE MUST BE CALLED IN ORDER TO ACTUALLY LOG
func (A *App) Log(data ...log.Fields) *log.Entry {
	return log.WithFields(Merge(A.Meta, Merge(data...)))
}

type loggingResponseWriter struct {
	http.ResponseWriter
	statusCode int
}

func (lrw *loggingResponseWriter) WriteHeader(code int) {
	lrw.statusCode = code
	lrw.ResponseWriter.WriteHeader(code)
}

// Router represents a network interface with optional metadata
type Router struct {
	router *mux.Router
}

// Frontend is a middleware which presents a CtxHandler chain as a HandlerFunc
type Frontend func(CtxHandler) http.HandlerFunc

// Middleware chains CtxHandlers
type Middleware func(CtxHandler) CtxHandler

// CtxHandler is a type of curried HandlerFunc supporting request scoped context
type CtxHandler func(*HTTPCtx) http.HandlerFunc

// HTTPCtx represents relevant metadata for the lifetime of a request
type HTTPCtx struct {
	Data     map[string]interface{}
	Message  string
	URL      string
	Status   string
	Duration string
	Start    time.Time `json:"-"`
}

// db   db d88888b db      d8888b. d88888b d8888b.
// 88   88 88'     88      88  `8D 88'     88  `8D
// 88ooo88 88ooooo 88      88oodD' 88ooooo 88oobY'
// 88~~~88 88~~~~~ 88      88~~~   88~~~~~ 88`8b
// 88   88 88.     88booo. 88      88.     88 `88.
// YP   YP Y88888P Y88888P 88      Y88888P 88   YD

// Recovering is how we overcome and report on panics
func (A *App) Recovering(err interface{}, where string) {
	why := strings.Split(fmt.Sprintf("%s", err), "\n")
	data := log.Fields{
		"message": fmt.Sprintf("PANICKED: %s", where),
		"status":  "PANIC",
		"stack":   why,
	}

	A.Log(data).Warn()
}

// Merge maps or Fields together
func Merge(maps ...log.Fields) log.Fields {
	out := log.Fields{}
	for _, m := range maps {
		for k, v := range m {
			out[k] = v
		}
	}
	return out
}

// CtxToLogfields does what it says on the label
func CtxToLogfields(ctx *HTTPCtx) log.Fields {
	return log.Fields{
		"url":      ctx.URL,
		"status":   ctx.Status,
		"message":  ctx.Message,
		"duration": ctx.Duration,
	}
}

func setInterval(f func(time.Time), interval int64) {
	ticker := time.NewTicker(time.Millisecond * time.Duration(interval))
	go func() {
		for e := range ticker.C {
			go f(e)
		}
	}()
}

// .88b  d88. db   d8b   db  .d8b.  d8888b. d88888b
// 88'YbdP`88 88   I8I   88 d8' `8b 88  `8D 88'
// 88  88  88 88   I8I   88 88ooo88 88oobY' 88ooooo
// 88  88  88 Y8   I8I   88 88~~~88 88`8b   88~~~~~
// 88  88  88 `8b d8'8b d8' 88   88 88 `88. 88.
// YP  YP  YP  `8b8' `8d8'  YP   YP 88   YD Y88888P

// SetContext is a middleware frontend responsible for presenting a
// HandlerFunc to the router which will configure a context
// on any incoming requests and feed that down to other middlewares
func (A *App) SetContext(first CtxHandler) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		defer func() {
			s := debug.Stack()
			if err := recover(); err != nil {
				A.Recovering(s, "HTTP")
				http.Error(w, "Server error", http.StatusInternalServerError)
			}
		}()

		w.Header().Set("Cache-Control", "no-cache, private, max-age=0")
		w.Header().Set("Pragma", "no-cache")
		w.Header().Set("X-Accel-Expires", "0")

		context := HTTPCtx{
			Data: map[string]interface{}{},
			URL:  r.RequestURI,
		}
		first(&context).ServeHTTP(w, r)
	}
}

// LoggerMiddleware is a middleware for reporting on traffic
func (A *App) LoggerMiddleware(next CtxHandler) CtxHandler {
	defaults := log.Fields{}

	return func(ctx *HTTPCtx) http.HandlerFunc {
		return func(w http.ResponseWriter, r *http.Request) {
			ctx.Start = time.Now()

			lrw := &loggingResponseWriter{w, 0}
			next(ctx).ServeHTTP(lrw, r)

			ctx.Status = fmt.Sprintf("%d", lrw.statusCode)
			ctx.Duration = fmt.Sprintf("%d", time.Since(ctx.Start))
			data := Merge(A.Meta, defaults, CtxToLogfields(ctx))
			log.WithFields(data).Info()
		}
	}
}
