package main

import (
	"bufio"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/ioutil"
	"log"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"

	"github.com/fatih/color"
	"github.com/garyburd/go-oauth/oauth"
)

type Tweet struct {
	Identifier string `json:"id_str"`
	User       struct {
		ScreenName string `json:"screen_name"`
	} `json:"user"`
}

func getConfigDir() (string, error) {
	dir := os.Getenv("HOME")
	if dir == "" && runtime.GOOS == "windows" {
		dir = os.Getenv("APPDATA")
		if dir == "" {
			dir = filepath.Join(os.Getenv("USERPROFILE"), "Application Data", "koirand-tweet")
		}
		dir = filepath.Join(dir, "koirand-tweet")
	} else {
		dir = filepath.Join(dir, ".config", "koirand-tweet")
	}
	if err := os.MkdirAll(dir, 0700); err != nil {
		return "", err
	}
	return dir, nil
}

func getConfig() (string, map[string]string, error) {
	dir, err := getConfigDir()
	if err != nil {
		return "", nil, err
	}

	var file string
	file = filepath.Join(dir, "settings.json")
	config := map[string]string{}

	b, err := ioutil.ReadFile(file)
	if err != nil && !os.IsNotExist(err) {
		return "", nil, err
	}
	if err != nil {
		config["ClientToken"] = "AIPf0Bq576tUsyZnPTnVb0th6"
		config["ClientSecret"] = "d78H90qH7SO1qJgeNPA1SRjfnyHrffrVLwb7NyMsEzFwjJImCe"
	} else {
		err = json.Unmarshal(b, &config)
		if err != nil {
			return "", nil, fmt.Errorf("could not unmarshal %v: %v", file, err)
		}
	}
	return file, config, nil
}

var oauthClient = oauth.Client{
	TemporaryCredentialRequestURI: "https://api.twitter.com/oauth/request_token",
	ResourceOwnerAuthorizationURI: "https://api.twitter.com/oauth/authenticate",
	TokenRequestURI:               "https://api.twitter.com/oauth/access_token",
}

func clientAuth(requestToken *oauth.Credentials) (*oauth.Credentials, error) {
	var err error
	browser := "xdg-open"
	url := oauthClient.AuthorizationURL(requestToken, nil)

	args := []string{url}
	if runtime.GOOS == "windows" {
		browser = "rundll32.exe"
		args = []string{"url.dll,FileProtocolHandler", url}
	} else if runtime.GOOS == "darwin" {
		browser = "open"
		args = []string{url}
	} else if runtime.GOOS == "plan9" {
		browser = "plumb"
	}
	color.Set(color.FgHiRed)
	fmt.Println("Open this URL and enter PIN.")
	color.Set(color.Reset)
	fmt.Println(url)
	browser, err = exec.LookPath(browser)
	if err == nil {
		cmd := exec.Command(browser, args...)
		cmd.Stderr = os.Stderr
		err = cmd.Start()
		if err != nil {
			return nil, fmt.Errorf("cannot start command: %v", err)
		}
	}

	fmt.Print("PIN: ")
	stdin := bufio.NewScanner(os.Stdin)
	if !stdin.Scan() {
		return nil, fmt.Errorf("canceled")
	}
	accessToken, _, err := oauthClient.RequestToken(http.DefaultClient, requestToken, stdin.Text())
	if err != nil {
		return nil, fmt.Errorf("cannot request token: %v", err)
	}
	return accessToken, nil
}

func getAccessToken(config map[string]string) (*oauth.Credentials, bool, error) {
	oauthClient.Credentials.Token = config["ClientToken"]
	oauthClient.Credentials.Secret = config["ClientSecret"]

	authorized := false
	var token *oauth.Credentials
	accessToken, foundToken := config["AccessToken"]
	accessSecret, foundSecret := config["AccessSecret"]
	if foundToken && foundSecret {
		token = &oauth.Credentials{Token: accessToken, Secret: accessSecret}
	} else {
		requestToken, err := oauthClient.RequestTemporaryCredentials(http.DefaultClient, "", nil)
		if err != nil {
			err = fmt.Errorf("cannot request temporary credentials: %v", err)
			return nil, false, err
		}
		token, err = clientAuth(requestToken)
		if err != nil {
			err = fmt.Errorf("cannot request temporary credentials: %v", err)
			return nil, false, err
		}

		config["AccessToken"] = token.Token
		config["AccessSecret"] = token.Secret
		authorized = true
	}
	return token, authorized, nil
}

func readFile(filename string) ([]byte, error) {
	if filename == "-" {
		return ioutil.ReadAll(os.Stdin)
	}
	return ioutil.ReadFile(filename)
}

func rawCall(token *oauth.Credentials, method string, uri string, status string, res interface{}) error {
	debug := false
	param := make(url.Values)
	param.Set("status", status)
	oauthClient.SignParam(token, method, uri, param)
	var resp *http.Response
	var err error
	resp, err = http.PostForm(uri, url.Values(param))
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 400 {
		return errors.New(resp.Status)
	}
	if res == nil {
		return nil
	}
	if debug {
		return json.NewDecoder(io.TeeReader(resp.Body, os.Stdout)).Decode(&res)
	}
	return json.NewDecoder(resp.Body).Decode(&res)
}

func openEditor(file string) error {
	editor := os.Getenv("EDITOR")
	if editor == "" {
		editor = "nano"
	}
	cmd := exec.Command(editor, file)
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd.Run()
}

func postTweet(token *oauth.Credentials) error {
	const statusFileName = "TWEET_STATUS"
	dir, err := getConfigDir()
	if err != nil {
		return err
	}
	statusFilePath := filepath.Join(dir, statusFileName)
	f, err := os.Create(statusFilePath)
	if err != nil {
		return err
	}
	f.Close()

	if err := openEditor(statusFilePath); err != nil {
		return err
	}

	status, err := readFile(statusFilePath)
	if err != nil {
		return err
	} else if len(status) == 0 {
		return errors.New("Tweet status is empty")
	}

	var tweet Tweet
	err = rawCall(token, http.MethodPost, "https://api.twitter.com/1.1/statuses/update.json", string(status), &tweet)
	if err != nil {
		return err
	}
	fmt.Printf("Tweeted: https://twitter.com/%s/status/%s\n", tweet.User.ScreenName, tweet.Identifier)

	return nil
}

func main() {
	file, config, err := getConfig()
	if err != nil {
		log.Fatalf("Cannot get configuration: %v", err)
	}

	token, authorized, err := getAccessToken(config)
	if err != nil {
		log.Fatalf("Cannot get access token: %v", err)
	}
	if authorized {
		b, err := json.MarshalIndent(config, "", "  ")
		if err != nil {
			log.Fatalf("Cannot store file: %v", err)
		}
		err = ioutil.WriteFile(file, b, 0700)
		if err != nil {
			log.Fatalf("Cannot store file: %v", err)
		}
	}
	if err := postTweet(token); err != nil {
		log.Fatalf("Cannot post tweet: %v", err)
	}
}
