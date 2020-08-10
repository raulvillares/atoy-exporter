package main

import (
	"encoding/json"
	"fmt"
	"log"
	"os"
	"strings"

	"github.com/gocolly/colly/v2"
	"github.com/urfave/cli/v2"
)

type album struct {
	ID          string
	Artist      string
	AlbumTitle  string
	ReleaseYear string
	Format      string
	Label       string
	Genres      string
	Tags        []string
}

func newAlbum() *album {
	var a album
	a.Tags = make([]string, 0)
	return &a
}

func (a album) containsTag(tag string) bool {
	for _, item := range a.Tags {
		if item == tag {
			return true
		}
	}
	return false
}

func visitAlbum(albumLink string) (*album, bool) {
	visitedAlbum := newAlbum()
	albumID := albumLink[30+7 : strings.Index(albumLink, "-")]
	visitedAlbum.ID = albumID

	albumCollector := colly.NewCollector()

	// ToDo This does not work, review Colly docs
	albumAlreadyVisited, _ := albumCollector.HasVisited(albumLink)
	if albumAlreadyVisited {
		return visitedAlbum, false
	}

	albumCollector.OnRequest(func(r *colly.Request) {
		fmt.Println("Visiting album", r.URL.String())
	})

	albumCollector.OnHTML("div.albumHeadline div.artist", func(artistElement *colly.HTMLElement) {
		visitedAlbum.Artist = artistElement.Text
	})

	albumCollector.OnHTML("div.albumHeadline div.albumTitle", func(albumTitleElement *colly.HTMLElement) {
		visitedAlbum.AlbumTitle = albumTitleElement.Text
	})

	albumCollector.OnHTML("div.albumTopBox.info", func(albumInfoElement *colly.HTMLElement) {
		albumInfoElement.ForEach("div.detailRow", func(_ int, detailElement *colly.HTMLElement) {
			detailText := detailElement.Text
			if strings.HasSuffix(detailText, "Release Date") {
				releaseDate := detailText[:strings.LastIndex(detailText, "Release Date")-4]
				releaseYear := releaseDate[len(releaseDate)-4:]
				visitedAlbum.ReleaseYear = releaseYear
			}
			if strings.HasSuffix(detailText, "Format") {
				format := detailText[:strings.LastIndex(detailText, "Format")-4]
				visitedAlbum.Format = format
			}

			if strings.HasSuffix(detailText, "Label") {
				label := detailText[:strings.LastIndex(detailText, "Label")-4]
				visitedAlbum.Label = label
			}

			if strings.HasSuffix(detailText, "Genres") {
				genres := detailText[:strings.LastIndex(detailText, "Genres")-4]
				visitedAlbum.Genres = genres
			}
		})

		albumInfoElement.ForEach("div.tag.strong", func(_ int, tagElement *colly.HTMLElement) {
			if !visitedAlbum.containsTag(tagElement.Text) {
				visitedAlbum.Tags = append(visitedAlbum.Tags, tagElement.Text)
			}
		})

	})

	albumCollector.OnScraped(func(r *colly.Response) {
		fmt.Println("Visited album", r.Request.URL)
	})

	albumCollector.Visit(albumLink)

	return visitedAlbum, true
}

func exportLibrary(username string, verbose bool) {
	libraryCollector := colly.NewCollector()
	libraryAlbums := make(map[string]*album)

	libraryCollector.OnHTML("div.albumBlock", func(albumBlock *colly.HTMLElement) {
		albumBlock.ForEach(("a[href]"), func(_ int, linkElement *colly.HTMLElement) {
			link := linkElement.Attr("href")
			if strings.HasPrefix(link, "/album/") {
				album, ok := visitAlbum("https://www.albumoftheyear.org" + link)
				if ok {
					libraryAlbums[album.ID] = album
				}
			}
		})

	})

	libraryCollector.OnRequest(func(r *colly.Request) {
		fmt.Println("Visiting", r.URL.String())
	})

	libraryCollector.OnHTML("div.pageSelectRow", func(pageSelectElement *colly.HTMLElement) {
		pageSelectElement.ForEach(("a[href]"), func(_ int, pageLinkElement *colly.HTMLElement) {
			pageLinkElement.ForEach("div.pageSelect", func(_ int, pageDiv *colly.HTMLElement) {
				if pageDiv.Text == "Next" {
					nextPageLink := pageLinkElement.Attr("href")
					libraryCollector.Visit("https://www.albumoftheyear.org" + nextPageLink)
				}
			})
		})
	})

	libraryCollector.OnScraped(func(r *colly.Response) {
		fmt.Println("Finished", r.Request.URL)
		for _, album := range libraryAlbums {
			albumJSON, _ := json.MarshalIndent(album, "", "\t")
			fmt.Println(string(albumJSON))
		}
	})

	librayURI := fmt.Sprintf("https://www.albumoftheyear.org/user/%s/library/", username)
	libraryCollector.Visit(librayURI)
}

func main() {
	var dataToExport string
	var userFromWichToExport string
	var verbose bool

	app := &cli.App{
		Name:      "atoy-exporter",
		Usage:     "simple web scraping utility to export library data from albumoftheyear.org ",
		UsageText: "atoy-exporter --user myusername --data library",
		Flags: []cli.Flag{
			&cli.StringFlag{
				Name:        "user",
				Aliases:     []string{"u"},
				Required:    true,
				Usage:       "user from which you want to export data",
				Destination: &userFromWichToExport,
			},
			&cli.StringFlag{
				Name:        "data",
				Aliases:     []string{"d"},
				Value:       "library",
				Required:    false,
				Usage:       "data to export (currently only 'library' accepted)",
				Destination: &dataToExport,
			},
			&cli.BoolFlag{
				Name:        "verbose",
				Aliases:     []string{"v"},
				Value:       false,
				Required:    false,
				Usage:       "show general debug messages",
				Destination: &verbose,
			},
		},
		Action: func(c *cli.Context) error {
			if dataToExport == "library" {
				exportLibrary(userFromWichToExport, verbose)
			}
			return nil
		},
	}

	err := app.Run(os.Args)
	if err != nil {
		log.Fatal(err)
	}
}
