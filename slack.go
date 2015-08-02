package main

import (
	"encoding/json"
	"fmt"
	"github.com/gorilla/websocket"
	"io/ioutil"
	"net/http"
	"net/url"
	"strings"
)

type User struct {
	ID      string `json:"id"`
	Name    string `json:"name"`
	Profile struct {
		FirstName string `json:"first_name"`
		LastName  string `json:"last_name"`
		RealName  string `json:"real_name"`
		Email     string `json:"email"`
		Skype     string `json:"skype"`
		Phone     string `json:"phone"`
	} `json:"profile"`
}

type Channel struct {
	ID      string   `json:"id"`
	Name    string   `json:"name"`
	Members []string `json:"members"`
}

type helloMessage struct {
	Ok      bool   `json:"ok"`
	Url     string `json:"url"`
	BotInfo struct {
		Id   string `json:"id"`
		Name string `json:"name"`
	} `json:"self"`
	Users    []User    `json:"users"`
	Channels []Channel `json:"channels"`
	Error    string    `json:"error"`
}

type SlackBot struct {
	token         string
	info          *helloMessage
	wsConn        *websocket.Conn
	Messages      chan SlackMessage
	stop          chan bool
	Quit          chan bool
	userIdName    map[string]string
	channelIdName map[string]string
}

const (
	slackUrl = "https://slack.com/api/rtm.start?token="
)

func NewSlackBot(token string) (sb *SlackBot, err error) {
	sb = new(SlackBot)
	sb.token = token
	resp, err := http.Get(slackUrl + sb.token)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}
	sb.info = new(helloMessage)
	err = json.Unmarshal(body, sb.info)
	if err != nil {
		return nil, err
	}
	sb.wsConn, _, err = websocket.DefaultDialer.Dial(sb.info.Url, nil)
	if err != nil {
		return nil, err
	}
	sb.channelIdName = make(map[string]string)
	for _, channel := range sb.info.Channels {
		sb.channelIdName[channel.ID] = channel.Name
	}
	sb.userIdName = make(map[string]string)
	for _, user := range sb.info.Users {
		sb.userIdName[user.ID] = user.Name
	}
	sb.Messages = make(chan SlackMessage, 100)
	sb.stop = make(chan bool)
	sb.Quit = make(chan bool)
	go sb.listenWS()
	return sb, nil
}

func (sb *SlackBot) IsBot(id string) bool {
	return (id == sb.info.BotInfo.Id) || (id == "")
}

func (sb *SlackBot) UserIdToName(id string) (name string) {
	return sb.userIdName[id]
}

func (sb *SlackBot) ChannelIdToName(id string) (name string) {
	return sb.channelIdName[id]
}

func (sb *SlackBot) Request(method string, mapArgs ...interface{}) (response map[string]interface{}, err error) {
	response = make(map[string]interface{})
	q := url.Values{}
	for i := 0; i < len(mapArgs); i += 2 {
		name, ok := mapArgs[i].(string)
		if !ok {
			err = fmt.Errorf("arg name is not string: %#v", mapArgs[i])
			return
		}
		value := mapArgs[i+1].(string)
		if !ok {
			err = fmt.Errorf("arg '%s' value is not string: %#v", name, mapArgs[i+1])
			return
		}
		q.Set(name, value)
	}
	q.Set("token", sb.token)
	methodUrl := fmt.Sprintf("https://slack.com/api/%s?%s", method, q.Encode())
	resp, err := http.Get(methodUrl)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}
	err = json.Unmarshal(body, &response)
	if err != nil {
		return nil, err
	}
	return
}

func (sb *SlackBot) ChannelList() (list []map[string]interface{}, err error) {
	var response map[string]interface{}
	var ok bool
	response, err = sb.Request("channels.list", "exclude_archived", "1")
	if err != nil {
		return
	}
	listRaw, ok := response["channels"].([]interface{})
	if !ok {
		err = fmt.Errorf("bad response: %v", response)
	}
	for _, listItemRaw := range listRaw {
		listItem, ok := listItemRaw.(map[string]interface{})
		if !ok {
			err = fmt.Errorf("bad channel data: %#v", listItemRaw)
			return
		}
		list = append(list, listItem)
	}
	return
}

type SlackEventBase struct {
	Type string `json:"type"`
}

type SlackEventError struct {
	Error struct {
		Code int    `json:"code"`
		Msg  string `json:msg"`
	} `json:"error"`
}

type SlackMessage struct {
	SlackEventBase
	SlackEventError
	Channel string `json:"channel"`
	User    string `json:"user"`
	Text    string `json:"text"`
	sb      *SlackBot
}

func (sm SlackMessage) LowerText() string {
	return strings.ToLower(sm.Text)
}

func (sm SlackMessage) UserName() string {
	return sm.sb.UserIdToName(sm.User)
}

func (sm SlackMessage) ChannelName() string {
	return sm.sb.ChannelIdToName(sm.Channel)
}

func (sb *SlackBot) listenWS() {
	for {
		select {
		case <-sb.stop:
			sb.Quit <- true
			return
		default:
			msgType, msg, err := sb.wsConn.ReadMessage()
			if err == nil {
				if msgType == websocket.TextMessage {
					var slackMsg SlackMessage
					err := json.Unmarshal(msg, &slackMsg)
					if err == nil {
						slackMsg.sb = sb
						sb.Messages <- slackMsg
					}
				}
			}
		}
	}
}

func (sb *SlackBot) Stop() {
	sb.stop <- true
}

func (sb *SlackBot) SendMessage(channel, text string) (err error) {
	_, err = http.PostForm("https://slack.com/api/chat.postMessage", map[string][]string{
		"token":    []string{sb.token},
		"channel":  []string{channel},
		"username": []string{sb.info.BotInfo.Name},
		"text":     []string{text},
	})
	if err != nil {
		return err
	}
	return nil
}
