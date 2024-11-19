package main

import (
	"context"
	"crypto/rand"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"log"
	"net"
	"net/http"
	"net/url"
	"os"
	"strings"
	"time"

	"golang.org/x/oauth2"
	"golang.org/x/oauth2/google"
	"tractor.dev/integra"
	"tractor.dev/toolkit-go/engine/cli"
)

func authCmd() *cli.Command {
	cmd := &cli.Command{
		Usage: "auth <service>",
		Short: "authenticate with a service",
		Args:  cli.MinArgs(1),
		Run: func(ctx *cli.Context, args []string) {
			// hard coding for now since this is just a
			// debug helper for now
			switch args[0] {
			case "spotify":
				authSpotify()
			case "google-calendar":
				authGoogleCalendar()
			default:
				log.Fatal("TODO")
			}

		},
	}
	return cmd
}

func authSpotify() {
	clientID, clientSecret := integra.ServiceClientCredentials("spotify")
	listener, err := net.Listen("tcp4", ":4532")
	if err != nil {
		log.Fatal(err)
	}
	state := randomState()
	redirectURI := fmt.Sprintf("http://%s/auth/callback", strings.ReplaceAll(listener.Addr().String(), "0.0.0.0", "localhost"))
	srv := &http.Server{
		Handler: http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Header().Add("content-type", "text/html")
			if r.URL.Query().Get("error") != "" {
				fmt.Fprintf(w, "<p>Error: %s</p>\n", r.URL.Query().Get("error"))
			} else if r.URL.Query().Get("state") != state {
				fmt.Fprintf(w, "<p>Error: bad state</p>\n")
			} else {
				data := url.Values{}
				data.Set("code", r.URL.Query().Get("code"))
				data.Set("redirect_uri", redirectURI)
				data.Set("grant_type", "authorization_code")

				req, err := http.NewRequest("POST", "https://accounts.spotify.com/api/token", strings.NewReader(data.Encode()))
				if err != nil {
					log.Fatal(err)
				}

				req.Header.Set("Authorization", fmt.Sprintf("Basic %s", base64.StdEncoding.EncodeToString([]byte(clientID+":"+clientSecret))))
				req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

				client := &http.Client{}
				resp, err := client.Do(req)
				if err != nil {
					log.Fatal(err)
				}
				defer resp.Body.Close()

				var reply map[string]any
				dec := json.NewDecoder(resp.Body)
				if err := dec.Decode(&reply); err != nil {
					log.Fatal(err)
				}

				expiry := time.Duration(int(reply["expires_in"].(float64))) * time.Second
				fmt.Printf("Access Token: %s (expires in %s)", reply["access_token"], expiry)
				fmt.Fprintln(w, "<p>Authenticated. You can close this window.</p>")
			}
			go func() {
				<-time.After(1 * time.Second)
				listener.Close()
			}()
		}),
	}
	done := make(chan bool)
	go func() {
		srv.Serve(listener)
		done <- true
	}()

	query := url.Values{}
	query.Set("client_id", clientID)
	query.Set("response_type", "code")
	query.Set("state", state)
	query.Set("scope", strings.Join(spotifyScopes, " "))
	query.Set("show_dialog", "false")
	query.Set("redirect_uri", redirectURI)
	fullURL := fmt.Sprintf("https://accounts.spotify.com/authorize?%s", strings.ReplaceAll(query.Encode(), "+", "%20"))
	open(fullURL)
	<-done
}

func randomState() string {
	bytes := make([]byte, 16)
	if _, err := rand.Read(bytes); err != nil {
		panic(err)
	}
	return hex.EncodeToString(bytes)
}

var spotifyScopes = []string{
	"user-read-private",
	"playlist-read-collaborative",
	"playlist-modify-public",
	"playlist-modify-private",
	"streaming",
	"ugc-image-upload",
	"user-follow-modify",
	"user-follow-read",
	"user-library-read",
	"user-library-modify",
	"user-read-private",
	"user-read-email",
	"user-top-read",
	"user-read-playback-state",
	"user-modify-playback-state",
	"user-read-currently-playing",
	"user-read-recently-played",
}

func authGoogleCalendar() {
	credentialsJSON := os.Getenv("GOOGLE_CLIENT_JSON")
	if credentialsJSON == "" {
		log.Fatal("need GOOGLE_CLIENT_JSON environment variable set")
	}

	scope := "https://www.googleapis.com/auth/calendar"
	config, err := google.ConfigFromJSON([]byte(credentialsJSON), scope)
	if err != nil {
		log.Fatalf("Unable to parse client secret file to config: %v", err)
	}

	listener, err := net.Listen("tcp4", ":4532")
	if err != nil {
		log.Fatal(err)
	}

	srv := &http.Server{
		Handler: http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Header().Add("content-type", "text/html")
			if r.URL.Query().Get("code") == "" {
				fmt.Fprintf(w, "<p>Error: %s</p>\n", r.URL.Query().Get("error"))
			} else {

				token, err := config.Exchange(context.Background(), r.URL.Query().Get("code"))
				if err != nil {
					log.Fatalf("Unable to retrieve token from web: %v", err)
				}

				fmt.Printf("Access Token: %s (expires in %s)", token.AccessToken, time.Until(token.Expiry))
				fmt.Fprintln(w, "<p>Authenticated. You can close this window.</p>")
			}
			go func() {
				<-time.After(1 * time.Second)
				listener.Close()
			}()
		}),
	}
	done := make(chan bool)
	go func() {
		srv.Serve(listener)
		done <- true
	}()

	open(config.AuthCodeURL(randomState(), oauth2.AccessTypeOffline))
	<-done
}
