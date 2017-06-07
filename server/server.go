package server

import (
	"fmt"
	"io/ioutil"
	"net"
	"net/http"
	"os"
	"os/exec"
	"regexp"
	"strconv"
	"sync"
	"text/template"
	"time"

	"github.com/Sirupsen/logrus"
	"github.com/gorilla/mux"
	"github.com/rancher/go-rancher/api"
	"github.com/rancher/go-rancher/client"
)

const (
	// DefaultMetadataURL ...
	DefaultMetadataURL = "http://rancher-metadata/2016-07-29"
	// DefaultServerPort ...
	DefaultServerPort    = 8080
	defaultHistoryLength = -1
)

// Server ...
type Server struct {
	sync.Mutex
	port          int
	exitCh        chan int
	l             net.Listener
	historyLength int
	logs          map[string]*Log
}

//Error structure contains the error resource definition
type Error struct {
	client.Resource
	Status  int    `json:"status"`
	Message string `json:"message"`
}

// Log ...
type Log struct {
	client.Resource
	FileName    string `json:"filename"`
	State       string `json:"state"`
	DownloadURL string `json:"downloadURL"`
	Self        string `json:"self"`
}

// LogCollection ...
type LogCollection struct {
	client.Collection
	Data []Log `json:"data,omitempty"`
}

var schemas *client.Schemas

func handler(schemas *client.Schemas, f func(http.ResponseWriter, *http.Request)) http.Handler {
	return api.ApiHandler(schemas, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		f(w, r)
	}))
}

//NewRouter creates and configures a mux router
func (s *Server) NewRouter() *mux.Router {
	schemas = &client.Schemas{}

	// ApiVersion
	apiVersion := schemas.AddType("apiVersion", client.Resource{})
	apiVersion.CollectionMethods = []string{}

	// Schema
	schemas.AddType("schema", client.Schema{})

	// Log
	log := schemas.AddType("log", Log{})
	log.CollectionMethods = []string{"GET", "POST"}
	delete(log.ResourceFields, "filename")

	// Error
	err := schemas.AddType("error", Error{})
	err.CollectionMethods = []string{}

	// API framework routes
	router := mux.NewRouter().StrictSlash(true)

	router.HandleFunc("/", s.homePageHandler).Methods("GET")
	router.Methods("GET").Path("/v1").Handler(api.VersionsHandler(schemas, "v1"))
	router.Methods("GET").Path("/v1/schemas").Handler(api.SchemasHandler(schemas))
	router.Methods("GET").Path("/v1/schemas/{id}").Handler(api.SchemaHandler(schemas))

	// Application routes
	router.Methods("GET").Path("/v1/logs").Name("s.ListLogs").Handler(handler(schemas, s.ListLogs))
	router.Methods("GET").Path("/v1/logs/{logId}").Name("s.LoadLogDetails").Handler(handler(schemas, s.LoadLogDetails))
	router.Methods("DELETE").Path("/v1/logs/{logId}").Name("s.DeleteLog").Handler(handler(schemas, s.DeleteLog))
	router.Methods("POST").Path("/v1/logs").Name("s.GenerateLog").Handler(handler(schemas, s.GenerateLog))

	return router
}

var homePageTmpl = `<html>
  <head>
    <title>Rancher Logs Collector</title>
    <script type="text/javascript">
      var logsUrl = "./static/logs/{{ .FileName }}.zip";
      function checkAndDownloadLogs() {
          var count = 0;
          var timerVar;
          var logsReady = false;

          function checkURL(urlToCheck) {

              var request = new XMLHttpRequest();
              request.open('HEAD', urlToCheck, true);
              request.onreadystatechange = function(){
                  if (request.readyState === 4){
                      if (request.status === 404) {
                          console.log("logs are not ready yet");
                          logsReady = false;
                      } else if (request.status === 200) {
                          console.log("logs are ready!");
                          logsReady = true;
                      }
                  }
              };
              request.send();
          }

          function checkIfLogsExist() {
              checkURL(logsUrl);
              if (logsReady) {
                  clearTimeout(timerVar);
                  // do a redirect
                  window.location.replace(logsUrl);
              }
          }
          timerVar = setInterval(checkIfLogsExist, 1000);
      }

      if (window.addEventListener) {
        window.addEventListener("load", checkAndDownloadLogs, false);
      } else if (window.attachEvent) {
        window.attachEvent("onload", checkAndDownloadLogs);
      } else {
          window.onload = checkAndDownloadLogs;
      }
    </script>
  </head>
  <body>
    <h1>Rancher Logs Collector</h1>
    <p>Please wait while the logs are being collected, this will take few minutes.</p>
    <p>Once ready, the download will start automatically.</p>
	<p>Logs should be available <a href="./static/logs/{{ .FileName }}.zip">here</a>.</p>
  </body>
</html>
`

func getLogsOnStart() map[string]*Log {
	logrus.Debugf("finding existing logs upon start")
	logs := make(map[string]*Log)
	files, err := ioutil.ReadDir("/logs")
	if err != nil {
		logrus.Errorf("error reading existing logs: %v", err)
		return logs
	}

	for _, file := range files {
		logrus.Debugf("found log: %v", file.Name())
		re := regexp.MustCompile(`(rancher-logs-(.*)).zip`)
		match := re.FindStringSubmatch(file.Name())
		if len(match) != 3 {
			continue
		}
		logFileID := match[2]
		fileName := match[1]
		log := &Log{
			Resource: client.Resource{
				Id:   logFileID,
				Type: "log",
			},
			FileName:    fileName,
			DownloadURL: fmt.Sprintf("http://%v.zip", fileName),
			State:       "created",
			Self:        logFileID,
		}
		logs[logFileID] = log
	}

	logrus.Debugf("logs: %+v", logs)
	return logs
}

// NewServer ...
func NewServer() (*Server, error) {
	exitCh := make(chan int)

	return &Server{
		port:          DefaultServerPort,
		exitCh:        exitCh,
		historyLength: defaultHistoryLength,
		logs:          getLogsOnStart(),
	}, nil
}

// GetExitChannel ...
func (s *Server) GetExitChannel() chan int {
	return s.exitCh
}

// Close ...
func (s *Server) Close() {
	s.l.Close()
}

// Run ...
func (s *Server) Run() error {
	logrus.SetLevel(logrus.DebugLevel)
	logrus.Infof("Starting webserver on port: %v", s.port)

	router := s.NewRouter()
	fs := http.FileServer(http.Dir("/logs"))
	router.Handle("/static/logs/{log}", http.StripPrefix("/static/logs/", fs))

	l, err := net.Listen("tcp", fmt.Sprintf(":%v", s.port))
	if err != nil {
		logrus.Errorf("error listening: %v", err)
		return err
	}
	s.l = l
	go http.Serve(l, router)

	return nil
}

// ListLogs ...
func (s *Server) ListLogs(w http.ResponseWriter, r *http.Request) {
	logrus.Debugf("Request to List Logs")

	apiContext := api.GetApiContext(r)
	logrus.Debugf("apiContext: %v", apiContext)
	context := apiContext.UrlBuilder.Current()

	resp := LogCollection{}
	s.Lock()
	for _, l := range s.logs {
		log := *l
		log.DownloadURL = fmt.Sprintf("%v/%v.zip", context, log.FileName)
		resp.Data = append(resp.Data, log)
	}
	s.Unlock()
	logrus.Debugf("resp: %+v", resp)

	apiContext.Write(&resp)
}

// DeleteLog ...
func (s *Server) DeleteLog(w http.ResponseWriter, r *http.Request) {
	logrus.Debugf("DeleteLog")
	var errMsg string
	var errStatus int
	ok := true

	defer func() {
		if !ok {
			errorHandlerJSON(w, r, errMsg, errStatus)
		}
	}()

	vars := mux.Vars(r)
	logID, found := vars["logId"]
	if !found {
		logrus.Errorf("logId not found in mux vars")
		ok = false
		errMsg = "Missing paramater log id"
		errStatus = http.StatusBadRequest
		return
	}
	logrus.Debugf("load log: %v", logID)
	s.Lock()
	l, found := s.logs[logID]
	delete(s.logs, logID)
	s.Unlock()

	if !found {
		logrus.Errorf("log with logId: %v not found", logID)
		ok = false
		errMsg = "log not found"
		errStatus = http.StatusNotFound
		return
	}

	go deleteLogFile(l.FileName)
	w.WriteHeader(200)
}

// LoadLogDetails ...
func (s *Server) LoadLogDetails(w http.ResponseWriter, r *http.Request) {
	logrus.Debugf("LoadLogDetails")
	apiContext := api.GetApiContext(r)
	context := apiContext.UrlBuilder.Current()
	var errMsg string
	var errStatus int
	ok := true

	defer func() {
		if !ok {
			errorHandlerJSON(w, r, errMsg, errStatus)
		}
	}()

	vars := mux.Vars(r)
	logID, found := vars["logId"]
	if !found {
		logrus.Errorf("logId not found in mux vars")
		ok = false
		errMsg = "Missing paramater log id"
		errStatus = http.StatusBadRequest
		return
	}
	logrus.Debugf("load log: %v", logID)
	s.Lock()
	resp, found := s.logs[logID]
	s.Unlock()

	if !found {
		logrus.Errorf("log with logId: %v not found", logID)
		ok = false
		errMsg = "log not found"
		errStatus = http.StatusNotFound
		return
	}
	resp.DownloadURL = fmt.Sprintf("%v/%v.zip", context, resp.FileName)
	apiContext.Write(&resp)
}

func (s *Server) appendLog(l *Log) {
	logrus.Debugf("Appending log %v", l)
	s.Lock()
	s.logs[l.Id] = l
	s.Unlock()
}

// NewLog ...
func NewLog() *Log {
	logFileID := fmt.Sprintf("%v", time.Now().UnixNano())
	fileName := fmt.Sprintf("rancher-logs-%v", logFileID)
	log := &Log{
		Resource: client.Resource{
			Id:   logFileID,
			Type: "log",
		},
		FileName: fileName,
		State:    "creating",
		Self:     logFileID,
	}
	logrus.Debugf("NewLog: %v", log)
	return log
}

// GenerateLog ...
func (s *Server) GenerateLog(w http.ResponseWriter, r *http.Request) {
	logrus.Debugf("Request to generate log")
	apiContext := api.GetApiContext(r)

	log := NewLog()
	s.appendLog(log)

	go s.collectLogs(log)

	apiContext.Write(log)
}

func (s *Server) homePageHandler(w http.ResponseWriter, r *http.Request) {
	logrus.Debugf("homePageHandler")

	log := NewLog()
	s.appendLog(log)

	go s.collectLogs(log)

	t := template.Must(template.New("homePageTmpl").Parse(homePageTmpl))
	if err := t.Execute(w, log); err != nil {
		logrus.Errorf("error parsing template: %v", err)
	}
}

func deleteLogFile(logFileName string) {
	logFilePath := fmt.Sprintf("/logs/%v.zip", logFileName)
	logrus.Debugf("deleting log file: %v", logFilePath)

	if err := os.Remove(logFilePath); err != nil {
		logrus.Errorf("error deleting file %v: %v", logFilePath, err)
	}
}

func (s *Server) collectLogs(log *Log) error {
	logrus.Debugf("start: collecting logs %v", log)

	cmd := exec.Command(
		"logs-collector.sh",
		"/logs",
		log.FileName,
		strconv.Itoa(s.historyLength),
	)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		logrus.Errorf("error collecting logs: %v", err)
		return err
	}

	logrus.Debugf("end: collecting logs %v", log.FileName)

	s.Lock()
	log.State = "created"
	s.Unlock()

	return nil
}

func errorHandlerJSON(w http.ResponseWriter, r *http.Request, errMsg string, errStatus int) {
	w.WriteHeader(errStatus)
	errResp := Error{
		Resource: client.Resource{
			Type: "error",
		},
		Status:  errStatus,
		Message: errMsg,
	}

	apiContext := api.GetApiContext(r)
	apiContext.Write(&errResp)
}

func errorHandler(w http.ResponseWriter, r *http.Request, status int) {
	w.WriteHeader(status)
	if status == http.StatusNotFound {
		fmt.Fprint(w, "404")
	}
}
