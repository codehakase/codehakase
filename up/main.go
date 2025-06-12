package main

import (
	"context"
	"encoding/xml"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"text/template"
	"time"

	"golang.org/x/sync/errgroup"
)

const baseURL = "https://codehakase.com"

var locale = mustLoadLocale("Africa/Lagos")

type Feed struct {
	XMLName xml.Name `xml:"rss"`
	Channel struct {
		Items []*Item `xml:"item"`
	} `xml:"channel"`
}

type Item struct {
	Title       string     `xml:"title"`
	Link        string     `xml:"link"`
	PublishedAt customTime `xml:"pubDate"`
	Summary     string     `xml:"description"`
}

type ReadmeInfo struct {
	Posts  []*Item
	Shorts []*Item
}

type customTime struct {
	time.Time
}

func (ct *customTime) UnmarshalXML(da *xml.Decoder, st xml.StartElement) error {
	var s string
	err := da.DecodeElement(&s, &st)
	if err != nil {
		return err
	}

	t, err := time.Parse(time.RFC1123Z, s)
	if err != nil {
		return err
	}

	*ct = customTime{t}
	return nil
}

func fetchRSSFeed(ctx context.Context, url string) (items []*Item, err error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return
	}

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		err = fmt.Errorf("expected status 200 for %s, got %d", url, resp.StatusCode)
		return
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return
	}

	var feed Feed
	if err := xml.Unmarshal(body, &feed); err != nil {
		return nil, fmt.Errorf("failed to parse feed data: %+v", err)
	}

	items = feed.Channel.Items
	return
}

func mustLoadLocale(localeName string) *time.Location {
	location, err := time.LoadLocation(localeName)
	if err != nil {
		log.Fatalf("failed to load local %s: %v", localeName, err)
	}

	return location
}

func formatTime(t customTime) string {
	return t.In(locale).Format("January 2, 2006")
}

func renderTemplate(data *ReadmeInfo) error {
	tmpl := template.Must(
		template.New("readme_tmpl").Funcs(template.FuncMap{
			"FormatTime": formatTime,
		}).ParseFiles("README.md.tmpl"))

	if err := tmpl.ExecuteTemplate(os.Stdout, "README.md.tmpl", data); err != nil {
		return err
	}

	return nil
}

func main() {
	ctx := context.Background()
	var readmeInfo ReadmeInfo

	errGroup, ctx := errgroup.WithContext(ctx)
	errGroup.SetLimit(5)

	errGroup.Go(func() error {
		var err error
		readmeInfo.Posts, err = fetchRSSFeed(ctx, fmt.Sprintf("%s/blog/index.xml", baseURL))
		return err
	})

	errGroup.Go(func() error {
		var err error
		readmeInfo.Shorts, err = fetchRSSFeed(ctx, fmt.Sprintf("%s/shorts/index.xml", baseURL))
		return err
	})

	if err := errGroup.Wait(); err != nil {
		log.Fatalf("failed to fetch feed data: %+v\n", err)
	}

	if err := renderTemplate(&readmeInfo); err != nil {
		log.Fatalf("failed to render readme, errr: %+v", err)
	}
}
