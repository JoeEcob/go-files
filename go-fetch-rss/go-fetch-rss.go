package main

import (
	"encoding/xml"
	"errors"
	"flag"
	"fmt"
	"io"
	"net/http"
	"os"
	"path"
	"time"
)

type Response struct {
	Ch Channel `xml:"channel"`
}

type Channel struct {
	Title string  `xml:"title"`
	Items []*Item `xml:"item"`
}

type Item struct {
	Title       string `xml:"title"`
	Guid        string `xml:"guid"`
	PublishDate string `xml:"pubDate"`
	Link        string `xml:"link"`
}

const dateFormat = "2006-01-02"

func main() {
	url := flag.String("url", "", "The URL to call to fetch RSS data including API key and search query.")
	outputDir := flag.String("out", ".", "Path to output directory.")
	fileExtension := flag.String("ext", "file", "File extension name to use.")
	redirectFileExtension := flag.String("redir-ext", "redirect", "Redirect file extension name to use.")
	targetDate := flag.String("date", time.Now().Format(dateFormat), "Date to find results from e.g. '2006-01-02'.")
	dryRun := flag.Bool("dry-run", true, "Flag to set dry-run mode.")
	verbose := flag.Bool("verbose", false, "Flag to set dry-run mode.")

	flag.Parse()

	if *url == "" {
		fmt.Println("Error, URL is required.")
		return
	}

	fmt.Printf("go-fetch-rss DryRun: %t Date: %s OutputDir: %s FileExtension: %s URL: %s\n", *dryRun, *targetDate, *outputDir, *fileExtension, *url)

	res, err := http.Get(*url)
	if res.StatusCode != 200 {
		fmt.Printf("Error fetching! %s", err)
		return
	}

	resBody, _ := io.ReadAll(res.Body)

	var r Response
	xml.Unmarshal(resBody, &r)

	fmt.Printf("Found %d items, starting download...\n", len(r.Ch.Items))

	// Create a custom client to catch redirects. Without this we get an "error supported protocol".
	client := http.Client{
		CheckRedirect: func(req *http.Request, via []*http.Request) error {
			loc, _ := req.Response.Location()

			// If the scheme matches our wanted redir file ext, return an error to stop the follow.
			if loc != nil && loc.Scheme == *redirectFileExtension {
				return errors.New("caught redirect")
			}

			// Otherwise return nil, to follow the redirect
			return nil
		},
	}

	for _, item := range r.Ch.Items {
		// e.g. "Thu, 11 Jan 2024 21:00:00 +0000"
		t, e := time.Parse("Mon, 2 Jan 2006 15:04:05 +0000", item.PublishDate)
		if e != nil {
			fmt.Printf("Err parsing time: %s %s\n", item.Title, e)
			continue
		}

		if *targetDate != t.Format(dateFormat) {
			if *verbose {
				fmt.Printf("Skipping, date mismatch: %s %s\n", item.Title, t.Format(dateFormat))
			}
			continue
		}

		if *dryRun {
			fmt.Printf("Skipping download, dry run enabled %s\n%s\n", item.Title, item.Link)
			continue
		}

		fmt.Printf("Doing %s\n", item.Title)

		itemRes, err := client.Get(item.Link)

		// Handle redirects by saving the URL to a file
		if err != nil && itemRes != nil && itemRes.StatusCode == http.StatusFound {
			loc, _ := itemRes.Location()
			fmt.Printf("Got 302. Writing %s\n", loc)
			os.WriteFile(path.Join(*outputDir, fmt.Sprintf("%s.%s", item.Title, *redirectFileExtension)), []byte(loc.String()), 0666)
			continue
		}

		// Every other error is unknown so exit
		if err != nil {
			fmt.Printf("Error fetching: %s err: %s\n", item.Title, err)
			continue
		}

		// Otherwise fetch the actual file
		if itemRes.StatusCode == http.StatusOK {
			fmt.Printf("Writing %s\n", item.Title)
			bytes, _ := io.ReadAll(itemRes.Body)
			os.WriteFile(path.Join(*outputDir, fmt.Sprintf("%s.%s", item.Title, *fileExtension)), bytes, 0666)
		}

		fmt.Printf("Done %s\n", item.Title)
	}

	fmt.Println("Done all!")
}
