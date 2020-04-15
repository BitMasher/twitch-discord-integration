package Scheduler

import (
	"cloud.google.com/go/firestore"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io/ioutil"
	"net/http"
	"net/url"
	"os"
	"strings"
)

type TwitchTokens struct {
	AccessToken  string   `json:"access_token"`
	RefreshToken string   `json:"refresh_token"`
	ExpiresIn    int      `json:"expires_in"`
	Scope        []string `json:"scope"`
	TokenType    string   `json:"token_type"`
}

func GetClientToken() TwitchTokens {
	oauthUrl, _ := url.Parse("https://id.twitch.tv/oauth2/token")
	query, _ := url.ParseQuery(oauthUrl.RawQuery)
	query.Add("client_id", os.Getenv("clientid"))
	query.Add("client_secret", os.Getenv("clientsecret"))
	query.Add("grant_type", "client_credentials")
	oauthUrl.RawQuery = query.Encode()
	resp, err := http.Post(oauthUrl.String(), "application/json", strings.NewReader(""))
	if err != nil {
		panic(err)
	}

	if resp.StatusCode >= 300 {
		fmt.Println(ioutil.ReadAll(resp.Body))
		fmt.Printf("%+v\n", resp)
		panic(errors.New("failed to get token"))
	}

	var d TwitchTokens
	if err := json.NewDecoder(resp.Body).Decode(&d); err != nil {
		panic(err)
	}

	return d
}

type TwitchUser struct {
	BroadcasterType string `json:"broadcaster_type"`
	Description     string `json:"description"`
	DisplayName     string `json:"display_name"`
	Email           string `json:"email"`
	Id              string `json:"id"`
	Login           string `json:"login"`
	OfflineImageUrl string `json:"offline_image_url"`
	ProfileImageUrl string `json:"profile_image_url"`
	UserType        string `json:"type"`
	ViewCount       int    `json:"view_count"`
}

type RootConfig struct {
	Watchlist []string `firestore:"watchlist"`
}

func SubscribeWebhooks(w http.ResponseWriter, r *http.Request) {

	ctx := context.Background()
	client, err := firestore.NewClient(ctx, "bitmasher-dev")
	if err != nil {
		panic(err)
	}
	defer client.Close()

	configCol := client.Collection("Configs")
	rootSnap, err := configCol.Doc("root").Get(ctx)
	if err != nil {
		panic(err)
	}

	if !rootSnap.Exists() {
		fmt.Println("no configs exist")
		return
	}
	var rootConfig RootConfig
	err = rootSnap.DataTo(&rootConfig)
	if err != nil {
		panic(err)
	}

	tokens := GetClientToken()
	for i := range rootConfig.Watchlist {
		req, err := http.NewRequest("GET", fmt.Sprintf("https://api.twitch.tv/helix/users?login=%s", rootConfig.Watchlist[i]), nil)
		if err != nil {
			panic(err)
		}
		req.Header.Add("Authorization", fmt.Sprintf("Bearer %s", tokens.AccessToken))
		req.Header.Add("Client-ID", os.Getenv("clientid"))
		resp, err := http.DefaultClient.Do(req)
		if err != nil {
			panic(err)
		}

		var userDets TwitchUser
		if err = json.NewDecoder(resp.Body).Decode(&userDets); err != nil {
			panic(err)
		}

		str := fmt.Sprintf("{\"hub.callback\": \"https://us-central1-bitmasher-dev.cloudfunctions.net/twitch-webhook?userid=%s\",\"hub.mode\": \"subscribe\",\"hub.topic\":\"https://api.twitch.tv/helix/streams?user_id=%s\",\"hub.lease_seconds\": \"864000\",\"hub.secret\": \"%s\"}", rootConfig.Watchlist[i], os.Getenv("clientsecret"))
		req, err = http.NewRequest("POST", "https://api.twitch.tv/helix/webhooks/hub", strings.NewReader(str))
		if err != nil {
			panic(err)
		}
		req.Header.Add("Authorization", fmt.Sprintf("Bearer %s", tokens.AccessToken))
		req.Header.Add("Client-ID", os.Getenv("clientid"))
		resp, err = http.DefaultClient.Do(req)
		if err != nil {
			panic(err)
		}
		if resp.StatusCode >= 300 {
			fmt.Println(ioutil.ReadAll(resp.Body))
			panic(errors.New("failed to create webhook"))
		}
	}
}
