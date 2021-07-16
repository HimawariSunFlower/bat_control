package main

import (
	"embed"
	"fmt"
	"io/ioutil"
	"net"
	"net/http"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"reflect"
	"regexp"
	"strings"
	"unsafe"

	"github.com/djimenez/iconv-go"
	"github.com/spf13/viper"
	"github.com/zserge/lorca"
)

const (
	server = "server"
)

type mapStr map[string]*Bat

type RespData struct {
	Id      string
	Name    string
	Remarks string
}

type Bat struct {
	Remarks string
	Path    string
	Name string
	Mark bool
}

var (
	//go:embed static/*
	fs      embed.FS
	mycache mapStr
)

func (m mapStr) GetPushData(typ int) []*RespData {
	push := []*RespData{}
	for k, v := range m {
		if typ == 1 || v.Mark {
			push = append(push, &RespData{Id: k, Name: v.Name, Remarks: v.Remarks})
		}
	}
	return push
}

func main() {
	mux := http.DefaultServeMux
	args := []string{}

	viper := viper.New()
	viper.SetConfigType("toml")
	viper.SetConfigFile("configs/config.toml")
	viper.ReadInConfig()
	mycache = make(map[string]*Bat)

	Marks := viper.GetStringMapString("server")
	exePath := GetProjectAbsPath(viper.GetString("exePath"))

	for _, v := range viper.GetStringSlice("batPath") {
		files, _ := ioutil.ReadDir(exePath + v)
		for _, f := range files {
			if strings.HasSuffix(f.Name(), ".bat") {
				mid := &Bat{Name: f.Name(), Path: exePath + v}
				if val, ok := Marks[f.Name()]; ok {
					mid.Mark = true
					mid.Remarks = val
				}
				mycache[f.Name()] = mid
			}
		}
	}

	ui, err := lorca.New("", "", 1024, 500, args...)
	if err != nil {
		panic(err)
	}
	defer ui.Close()
	mux.Handle("/", http.FileServer(http.FS(fs)))
	mux.HandleFunc("/build", build)
	mux.HandleFunc("/show", show)
	mux.HandleFunc("/open", openDir)
	mux.HandleFunc("/edit", edit)

	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		panic(err)
	}
	defer ln.Close()

	go http.Serve(ln, mux)

	ui.Load(fmt.Sprintf("http://%s/static/bat.html", ln.Addr()))
	ui.Bind("getData", mycache.GetPushData)
	ui.Bind("url", ln.Addr())

	sigc := make(chan os.Signal)
	signal.Notify(sigc, os.Interrupt)
	select {
	case <-sigc:
	case <-ui.Done():
	}
}

//执行bat
func build(w http.ResponseWriter, r *http.Request) {
	key := r.URL.RawQuery
	val, ok := mycache[key]
	if !ok {
		return
	}
	batPath := val.Path + val.Name

	w.Header().Set("Content-Type", "charset=utf-8")
	w.WriteHeader(http.StatusOK)
	cmd := exec.Command(`cmd.exe`, `/C`, "start "+batPath) //初始化Cmd
	cmd.Dir = val.Path
	cmd.Run()
}

//显示内容
func show(w http.ResponseWriter, r *http.Request) {
	key := r.URL.RawQuery
	val, ok := mycache[key]
	if !ok {
		return
	}
	mid := val.Path + val.Name
	resp, err := os.ReadFile(mid)
	if err != nil {
		d_v, _ := iconv.ConvertString(err.Error(), "GB2312", "utf-8")
		fmt.Println(d_v)
	}
	w.Header().Set("Content-Type", "charset=utf-8")
	w.Write(resp)
}

//打开文件所在目录 only wins
func openDir(w http.ResponseWriter, r *http.Request) {
	key := r.URL.RawQuery
	val, ok := mycache[key]
	if !ok {
		return
	}
	mid := val.Path
	re := regexp.MustCompile(`\/`)
	rep := re.ReplaceAllString(mid, `\`)
	//fmt.Println(rep)

	err := exec.Command(`cmd`, `/c`, `explorer`, rep).Start()
	if err != nil {
		fmt.Println(err.Error())
	}
}

//旧文件删除,新文件覆盖
func edit(w http.ResponseWriter, r *http.Request) {
	r.ParseForm() //必须先解析
	data := r.PostForm.Get("value")
	//fmt.Println(data)

	key := r.URL.RawQuery
	val, ok := mycache[key]
	if !ok {
		return
	}
	mid := val.Path + val.Name

	f, err := os.OpenFile(mid, os.O_WRONLY|os.O_TRUNC, 0600)

	defer f.Close()
	if err != nil {
		fmt.Println(err.Error())
	}
	_, err = f.WriteString(data)
	if err != nil {
		fmt.Println(err.Error())
	}
	if err == nil {
		w.Write(StringBytes("编辑成功"))
	} else {
		w.Write(StringBytes("error"))
	}

}

func BytesString(b []byte) string {
	return *(*string)(unsafe.Pointer(&b))
}

func StringBytes(s string) []byte {
	stringHeader := (*reflect.StringHeader)(unsafe.Pointer(&s))
	sliceHeader := &reflect.SliceHeader{
		Data: stringHeader.Data,
		Cap:  stringHeader.Len,
		Len:  stringHeader.Len,
	}
	return *(*[]byte)(unsafe.Pointer(sliceHeader))
}

func GetProjectAbsPath(param string) string {
	pwd, _ := os.Getwd()
	return filepath.Join(pwd, param)
}
