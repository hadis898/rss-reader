package utils

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"rss-reader/globals"
	"rss-reader/models"
	"strings"
	"sync"
	"time"

	"github.com/fsnotify/fsnotify"
	"github.com/mmcdole/gofeed"
)

var (
	archiveInitOnce sync.Once
	archiveLock     sync.Mutex
	sentArchiveMap  = map[string]struct{}{}
)

func UpdateFeeds() {
	var (
		tick = time.Tick(time.Duration(globals.RssUrls.ReFresh) * time.Minute)
	)
	for {
		formattedTime := time.Now().Format("2006-01-02 15:04:05")
		for _, url := range globals.RssUrls.Values {
			go UpdateFeed(url, formattedTime)
		}
		<-tick
	}
}

func UpdateFeed(url, formattedTime string) {
	result, err := globals.Fp.ParseURL(url)
	if err != nil {
		log.Printf("Error fetching feed: %v | %v", url, err)
		return
	}
	//feed内容无更新时无需更新缓存
	if cache, ok := globals.DbMap[url]; ok &&
		len(result.Items) > 0 &&
		len(cache.Items) > 0 &&
		result.Items[0].Link == cache.Items[0].Link {
		return
	}

	notifyNewItems(url, result.Title, result.Items)

	customFeed := models.Feed{
		Title:  result.Title,
		Link:   url,
		Custom: map[string]string{"lastupdate": formattedTime},
		Items:  make([]models.Item, 0, len(result.Items)),
	}
	for _, v := range result.Items {
		customFeed.Items = append(customFeed.Items, models.Item{
			Link:        v.Link,
			Title:       v.Title,
			Description: v.Description,
		})
	}
	globals.Lock.Lock()
	defer globals.Lock.Unlock()
	globals.DbMap[url] = customFeed
}

func notifyNewItems(url, feedTitle string, items []*gofeed.Item) {
	conf := globals.RssUrls
	if len(conf.Keywords) == 0 {
		return
	}
	if conf.Notify.Telegram.ChatID == "" || (conf.Notify.Telegram.API == "" && conf.Notify.Telegram.Token == "") {
		return
	}

	// 仅对“本轮新增”的条目通知，避免重复轰炸
	existed := map[string]struct{}{}
	globals.Lock.RLock()
	if cache, ok := globals.DbMap[url]; ok {
		for _, item := range cache.Items {
			if item.Link != "" {
				existed[item.Link] = struct{}{}
			}
		}
	}
	globals.Lock.RUnlock()

	for _, item := range items {
		if item == nil || item.Link == "" {
			continue
		}
		if _, ok := existed[item.Link]; ok {
			continue
		}
		if !matchKeywords(item.Title, conf.Keywords) {
			continue
		}
		if alreadyNotified(item.Link, conf.Archives) {
			continue
		}
		if err := sendTelegram(feedTitle, item.Title, item.Link); err != nil {
			log.Printf("telegram notify failed: %v", err)
			continue
		}
		markNotified(item.Link, conf.Archives)
	}
}

func matchKeywords(title string, keywords []string) bool {
	text := strings.ToLower(strings.TrimSpace(title))
	if text == "" {
		return false
	}
	for _, k := range keywords {
		kw := strings.ToLower(strings.TrimSpace(k))
		if kw == "" {
			continue
		}
		if strings.Contains(text, kw) {
			return true
		}
	}
	return false
}

func sendTelegram(feedTitle, itemTitle, link string) error {
	tg := globals.RssUrls.Notify.Telegram
	apiURL := strings.TrimSpace(tg.API)
	if strings.Contains(apiURL, "${token}") {
		apiURL = strings.ReplaceAll(apiURL, "${token}", strings.TrimSpace(tg.Token))
	}
	if apiURL == "" && strings.TrimSpace(tg.Token) != "" {
		apiURL = "https://api.telegram.org/bot" + strings.TrimSpace(tg.Token) + "/sendMessage"
	}
	if apiURL == "" {
		return nil
	}
	payload := map[string]string{
		"chat_id": tg.ChatID,
		"text":    "【" + feedTitle + "】\n" + itemTitle + "\n" + link,
	}
	body, err := json.Marshal(payload)
	if err != nil {
		return err
	}
	resp, err := http.Post(apiURL, "application/json", bytes.NewBuffer(body))
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		respBody, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("telegram status=%d body=%s", resp.StatusCode, strings.TrimSpace(string(respBody)))
	}
	return nil
}

func alreadyNotified(link, archiveFile string) bool {
	archiveInitOnce.Do(func() {
		loadArchive(archiveFile)
	})
	archiveLock.Lock()
	defer archiveLock.Unlock()
	_, ok := sentArchiveMap[link]
	return ok
}

func markNotified(link, archiveFile string) {
	archiveLock.Lock()
	defer archiveLock.Unlock()
	sentArchiveMap[link] = struct{}{}
	if strings.TrimSpace(archiveFile) == "" {
		return
	}
	f, err := os.OpenFile(archiveFile, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o644)
	if err != nil {
		log.Printf("open archive file failed: %v", err)
		return
	}
	defer f.Close()
	_, _ = f.WriteString(link + "\n")
}

func loadArchive(archiveFile string) {
	if strings.TrimSpace(archiveFile) == "" {
		return
	}
	data, err := os.ReadFile(archiveFile)
	if err != nil {
		return
	}
	lines := strings.Split(string(data), "\n")
	archiveLock.Lock()
	defer archiveLock.Unlock()
	for _, line := range lines {
		item := strings.TrimSpace(line)
		if item != "" {
			sentArchiveMap[item] = struct{}{}
		}
	}
}

//获取feeds列表
func GetFeeds() []models.Feed {
	feeds := make([]models.Feed, 0, len(globals.RssUrls.Values))
	for _, url := range globals.RssUrls.Values {
		globals.Lock.RLock()
		cache, ok := globals.DbMap[url]
		globals.Lock.RUnlock()
		if !ok {
			log.Printf("Error getting feed from db is null %v", url)
			continue
		}

		feeds = append(feeds, cache)
	}
	return feeds
}

func WatchConfigFileChanges(filePath string) {
	// 创建一个新的监控器
	watcher, err := fsnotify.NewWatcher()
	if err != nil {
		log.Fatal(err)
	}
	defer watcher.Close()

	// 添加要监控的文件
	err = watcher.Add(filePath)
	if err != nil {
		log.Fatal(err)
	}

	// 启动一个 goroutine 来处理文件变化事件
	go func() {
		for {
			time.Sleep(7 * time.Second)
			select {
			case event, ok := <-watcher.Events:
				if !ok {
					log.Println("通道关闭1")
					return
				}
				if event.Op&fsnotify.Write == fsnotify.Write {
					log.Println("文件已修改")
					globals.Init()
					formattedTime := time.Now().Format("2006-01-02 15:04:05")
					for _, url := range globals.RssUrls.Values {
						go UpdateFeed(url, formattedTime)
					}
				}
			case err, ok := <-watcher.Errors:
				if !ok {
					log.Println("通道关闭2")
					return
				}
				log.Println("错误:", err)
				return
			}
		}
	}()

	select {}
}
