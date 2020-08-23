package main

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"log"
	"os"
	"strings"
	"time"

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
	MyTags      []string
}

func newAlbum() *album {
	var a album
	a.Tags = make([]string, 0)
	a.MyTags = make([]string, 0)
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

func sliceOfStringsContainsElement(slice []string, element string) bool {
	for _, e := range slice {
		if e == element {
			return true
		}
	}
	return false
}

func printMessage(message string, verbose bool) {
	if verbose {
		fmt.Println(message)
	}
}

func visitAlbum(albumLink string, verbose bool) (*album, bool) {
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
		printMessage("Visiting album "+r.URL.String(), verbose)
	})

	albumCollector.OnHTML("div.albumHeadline div.artist", func(artistElement *colly.HTMLElement) {
		printMessage("Artist: "+artistElement.Text, verbose)
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
		printMessage("Visited "+r.Request.URL.String(), verbose)
		printMessage("Data obtained: "+fmt.Sprintf("%#v", visitedAlbum), verbose)
	})

	albumCollector.Visit(albumLink)

	return visitedAlbum, true
}

func writeLibraryJSONFile(username string, libraryData map[string]*album) {
	libraryAlbums := libraryMapToSlice(libraryData)
	YYYYMMddmmss := "20060102150405"
	timestamp := time.Now().Format(YYYYMMddmmss)
	filename := fmt.Sprintf("atoy_%s_library_%s.json", username, timestamp)
	libraryJSONData, _ := json.MarshalIndent(libraryAlbums, "", "\t")
	err := ioutil.WriteFile(filename, libraryJSONData, 0644)
	if err != nil {
		fmt.Println(err)
	}
}

func libraryMapToSlice(libraryMap map[string]*album) []*album {
	librarySlice := make([]*album, 0, len(libraryMap))
	for _, value := range libraryMap {
		librarySlice = append(librarySlice, value)
	}
	return librarySlice
}

func exportLibrary(username string, verbose bool, myTags map[string][]string) {
	fmt.Println("Exporting...")
	libraryCollector := colly.NewCollector()
	libraryAlbums := make(map[string]*album)

	libraryCollector.OnHTML("div.albumBlock", func(albumBlock *colly.HTMLElement) {
		albumBlock.ForEach(("a[href]"), func(_ int, linkElement *colly.HTMLElement) {
			link := linkElement.Attr("href")
			if strings.HasPrefix(link, "/album/") {
				album, ok := visitAlbum("https://www.albumoftheyear.org"+link, verbose)
				if ok {
					album.MyTags = myTags[album.ID]
					libraryAlbums[album.ID] = album
				}
			}
		})

	})

	libraryCollector.OnRequest(func(r *colly.Request) {
		printMessage("Visiting library "+r.URL.String(), verbose)
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
		printMessage("Visited library "+r.Request.URL.String(), verbose)
	})

	librayURI := fmt.Sprintf("https://www.albumoftheyear.org/user/%s/library/", username)
	libraryCollector.Visit(librayURI)

	writeLibraryJSONFile(username, libraryAlbums)
	fmt.Println("Done!")
}

func loadMyTags(username string, verbose bool) map[string][]string {
	printMessage("Loading my tags...", verbose)
	myTags := make(map[string][]string)
	myTagsCollector := colly.NewCollector()

	myTagsCollector.OnHTML("div.tag", func(tagBlock *colly.HTMLElement) {
		tagBlock.ForEach(("a[href]"), func(_ int, linkElement *colly.HTMLElement) {
			myTagLink := linkElement.Attr("href")
			myTag := linkElement.Text
			albumIDs := getAlbumsIDForTag(username, myTag, myTagLink, verbose)
			for _, albumID := range albumIDs {
				albumMyTags, _ := myTags[albumID]
				if !sliceOfStringsContainsElement(albumMyTags, myTag) {
					albumMyTags = append(albumMyTags, myTag)
					myTags[albumID] = albumMyTags
				}
			}
		})

	})

	myTagsURI := fmt.Sprintf("https://www.albumoftheyear.org/user/%s/tags/?s=name", username)
	myTagsCollector.Visit(myTagsURI)

	return myTags
}

func getAlbumsIDForTag(username string, myTag string, myTagLink string, verbose bool) []string {
	printMessage("Loading albums with myTag "+myTag, verbose)
	albumIDs := make([]string, 0)
	myTagsAlbumsCollector := colly.NewCollector()
	myTagsAlbumsCollector.OnHTML("div.albumBlock", func(albumBlock *colly.HTMLElement) {
		albumBlock.ForEach(("a[href]"), func(_ int, linkElement *colly.HTMLElement) {
			albumLink := linkElement.Attr("href")
			if strings.HasPrefix(albumLink, "/album/") {
				albumID := albumLink[7:strings.Index(albumLink, "-")]
				printMessage("Found album id "+albumID+" with myTag "+myTag, verbose)
				albumIDs = append(albumIDs, albumID)
			}
		})
	})

	myTagsAlbumsCollector.Visit("https://www.albumoftheyear.org" + myTagLink)

	return albumIDs
}

func main() {
	var dataToExport string
	var userFromWichToExport string
	var verbose bool
	var withMyTags bool

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
			&cli.BoolFlag{
				Name:        "with-my-tags",
				Aliases:     []string{"t"},
				Value:       true,
				Required:    false,
				Usage:       "add 'my tags' (user custom tags) to exported albums",
				Destination: &withMyTags,
			},
		},
		Action: func(c *cli.Context) error {
			myTags := make(map[string][]string)
			if withMyTags {
				myTags = loadMyTags(userFromWichToExport, verbose)
			}
			if dataToExport == "library" {
				exportLibrary(userFromWichToExport, verbose, myTags)
			}
			return nil
		},
	}

	err := app.Run(os.Args)
	if err != nil {
		log.Fatal(err)
	}
}
