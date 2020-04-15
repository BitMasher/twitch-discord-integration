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

func GetClientToken() (*TwitchTokens, error) {
	oauthUrl, _ := url.Parse("https://id.twitch.tv/oauth2/token")
	query, _ := url.ParseQuery(oauthUrl.RawQuery)
	query.Add("client_id", os.Getenv("clientid"))
	query.Add("client_secret", os.Getenv("clientsecret"))
	query.Add("grant_type", "client_credentials")
	oauthUrl.RawQuery = query.Encode()
	resp, err := http.Post(oauthUrl.String(), "application/json", strings.NewReader(""))
	if err != nil {
		return nil, err
	}

	if resp.StatusCode >= 300 {
		fmt.Println(ioutil.ReadAll(resp.Body))
		fmt.Printf("%+v\n", resp)
		return nil, errors.New("failed to get token")
	}

	var d TwitchTokens
	if err := json.NewDecoder(resp.Body).Decode(&d); err != nil {
		return nil, err
	}

	return &d, nil
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

type PubSubMessage struct {
	Data []byte `json:"data"`
}

func SubscribeWebhooks(ctx context.Context, m PubSubMessage) error {
	client, err := firestore.NewClient(ctx, "bitmasher-dev")
	if err != nil {
		return err
	}
	defer client.Close()

	configCol := client.Collection("Configs")
	rootSnap, err := configCol.Doc("root").Get(ctx)
	if err != nil {
		return err
	}

	if !rootSnap.Exists() {
		fmt.Println("no configs exist")
		return nil
	}
	var rootConfig RootConfig
	err = rootSnap.DataTo(&rootConfig)
	if err != nil {
		return err
	}

	tokens, err := GetClientToken()
	if err != nil {
		return err
	}
	for i := range rootConfig.Watchlist {
		req, err := http.NewRequest("GET", fmt.Sprintf("https://api.twitch.tv/helix/users?login=%s", rootConfig.Watchlist[i]), nil)
		if err != nil {
			return err
		}
		req.Header.Add("Authorization", fmt.Sprintf("Bearer %s", tokens.AccessToken))
		req.Header.Add("Client-ID", os.Getenv("clientid"))
		resp, err := http.DefaultClient.Do(req)
		if err != nil {
			return err
		}

		if resp.StatusCode >= 300 {
			fmt.Println(ioutil.ReadAll(resp.Body))
			return errors.New("failed to load user")
		}

		var userDets TwitchUser
		if err = json.NewDecoder(resp.Body).Decode(&userDets); err != nil {
			return err
		}

		str := fmt.Sprintf("{\"hub.callback\": \"https://us-central1-bitmasher-dev.cloudfunctions.net/twitch-webhook?userid=%s\",\"hub.mode\": \"subscribe\",\"hub.topic\":\"https://api.twitch.tv/helix/streams?user_id=%s\",\"hub.lease_seconds\": \"864000\",\"hub.secret\": \"%s\"}", userDets.Id, os.Getenv("clientsecret"))
		req, err = http.NewRequest("POST", "https://api.twitch.tv/helix/webhooks/hub", strings.NewReader(str))
		if err != nil {
			return err
		}
		req.Header.Add("Authorization", fmt.Sprintf("Bearer %s", tokens.AccessToken))
		req.Header.Add("Client-ID", os.Getenv("clientid"))
		req.Header.Add("Content-Type", "application/json")
		resp, err = http.DefaultClient.Do(req)
		if err != nil {
			return err
		}
		if resp.StatusCode >= 300 {
			fmt.Println(ioutil.ReadAll(resp.Body))
			return errors.New("failed to create webhook")
		}
	}
	return nil
}
