package main

import (
	"fmt"
	"net/http"
	"os"
	"runtime/debug"
	"strings"
	"time"

	"github.com/gorilla/mux"
	log "github.com/sirupsen/logrus"
)

func init() {
	defer func() {
		// s := debug.Stack()
		if err := recover(); err != nil {
			App{Meta: log.Fields{
				"State": "Fatal",
			}}.Recovering(err, "Init")
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
			App{Meta: meta}.Recovering(err, "Main")
		}
	}()

	// Config
	Port, ok := os.LookupEnv("PORT")
	if !ok {
		Port = ":8000"
	}
	// Setup
	App := NewApp(meta)
	App.Log(log.Fields{}).Info(fmt.Sprintf("Listening on %s", Port))
	log.Fatal(http.ListenAndServe(Port, App.Router.router))
}

// NewApp is responsible for the overall coordination of the App
// (which is passed around in a dependency injection manner)
func NewApp(meta log.Fields) App {
	A := App{
		// Metadata to include in logs
		Meta: meta,
	}

	mr := mux.NewRouter()
	R := Router{
		router: mr,
	}
	mr.HandleFunc("/health", A.HealthCheck).Methods("GET")

	mr.Path("/register").Queries("key", "{key}").HandlerFunc(
		A.SetContext(A.LoggerMiddleware(A.RegisterHandler)),
	).Name("Register")

	return App{
		Router: R,
	}
}

// RegisterHandler will handle the registration endpoint
func (A App) RegisterHandler(ctx *HTTPCtx) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		key := r.FormValue("key")

		A.Log(CtxToLogfields(ctx)).Info(key)
	}
}

// HealthCheck will return a 200, hopefully
func (A App) HealthCheck(w http.ResponseWriter, r *http.Request) {
	w.WriteHeader(200)
	w.Write([]byte(`{"status":"OK"}`))
}

// d888888b db    db d8888b. d88888b .d8888.
// `~~88~~' `8b  d8' 88  `8D 88'     88'  YP
//    88     `8bd8'  88oodD' 88ooooo `8bo.
//    88       88    88~~~   88~~~~~   `Y8b.
//    88       88    88      88.     db   8D
//    YP       YP    88      Y88888P `8888Y'

// App is the basic reference to the DB and request scope used by API handlers.
type App struct {
	Router Router
	Meta   log.Fields
}

// Log wraps the common logging pattern of merging app metadata,
// feeding to logrus and calling .Info or .Warn.
// NOTE THAT ONE OF THESE MUST BE CALLED IN ORDER TO ACTUALLY LOG
func (A App) Log(data log.Fields) *log.Entry {
	return log.WithFields(Merge(A.Meta, data))
}

type loggingResponseWriter struct {
	http.ResponseWriter
	statusCode int
}

func (lrw *loggingResponseWriter) WriteHeader(code int) {
	lrw.statusCode = code
	lrw.ResponseWriter.WriteHeader(code)
}

type Router struct {
	router *mux.Router
}
type Frontend func(CtxHandler) http.HandlerFunc
type Middleware func(CtxHandler) CtxHandler
type CtxHandler func(*HTTPCtx) http.HandlerFunc
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
func (A App) Recovering(err interface{}, where string) {
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

// .88b  d88. db   d8b   db  .d8b.  d8888b. d88888b
// 88'YbdP`88 88   I8I   88 d8' `8b 88  `8D 88'
// 88  88  88 88   I8I   88 88ooo88 88oobY' 88ooooo
// 88  88  88 Y8   I8I   88 88~~~88 88`8b   88~~~~~
// 88  88  88 `8b d8'8b d8' 88   88 88 `88. 88.
// YP  YP  YP  `8b8' `8d8'  YP   YP 88   YD Y88888P

// SetContext is a middleware frontend responsible for presenting a
// HandlerFunc to the router which will configure a context
// on any incoming requests and feed that down to other middlewares
func (A App) SetContext(first CtxHandler) http.HandlerFunc {
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
func (A App) LoggerMiddleware(next CtxHandler) CtxHandler {
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
