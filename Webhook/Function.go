// Package p contains an HTTP Cloud Function.
package p

import (
	"bytes"
	"cloud.google.com/go/firestore"
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io/ioutil"
	"net/http"
	"os"
	"strings"
)

type TwitchStreamInfo struct {
	GameId       string   `json:"game_id"`
	Id           string   `json:"id"`
	Language     string   `json:"language"`
	Pagination   string   `json:"pagination"`
	StartedAt    string   `json:"started_at"`
	TagIds       []string `json:"tag_ids"`
	ThumbnailUrl string   `json:"thumbnail_url"`
	Title        string   `json:"title"`
	StreamType   string   `json:"type"`
	UserId       string   `json:"user_id"`
	UserName     string   `json:"user_name"`
	ViewerCount  int      `json:"viewer_count"`
}

type TwitchPayload struct {
	Data []TwitchStreamInfo
}

type DiscordMessage struct {
	Content string `json:"content"`
}

func PostDiscordMessage(channelId string, message string) {

	fmt.Printf("Posting message %s to channel %s\n", message, channelId)

	msg := DiscordMessage{Content: message}

	jsonMsgBytes, err := json.Marshal(msg)
	if err != nil {
		panic(err)
	}

	req, err := http.NewRequest("POST", fmt.Sprintf("https://discord.com/api/v6/channels/%s/messages", channelId), bytes.NewReader(jsonMsgBytes))
	if err != nil {
		panic(err)
	}
	req.Header.Add("Content-Type", "application/json")
	req.Header.Add("Authorization", fmt.Sprintf("Bot %s", os.Getenv("discordtoken")))
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		panic(err)
	}

	if resp.StatusCode >= 300 {
		body, err := ioutil.ReadAll(resp.Body)
		if err != nil {
			panic(err)
		}
		fmt.Printf("Got bad status %d with response %s\n", resp.StatusCode, body)
	}
}

func TwitchWebhook(w http.ResponseWriter, r *http.Request) {
	if len(r.URL.Query().Get("hub.challenge")) > 0 {
		fmt.Fprint(w, r.URL.Query().Get("hub.challenge"))
		return
	}
	body, err := ioutil.ReadAll(r.Body)
	if err != nil {
		panic(err)
	}
	fmt.Println(body)
	signature := r.Header.Get("X-Hub-Signature")
	if len(signature) > 0 {
		sigParts := strings.Split(signature, "=")
		if len(sigParts) > 1 {
			signature = sigParts[1]
		}
	}
	mac := hmac.New(sha256.New, []byte(os.Getenv("clientsecret")))
	mac.Write(body)
	resMac := mac.Sum(nil)
	if !strings.EqualFold(signature, hex.EncodeToString(resMac)) {
		fmt.Print(errors.New(fmt.Sprintf("invalid signature, message rejected\n%s - %s", signature, hex.EncodeToString(resMac))))
		return
	}

	client, err := firestore.NewClient(context.Background(), "bitmasher-dev")
	if err != nil {
		panic(err)
	}
	defer client.Close()

	configCol := client.Collection("Configs")
	userSnap, err := configCol.Doc("usermap").Get(context.Background())
	if err != nil {
		panic(err)
	}

	if !userSnap.Exists() {
		fmt.Println("no configs exist")
		return
	}
	var userMap map[string][]string
	err = userSnap.DataTo(&userMap)
	if err != nil {
		panic(err)
	}

	userId := r.URL.Query().Get("userid")

	if _, ok := userMap[userId]; !ok {
		fmt.Printf("Got twitch notification for unmapped user %s\n", userId)
		return
	}

	var d TwitchPayload
	if err := json.NewDecoder(strings.NewReader(string(body))).Decode(&d); err != nil {
		panic(err)
	}
	if len(d.Data) > 0 {
		for i := range d.Data {
			fmt.Printf("Stream for %s is %s\n", d.Data[i].UserName, d.Data[i].StreamType)
			fmt.Fprintf(w, "Stream for %s is %s\n", d.Data[i].UserName, d.Data[i].StreamType)

			for i := range userMap[userId] {

				PostDiscordMessage(userMap[userId][i], fmt.Sprintf("Hey everyone %s is live!\nCurrently streaming %s\nhttps://twitch.tv/%s", d.Data[i].UserName, d.Data[i].Title, d.Data[i].UserName))
			}
		}
	} else {
		fmt.Fprintf(w, "Stream for %s is offline\n", userId)
		http.Post(os.Getenv("discorduri"), "application/json", strings.NewReader(fmt.Sprintf("{\"content\": \"Aww %s has finished streaming, you just missed em\"}", userId)))
	}
}
