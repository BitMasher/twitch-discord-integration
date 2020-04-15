// Package p contains an HTTP Cloud Function.
package p

import (
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
	GameId       string `json:"game_id"`
	Id           string `json:"id"`
	Language     string `json:"language"`
	Pagination   string `json:"pagination"`
	StartedAt    string `json:"started_at"`
	TagIds       string `json:"tag_ids"`
	ThumbnailUrl string `json:"thumbnail_url"`
	Title        string `json:"title"`
	StreamType   string `json:"type"`
	UserId       string `json:"user_id"`
	UserName     string `json:"user_name"`
	ViewerCount  int    `json:"viewer_count"`
}

type TwitchPayload struct {
	Data []TwitchStreamInfo
}

func TwitchWebhook(w http.ResponseWriter, r *http.Request) {
	signature := r.Header.Get("X-Hub-Signature")
	if len(signature) > 0 {
		sigParts := strings.Split(signature, "=")
		if len(sigParts) > 1 {
			signature = sigParts[1]
		}
	}
	mac := hmac.New(sha256.New, []byte(os.Getenv("clientsecret")))
	body, err := ioutil.ReadAll(r.Body)
	if err != nil {
		panic(err)
	}
	mac.Write(body)
	resMac := mac.Sum(nil)
	if !strings.EqualFold(signature, hex.EncodeToString(resMac)) {
		fmt.Print(errors.New(fmt.Sprintf("invalid signature, message rejected\n%s - %s", signature, hex.EncodeToString(resMac))))
		return
	}

	userId := r.URL.Query().Get("userid")
	var d TwitchPayload
	if err := json.NewDecoder(r.Body).Decode(&d); err != nil {
		panic(err)
	}
	if len(d.Data) > 0 {
		for i := range d.Data {
			fmt.Printf("Stream for %s is %s\n", d.Data[i].UserName, d.Data[i].StreamType)
			fmt.Fprintf(w, "Stream for %s is %s\n", d.Data[i].UserName, d.Data[i].StreamType)
			http.Post(os.Getenv("discorduri"), "application/json", strings.NewReader(fmt.Sprintf("{\"content\": \"Hey everyone %s is live!\nCurrently streaming %s\nhttps://twitch.tv/%s\"}", d.Data[i].UserName, d.Data[i].Title, d.Data[i].UserName)))
		}
	} else {
		fmt.Fprintf(w, "Stream for %s is offline\n", userId)
		http.Post(os.Getenv("discorduri"), "application/json", strings.NewReader(fmt.Sprintf("{\"content\": \"Aww %s has finished streaming, you just missed em\"}", userId)))
	}
}
