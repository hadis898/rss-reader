package main

import (
	"encoding/json"
	"html/template"
	"io"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"rss-reader/globals"
	"rss-reader/models"
	"strings"

	"rss-reader/utils"
	"time"

	"github.com/gorilla/websocket"
)

func init() {
	globals.Init()
}

func main() {
	go utils.UpdateFeeds()
	go utils.WatchConfigFileChanges("config.json")
	http.HandleFunc("/feeds", getFeedsHandler)
	http.HandleFunc("/ws", wsHandler)
	http.HandleFunc("/admin", adminPageHandler)
	http.HandleFunc("/api/admin/config", adminConfigHandler)
	http.HandleFunc("/api/admin/test-source", adminTestSourceHandler)
	// http.HandleFunc("/", serveHome)
	http.HandleFunc("/", tplHandler)

	//加载静态文件
	devStaticDir := strings.TrimSpace(os.Getenv("DEV_STATIC_DIR"))
	if devStaticDir != "" {
		log.Printf("dev static mode enabled: %s", devStaticDir)
		fs := http.StripPrefix("/static/", http.FileServer(http.Dir(devStaticDir)))
		http.Handle("/static/", fs)
	} else {
		fs := http.FileServer(http.FS(globals.DirStatic))
		http.Handle("/static/", fs)
	}
	log.Fatal(http.ListenAndServe(":8080", nil))
}

type adminConfigResponse struct {
	Values         []string `json:"values"`
	Keywords       []string `json:"keywords,omitempty"`
	ReFresh        int      `json:"refresh"`
	AutoUpdatePush int      `json:"autoUpdatePush"`
	NightStartTime string   `json:"nightStartTime"`
	NightEndTime   string   `json:"nightEndTime"`
	Notify         struct {
		Feishu struct {
			API string `json:"api,omitempty"`
		} `json:"feishu,omitempty"`
		Telegram struct {
			API    string `json:"api,omitempty"`
			ChatID string `json:"chat_id,omitempty"`
			Token  string `json:"token,omitempty"`
		} `json:"telegram,omitempty"`
	} `json:"notify,omitempty"`
	Archives      string `json:"archives,omitempty"`
	HasAdminToken  bool     `json:"hasAdminToken"`
}

type adminTestSourceRequest struct {
	URL string `json:"url"`
}

type adminTestSourceResponse struct {
	Title string `json:"title"`
	Items int    `json:"items"`
}

func adminPageHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}
	w.Header().Add("Content-Type", "text/html; charset=utf-8")
	if content, err := readStaticFile("admin.html"); err == nil {
		w.Write(content)
		return
	}
	http.Error(w, "admin page not found", http.StatusNotFound)
}

func adminConfigHandler(w http.ResponseWriter, r *http.Request) {
	if !adminAuthorized(r) {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}
	switch r.Method {
	case http.MethodGet:
		conf := globals.RssUrls
		resp := adminConfigResponse{
			Values:         conf.Values,
			Keywords:       conf.Keywords,
			ReFresh:        conf.ReFresh,
			AutoUpdatePush: conf.AutoUpdatePush,
			NightStartTime: conf.NightStartTime,
			NightEndTime:   conf.NightEndTime,
			Archives:       conf.Archives,
			HasAdminToken:  resolveAdminToken() != "",
		}
		resp.Notify.Feishu.API = conf.Notify.Feishu.API
		resp.Notify.Telegram.API = conf.Notify.Telegram.API
		resp.Notify.Telegram.ChatID = conf.Notify.Telegram.ChatID
		resp.Notify.Telegram.Token = conf.Notify.Telegram.Token
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(resp)
	case http.MethodPut:
		body, err := io.ReadAll(r.Body)
		if err != nil {
			http.Error(w, "invalid request body", http.StatusBadRequest)
			return
		}
		defer r.Body.Close()
		var conf models.Config
		if err = json.Unmarshal(body, &conf); err != nil {
			http.Error(w, "invalid json", http.StatusBadRequest)
			return
		}
		// 默认保留当前 token，避免前端编辑时意外覆盖
		if conf.AdminToken == "" {
			conf.AdminToken = globals.RssUrls.AdminToken
		}
		if conf.Keywords == nil {
			conf.Keywords = globals.RssUrls.Keywords
		}
		if conf.Notify.Telegram.API == "" {
			conf.Notify.Telegram.API = globals.RssUrls.Notify.Telegram.API
		}
		if conf.Notify.Telegram.ChatID == "" {
			conf.Notify.Telegram.ChatID = globals.RssUrls.Notify.Telegram.ChatID
		}
		if conf.Notify.Telegram.Token == "" {
			conf.Notify.Telegram.Token = globals.RssUrls.Notify.Telegram.Token
		}
		if conf.Notify.Feishu.API == "" {
			conf.Notify.Feishu.API = globals.RssUrls.Notify.Feishu.API
		}
		if strings.TrimSpace(conf.Archives) == "" {
			conf.Archives = globals.RssUrls.Archives
		}
		if err = conf.Validate(); err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		if err = models.SaveConf(conf); err != nil {
			http.Error(w, "failed to save config", http.StatusInternalServerError)
			return
		}
		globals.Init()
		formattedTime := time.Now().Format("2006-01-02 15:04:05")
		for _, url := range globals.RssUrls.Values {
			go utils.UpdateFeed(url, formattedTime)
		}
		w.WriteHeader(http.StatusNoContent)
	default:
		w.WriteHeader(http.StatusMethodNotAllowed)
	}
}

func adminTestSourceHandler(w http.ResponseWriter, r *http.Request) {
	if !adminAuthorized(r) {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}
	if r.Method != http.MethodPost {
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}
	body, err := io.ReadAll(r.Body)
	if err != nil {
		http.Error(w, "invalid request body", http.StatusBadRequest)
		return
	}
	defer r.Body.Close()
	var req adminTestSourceRequest
	if err = json.Unmarshal(body, &req); err != nil {
		http.Error(w, "invalid json", http.StatusBadRequest)
		return
	}
	req.URL = strings.TrimSpace(req.URL)
	if req.URL == "" {
		http.Error(w, "url cannot be empty", http.StatusBadRequest)
		return
	}
	if !strings.HasPrefix(req.URL, "http://") && !strings.HasPrefix(req.URL, "https://") {
		http.Error(w, "url must start with http:// or https://", http.StatusBadRequest)
		return
	}
	feed, err := globals.Fp.ParseURL(req.URL)
	if err != nil {
		http.Error(w, "failed to fetch source: "+err.Error(), http.StatusBadRequest)
		return
	}
	resp := adminTestSourceResponse{
		Title: feed.Title,
		Items: len(feed.Items),
	}
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(resp)
}

func adminAuthorized(r *http.Request) bool {
	adminToken := resolveAdminToken()
	if adminToken == "" {
		return false
	}
	authHeader := r.Header.Get("Authorization")
	if strings.HasPrefix(authHeader, "Bearer ") {
		token := strings.TrimSpace(strings.TrimPrefix(authHeader, "Bearer "))
		if token == adminToken {
			return true
		}
	}
	return r.Header.Get("X-Admin-Token") == adminToken
}

func resolveAdminToken() string {
	if token := strings.TrimSpace(os.Getenv("ADMIN_TOKEN")); token != "" {
		return token
	}
	return strings.TrimSpace(globals.RssUrls.AdminToken)
}

func serveHome(w http.ResponseWriter, r *http.Request) {
	w.Header().Add("Content-Type", "text/html; charset=utf-8")
	w.Write(globals.HtmlContent)
}

func tplHandler(w http.ResponseWriter, r *http.Request) {
	// 创建一个新的模板，并设置自定义分隔符为<< >>，避免与Vue的语法冲突
	tmplInstance := template.New("index.html").Delims("<<", ">>")
	//添加加法函数计数
	funcMap := template.FuncMap{
		"inc": func(i int) int {
			return i + 1
		},
	}
	// 加载模板文件
	tmpl, err := parseIndexTemplate(tmplInstance.Funcs(funcMap))
	if err != nil {
		log.Println("模板加载错误:", err)
		return
	}

	//判断现在是否是夜间
	formattedTime := time.Now().Format("15:04:05")
	darkMode := false
	if globals.RssUrls.NightStartTime != "" && globals.RssUrls.NightEndTime != "" {
		if globals.RssUrls.NightStartTime > formattedTime || formattedTime > globals.RssUrls.NightEndTime {
			darkMode = true
		}
	}

	// 定义一个数据对象
	data := struct {
		Keywords       string
		ConfigKeywords []string
		RssDataList    []models.Feed
		DarkMode       bool
		AutoUpdatePush int
	}{
		Keywords:       getKeywords(),
		ConfigKeywords: globals.RssUrls.Keywords,
		RssDataList:    utils.GetFeeds(),
		DarkMode:       darkMode,
		AutoUpdatePush: globals.RssUrls.AutoUpdatePush,
	}

	// 渲染模板并将结果写入响应
	err = tmpl.Execute(w, data)
	if err != nil {
		log.Println("模板渲染错误:", err)
	}
}

func wsHandler(w http.ResponseWriter, r *http.Request) {
	conn, err := globals.Upgrader.Upgrade(w, r, nil)
	if err != nil {
		log.Printf("Upgrade failed: %v", err)
		return
	}

	defer conn.Close()
	for {
		for _, url := range globals.RssUrls.Values {
			globals.Lock.RLock()
			cache, ok := globals.DbMap[url]
			globals.Lock.RUnlock()
			if !ok {
				log.Printf("Error getting feed from db is null %v", url)
				continue
			}
			data, err := json.Marshal(cache)
			if err != nil {
				log.Printf("json marshal failure: %s", err.Error())
				continue
			}

			err = conn.WriteMessage(websocket.TextMessage, data)
			//错误直接关闭更新
			if err != nil {
				log.Printf("Error sending message or Connection closed: %v", err)
				return
			}
		}
		//如果未配置则不自动更新
		if globals.RssUrls.AutoUpdatePush == 0 {
			return
		}
		time.Sleep(time.Duration(globals.RssUrls.AutoUpdatePush) * time.Minute)
	}
}

//获取关键词也就是title
//获取feeds列表
func getKeywords() string {
	words := ""
	for _, url := range globals.RssUrls.Values {
		globals.Lock.RLock()
		cache, ok := globals.DbMap[url]
		globals.Lock.RUnlock()
		if !ok {
			log.Printf("Error getting feed from db is null %v", url)
			continue
		}
		if cache.Title != "" {
			words += cache.Title + ","
		}
	}
	return words
}

func getFeedsHandler(w http.ResponseWriter, r *http.Request) {
	feeds := utils.GetFeeds()

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(feeds)
}

func parseIndexTemplate(t *template.Template) (*template.Template, error) {
	devStaticDir := strings.TrimSpace(os.Getenv("DEV_STATIC_DIR"))
	if devStaticDir != "" {
		indexPath := filepath.Join(devStaticDir, "index.html")
		return t.ParseFiles(indexPath)
	}
	return t.ParseFS(globals.DirStatic, "static/index.html")
}

func readStaticFile(fileName string) ([]byte, error) {
	devStaticDir := strings.TrimSpace(os.Getenv("DEV_STATIC_DIR"))
	if devStaticDir != "" {
		target := filepath.Join(devStaticDir, fileName)
		return os.ReadFile(target)
	}
	return globals.DirStatic.ReadFile("static/" + fileName)
}
