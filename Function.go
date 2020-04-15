// Package p contains an HTTP Cloud Function.
package p

import (
	"encoding/json"
	"fmt"
	"net/http"
)

type twitchStreamInfo struct {
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

type twitchPayload struct {
	Data []twitchStreamInfo
}

// HelloWorld prints the JSON encoded "message" field in the body
// of the request or "Hello, World!" if there isn't one.
func TwitchWebhook(w http.ResponseWriter, r *http.Request) {
	userId := r.URL.Query().Get("userid")
	var d twitchPayload
	if err := json.NewDecoder(r.Body).Decode(&d); err != nil {
		if len(d.Data) > 0 {
			for i := range d.Data {
				fmt.Printf("Stream for %s is %s\n", d.Data[i].UserName, d.Data[i].StreamType)
				fmt.Fprintf(w, "Stream for %s is %s\n", d.Data[i].UserName, d.Data[i].StreamType)
			}
		} else {
			fmt.Fprintf(w, "Stream for %s is offline\n", userId)
		}
		return
	} else {
		fmt.Println(err)
		fmt.Fprintln(w, err)
		fmt.Println(r.Body)
		fmt.Fprintln(w, r.Body)
	}
}
