package models

import (
	"encoding/json"
	"errors"
	"os"
)

func ParseConf() (Config, error) {
	var conf Config
	data, err := os.ReadFile("config.json")
	if err != nil {
		return conf, err
	}
	// 解析JSON数据到Config结构体
	err = json.Unmarshal(data, &conf)

	return conf, err
}

type Config struct {
	Values         []string `json:"values"`
	Keywords       []string `json:"keywords,omitempty"`
	ReFresh        int      `json:"refresh"`
	AutoUpdatePush int      `json:"autoUpdatePush"`
	NightStartTime string   `json:"nightStartTime"`
	NightEndTime   string   `json:"nightEndTime"`
	AdminToken     string   `json:"adminToken,omitempty"`
	Notify         Notify   `json:"notify,omitempty"`
	Archives       string   `json:"archives,omitempty"`
}

type Notify struct {
	Feishu   FeishuNotify   `json:"feishu,omitempty"`
	Telegram TelegramNotify `json:"telegram,omitempty"`
}

type FeishuNotify struct {
	API string `json:"api,omitempty"`
}

type TelegramNotify struct {
	API    string `json:"api,omitempty"`
	ChatID string `json:"chat_id,omitempty"`
	Token  string `json:"token,omitempty"`
}

func (c Config) Validate() error {
	if len(c.Values) == 0 {
		return errors.New("values cannot be empty")
	}
	if c.ReFresh <= 0 {
		return errors.New("refresh must be greater than 0")
	}
	if c.AutoUpdatePush < 0 {
		return errors.New("autoUpdatePush must be greater than or equal to 0")
	}
	return nil
}

func SaveConf(conf Config) error {
	data, err := json.MarshalIndent(conf, "", "    ")
	if err != nil {
		return err
	}
	data = append(data, '\n')
	return os.WriteFile("config.json", data, 0o644)
}

func (older Config) GetIncrement(newer Config) []string {
	var (
		urlMap    = make(map[string]struct{})
		increment = make([]string, 0, len(newer.Values))
	)
	for _, item := range older.Values {
		urlMap[item] = struct{}{}
	}

	for _, item := range newer.Values {
		if _, ok := urlMap[item]; ok {
			continue
		}
		increment = append(increment, item)
	}

	return increment
}
