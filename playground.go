package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"io/ioutil"
	"log"
	"net/http"
	"regexp"
	"strings"
)

var (
	token = flag.String("t", "", "slack api bot token")
)

type compileResult struct {
	Errors string `json:"compile_errors"`
	Output string `json:"output"`
}

func panicOnErr(err interface{}) {
	if err != nil {
		panic(err)
	}
}

func playground(uri string) (res string) {
	resp, err := http.Get(uri + ".go")
	if err != nil {
		return
	}
	defer resp.Body.Close()
	bytes, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return
	}
	res = string(bytes)
	resp, err = http.PostForm("https://play.golang.org/compile", map[string][]string{
		"body": []string{res},
	})
	panicOnErr(err)
	defer resp.Body.Close()
	bytes, err = ioutil.ReadAll(resp.Body)
	if err != nil {
		return
	}
	compileRes := new(compileResult)
	err = json.Unmarshal(bytes, compileRes)
	if err != nil {
		return
	}
	if len([]rune(res)) > 1024 {
		src := []rune(res)[:1024]
		res = string(src) + "\n..."
	}
	res = "Source:\n```" + res + "```"
	if compileRes.Errors != "" {
		res += "\n Errors:\n```" + compileRes.Errors + "```"
	}
	if compileRes.Output != "" {
		res += "\nResult:```"
		res += compileRes.Output
		res += "```"
	}
	return res
}

func wiki(uri string) string {
	langRex := regexp.MustCompilePOSIX(`[a-z]+\.wiki`)
	lang := langRex.FindString(uri)
	lang = lang[:len(lang)-5]
	articleRex := regexp.MustCompilePOSIX(`.org\/wiki\/[a-zA-Z0-9а-яА-Я_\(\)\%\,\:]+`)
	article := articleRex.FindString(uri)
	article = article[10:]
	fmt.Println(article)
	wikiUrl := "https://" + lang + ".wikipedia.org/w/api.php?format=json&action=query&prop=extracts&exintro=&explaintext=&titles=" + article
	resp, err := http.Get(wikiUrl)
	if err != nil {
		return ""
	}
	defer resp.Body.Close()
	jsraw, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return ""
	}
	wikiSearch := new(struct {
		Query struct {
			Pages map[string]map[string]interface{} `json:"pages"`
		} `json:"query"`
	})
	err = json.Unmarshal(jsraw, wikiSearch)
	fmt.Println(string(jsraw))
	for _, page := range wikiSearch.Query.Pages {
		var msg string
		if title, ok := page["title"].(string); ok {
			msg += "*" + title + "*\n"
			if extract, ok := page["extract"].(string); ok {
				msg += "```" + extract + "```"
			}
		}
		return msg
	}
	return ""
}

func main() {
	flag.Parse()
	log.Printf("connecting to Slack API...")
	bot, err := NewSlackBot(*token)
	panicOnErr(err)
	for {
		log.Printf("awaiting message")
		select {
		case <-bot.Quit:
			log.Printf("quit")
			return
		case msg := <-bot.Messages:
			data, _ := json.Marshal(msg)
			log.Printf("new message %s", string(data))
			if msg.Type == "message" {
				if !bot.IsBot(msg.User) {
					fmt.Println("Author:", bot.UserIdToName(msg.User))
					fmt.Println("Channel:", bot.ChannelIdToName(msg.Channel))
					fmt.Println("Text:", msg.Text)
					playgroundUrl := regexp.MustCompile(`https?\:\/\/play\.golang\.org\/p\/[^>^/]+`)
					wikiUrl := regexp.MustCompilePOSIX(`https?\:\/\/[a-z]*\.wikipedia\.org\/wiki\/([a-zA-Z0-9%_\(\)а-яА-я\,\:]+)`)
					switch {
					case playgroundUrl.MatchString(msg.LowerText()):
						uris := playgroundUrl.FindAllString(msg.Text, -1)
						for _, uri := range uris {
							log.Printf("playground url: %s", uri)
							bot.SendMessage(msg.Channel, playground(uri))
						}
					case wikiUrl.MatchString(msg.LowerText()):
						uris := wikiUrl.FindAllString(msg.LowerText(), -1)
						for _, uri := range uris {
							bot.SendMessage(msg.Channel, wiki(uri))
						}
					case strings.Contains(msg.LowerText(), "pong"):
						bot.SendMessage(msg.Channel, "@"+bot.UserIdToName(msg.User)+": ping")
					}
				}
			}
		}
	}
}
