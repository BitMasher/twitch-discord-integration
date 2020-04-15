package Scheduler

import (
	"encoding/json"
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

func SubscribeWebhooks(w http.ResponseWriter, r *http.Request) {
	oauthUrl, _ := url.Parse("https://id.twitch.tv/oauth2/token")
	oauthUrl.Query().Add("client_id", os.Getenv("clientid"))
	oauthUrl.Query().Add("client_secret", os.Getenv("clientsecret"))
	oauthUrl.Query().Add("grant_type", "client_credentials")
	resp, err := http.Post(oauthUrl.String(), "application/json", strings.NewReader(""))
	if err != nil {
		panic(err)
	}

	if resp.StatusCode >= 300 {
		fmt.Println(ioutil.ReadAll(resp.Body))
		fmt.Printf("%+v\n", resp)
	}

	var d TwitchTokens

	if err := json.NewDecoder(resp.Body).Decode(&d); err != nil {
		panic(err)
	}

	fmt.Printf("%+v", d)
}
