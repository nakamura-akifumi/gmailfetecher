package main

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"golang.org/x/oauth2"
	"golang.org/x/oauth2/google"
	"google.golang.org/api/gmail/v1"
	"google.golang.org/api/option"
	"log"
	"net/http"
	"os"
	"path/filepath"
)

type Config struct {
	Query string `json:"query"`
	Store string `json:"store"`
}

type GmailAdapter struct {
	srv       *gmail.Service
	user      string
	appconfig Config
}

func (g *GmailAdapter) SearchMailAndFetchAttachFile(query string) error {
	res, err := g.srv.Users.Messages.List(g.user).Q(query).Do()
	if err != nil {
		return err
	}
	fmt.Printf("response message size:%d\n", len(res.Messages))
	for _, e := range res.Messages {
		fmt.Printf("MessageID:%s\n", e.Id)
		msgresponse, _ := g.srv.Users.Messages.Get(g.user, e.Id).Format("full").Do()
		if len(msgresponse.Payload.Parts) > 0 {
			for i, part := range msgresponse.Payload.Parts {
				fmt.Println(i)
				g.pluckFile(e.Id, part)
			}
		}
	}
	return nil
}

func (g *GmailAdapter) pluckFile(messageId string, part *gmail.MessagePart) {
	if part.Filename != "" {
		encodeData := ""
		log.Printf("findfile mimetype:%s filename:%s attachmentId:%s\n", part.MimeType, part.Filename, part.Body.AttachmentId)
		if part.Body.AttachmentId == "" {
			encodeData = part.Body.Data
		} else {
			attachPart, err := g.srv.Users.Messages.Attachments.Get(g.user, messageId, part.Body.AttachmentId).Do()
			if err != nil {
				log.Fatal(err)
				panic(err)
			}
			if attachPart == nil {
				panic(err)
			}
			encodeData = attachPart.Data
		}
		//dec, err := base64.StdEncoding.DecodeString(encodeData)
		dec, err := base64.URLEncoding.DecodeString(encodeData)
		if err != nil {
			log.Println(encodeData)
			log.Fatal(err)
		}

		filename := filepath.Join(g.appconfig.Store, part.Filename)
		f, err := os.Create(filename)
		if err != nil {
			panic(err)
		}
		defer func(f *os.File) {
			err := f.Close()
			if err != nil {
				fmt.Println(err)
			}
		}(f)

		if _, err := f.Write(dec); err != nil {
			panic(err)
		}
		if err := f.Sync(); err != nil {
			panic(err)
		}
		fmt.Printf("Writefile: %s", filename)
	}
	if len(part.Parts) > 0 {
		for _, part := range part.Parts {
			g.pluckFile(messageId, part)
		}
	}
}

func NewGmailClient(ctx context.Context, apc Config) (*GmailAdapter, error) {
	b, err := os.ReadFile("credentials.json")
	if err != nil {
		log.Fatalf("Unable to read client secret file: %v", err)
	}

	config, err := google.ConfigFromJSON(b, gmail.GmailReadonlyScope)
	if err != nil {
		log.Fatalf("Unable to parse client secret file to config: %v", err)
		return nil, err
	}
	client := getClient(config)

	srv, err := gmail.NewService(ctx, option.WithHTTPClient(client))
	if err != nil {
		log.Fatalf("Unable to retrieve Gmail client: %v", err)
		return nil, err
	}

	return &GmailAdapter{srv: srv, user: "me", appconfig: apc}, nil
}

func loadConfig() (*Config, error) {
	f, err := os.Open("config.json")
	if err != nil {
		log.Fatal("loadConfig os.Open err:", err)
		return nil, err
	}
	defer func(f *os.File) {
		err := f.Close()
		if err != nil {
			fmt.Println(err)
		}
	}(f)

	var cfg Config
	err = json.NewDecoder(f).Decode(&cfg)
	return &cfg, err
}

// Retrieve a token, saves the token, then returns the generated client.
func getClient(config *oauth2.Config) *http.Client {
	// The file token.json stores the user's access and refresh tokens, and is
	// created automatically when the authorization flow completes for the first
	// time.
	tokFile := "token.json"
	tok, err := tokenFromFile(tokFile)
	if err != nil {
		tok = getTokenFromWeb(config)
		saveToken(tokFile, tok)
	}
	return config.Client(context.Background(), tok)
}

// Request a token from the web, then returns the retrieved token.
func getTokenFromWeb(config *oauth2.Config) *oauth2.Token {
	authURL := config.AuthCodeURL("state-token", oauth2.AccessTypeOffline)
	fmt.Printf("Go to the following link in your browser then type the "+
		"authorization code: \n%v\n", authURL)

	var authCode string
	if _, err := fmt.Scan(&authCode); err != nil {
		log.Fatalf("Unable to read authorization code: %v", err)
	}

	tok, err := config.Exchange(context.TODO(), authCode)
	if err != nil {
		log.Fatalf("Unable to retrieve token from web: %v", err)
	}
	return tok
}

// Retrieves a token from a local file.
func tokenFromFile(file string) (*oauth2.Token, error) {
	f, err := os.Open(file)
	if err != nil {
		return nil, err
	}
	defer func(f *os.File) {
		err := f.Close()
		if err != nil {
			fmt.Println(err)
		}
	}(f)
	tok := &oauth2.Token{}
	err = json.NewDecoder(f).Decode(tok)
	return tok, err
}

// Saves a token to a file path.
func saveToken(path string, token *oauth2.Token) {
	fmt.Printf("Saving credential file to: %s\n", path)
	f, err := os.OpenFile(path, os.O_RDWR|os.O_CREATE|os.O_TRUNC, 0600)
	if err != nil {
		log.Fatalf("Unable to cache oauth token: %v", err)
	}
	defer func(f *os.File) {
		err := f.Close()
		if err != nil {
			fmt.Println(err)
		}
	}(f)
	err = json.NewEncoder(f).Encode(token)
	if err != nil {
		return
	}
}

func main() {
	apc, err := loadConfig()
	if err != nil {
		log.Fatal(err)
		return
	}

	ctx := context.Background()
	g, err := NewGmailClient(ctx, *apc)
	if err != nil {
		log.Fatal(err)
		return
	}

	fmt.Printf("S:%s Q:%s\n", apc.Store, apc.Query)
	err = g.SearchMailAndFetchAttachFile(apc.Query)
	if err != nil {
		log.Fatal(err)
	}

}
