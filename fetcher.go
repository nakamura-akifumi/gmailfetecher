package main

import (
	"context"
	"database/sql"
	"encoding/base64"
	"encoding/json"
	"flag"
	"fmt"
	_ "github.com/mattn/go-sqlite3"
	"golang.org/x/oauth2"
	"golang.org/x/oauth2/google"
	"google.golang.org/api/gmail/v1"
	"google.golang.org/api/option"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"time"
)

type Config struct {
	Query    string `json:"query"`
	Store    string `json:"store"`
	Database string `json:"database"`
}

type GmailAdapter struct {
	srv       *gmail.Service
	user      string
	appconfig Config
	db        *sql.DB
}

func (g *GmailAdapter) SearchMailAndFetchAttachFile(query string) error {
	var output string

	res, err := g.srv.Users.Messages.List(g.user).Q(query).Do()
	if err != nil {
		return err
	}
	log.Printf("response message size:%d\n", len(res.Messages))
	for _, e := range res.Messages {
		log.Printf("MessageID:%s\n", e.Id)

		sqlStmt := "select count(*) from messages where messageid = ?"
		err = g.db.QueryRow(sqlStmt, e.Id).Scan(&output)
		if err != nil {
			log.Fatal(err)
		}
		if output == "0" {
			msgresponse, _ := g.srv.Users.Messages.Get(g.user, e.Id).Format("full").Do()
			if len(msgresponse.Payload.Parts) > 0 {
				for _, part := range msgresponse.Payload.Parts {
					g.pluckFile(e.Id, part)
				}
			}
			query := `INSERT INTO messages (messageid) VALUES (?)`
			result, err := g.db.Exec(query, e.Id)
			if err != nil {
				log.Printf("error occured: insert data into table\n")
				log.Println(err)
				return err
			}
			// 挿入されたレコード数を取得
			count, err := result.RowsAffected()
			if err != nil {
				log.Println(err)
				return err
			}
			log.Printf("%d rows inserted\n", count)
		} else {
			log.Println("skip")
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
		dec, err := base64.URLEncoding.DecodeString(encodeData)
		if err != nil {
			log.Println(encodeData)
			log.Fatal(err)
		}

		s := strings.Replace(part.Filename, "\\", "_", -1)
		s = strings.Replace(s, "/", "_", -1)
		filename := g.buildFilename(messageId, s)
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
		fmt.Printf("Writefile: %s\n", filename)
	}
	if len(part.Parts) > 0 {
		for _, part := range part.Parts {
			g.pluckFile(messageId, part)
		}
	}
}

func (g *GmailAdapter) buildFilename(messageId string, partfilename string) string {
	fName := filepath.Base(partfilename)
	extName := filepath.Ext(partfilename)
	bName := fName[:len(fName)-len(extName)]
	filename := filepath.Join(g.appconfig.Store, partfilename)
	if FileExists(filename) {
		for i := 2; ; i++ {
			filename = filepath.Join(g.appconfig.Store, bName+"_"+strconv.Itoa(i)+extName)
			if FileExists(filename) == false {
				break
			}
		}
	}
	return filename
}

func NewGmailClient(ctx context.Context, apc Config, db *sql.DB) (*GmailAdapter, error) {
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

	return &GmailAdapter{srv: srv, user: "me", appconfig: apc, db: db}, nil
}

func FileExists(filename string) bool {
	_, err := os.Stat(filename)
	return err == nil
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
			log.Println(err)
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
	log.Printf("Go to the following link in your browser then type the "+
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
			log.Println(err)
		}
	}(f)
	tok := &oauth2.Token{}
	err = json.NewDecoder(f).Decode(tok)
	return tok, err
}

// Saves a token to a file path.
func saveToken(path string, token *oauth2.Token) {
	log.Printf("Saving credential file to: %s\n", path)
	f, err := os.OpenFile(path, os.O_RDWR|os.O_CREATE|os.O_TRUNC, 0600)
	if err != nil {
		log.Fatalf("Unable to cache oauth token: %v", err)
	}
	defer func(f *os.File) {
		err := f.Close()
		if err != nil {
			log.Println(err)
		}
	}(f)
	err = json.NewEncoder(f).Encode(token)
	if err != nil {
		return
	}
}

func prepareDatabase(apc *Config) (*sql.DB, error) {
	db, err := sql.Open("sqlite3", apc.Database)
	if err != nil {
		log.Fatal(err)
	}
	//defer db.Close()

	var output string
	sqlStmt := "SELECT COUNT(*) FROM sqlite_master WHERE TYPE='table' AND name='messages'"
	err = db.QueryRow(sqlStmt).Scan(&output)
	if err != nil {
		log.Fatal(err)
	}
	//fmt.Println(output)
	if output != "1" {
		sqlStmt = "create table messages (messageid varchar not null primary key, messagedatetime datetime)"
		_, err = db.Exec(sqlStmt)
		if err != nil {
			log.Printf("%q: %s\n", err, sqlStmt)
			return nil, err
		}
	}

	return db, nil
}

func main() {

	var (
		afterdayfilterstr = flag.String("after", "", "afterday filter (2d 3w 4m 5y)")
	)
	flag.Parse()

	apc, err := loadConfig()
	if err != nil {
		log.Fatal(err)
		return
	}

	db, err := prepareDatabase(apc)
	if err != nil {
		log.Fatal(err)
		return
	}

	ctx := context.Background()
	g, err := NewGmailClient(ctx, *apc, db)
	if err != nil {
		log.Fatal(err)
		return
	}

	query := apc.Query
	if *afterdayfilterstr != "" {
		r := regexp.MustCompile(`(\d)([dwmy])`)
		rs := r.FindAllStringSubmatch(*afterdayfilterstr, -1)
		t := time.Now()

		if len(rs) == 0 {
			fmt.Println("paramter -after format error")
			return
		}

		if len(rs) > 0 && len(rs[0]) == 3 {
			yadd := 0
			madd := 0
			dadd := 0

			switch rs[0][2] {
			case "y":
				yadd, _ = strconv.Atoi(rs[0][1])
				yadd = yadd * -1
			case "m":
				madd, _ = strconv.Atoi(rs[0][1])
				madd = madd * -1
			case "w":
				dadd, _ = strconv.Atoi(rs[0][1])
				dadd = dadd * -7
			case "d":
				dadd, _ = strconv.Atoi(rs[0][1])
				dadd = dadd * -1
			}

			t_add := t.AddDate(yadd, madd, dadd)
			query = query + " after:" + t_add.Format("2006/1/2")
		}
	}
	log.Printf("S:%s Q:%s q:%s\n", apc.Store, apc.Query, query)
	err = g.SearchMailAndFetchAttachFile(query)
	if err != nil {
		log.Println(err)
	}

	g.db.Close()
}
