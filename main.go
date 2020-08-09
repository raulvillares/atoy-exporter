package main

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/gocolly/colly/v2"
)

type Album struct {
	ID          string
	Artist      string
	AlbumTitle  string
	ReleaseYear string
	Format      string
	Label       string
	Genres      string
	Tags        []string
}

func newAlbum() *Album {
	var a Album
	a.Tags = make([]string, 0)
	return &a
}

func (album Album) containsTag(tag string) bool {
	for _, item := range album.Tags {
		if item == tag {
			return true
		}
	}
	return false
}

func visitAlbum(albumLink string) (*Album, bool) {
	album := newAlbum()
	albumID := albumLink[30+7 : strings.Index(albumLink, "-")]
	album.ID = albumID

	albumCollector := colly.NewCollector()

	// ToDo This does not work, review Colly docs
	albumAlreadyVisited, _ := albumCollector.HasVisited(albumLink)
	if albumAlreadyVisited {
		return album, false
	}

	albumCollector.OnRequest(func(r *colly.Request) {
		fmt.Println("Visiting album", r.URL.String())
	})

	albumCollector.OnHTML("div.albumHeadline div.artist", func(artistElement *colly.HTMLElement) {
		album.Artist = artistElement.Text
	})

	albumCollector.OnHTML("div.albumHeadline div.albumTitle", func(albumTitleElement *colly.HTMLElement) {
		album.AlbumTitle = albumTitleElement.Text
	})

	albumCollector.OnHTML("div.albumTopBox.info", func(albumInfoElement *colly.HTMLElement) {
		albumInfoElement.ForEach("div.detailRow", func(_ int, detailElement *colly.HTMLElement) {
			detailText := detailElement.Text
			if strings.HasSuffix(detailText, "Release Date") {
				releaseDate := detailText[:strings.LastIndex(detailText, "Release Date")-4]
				releaseYear := releaseDate[len(releaseDate)-4:]
				album.ReleaseYear = releaseYear
			}
			if strings.HasSuffix(detailText, "Format") {
				format := detailText[:strings.LastIndex(detailText, "Format")-4]
				album.Format = format
			}

			if strings.HasSuffix(detailText, "Label") {
				label := detailText[:strings.LastIndex(detailText, "Label")-4]
				album.Label = label
			}

			if strings.HasSuffix(detailText, "Genres") {
				genres := detailText[:strings.LastIndex(detailText, "Genres")-4]
				album.Genres = genres
			}
		})

		albumInfoElement.ForEach("div.tag.strong", func(_ int, tagElement *colly.HTMLElement) {
			if !album.containsTag(tagElement.Text) {
				album.Tags = append(album.Tags, tagElement.Text)
			}
		})

	})

	albumCollector.OnScraped(func(r *colly.Response) {
		fmt.Println("Visited album", r.Request.URL)
	})

	albumCollector.Visit(albumLink)

	return album, true
}

func exportLibrary(username string) {
	libraryCollector := colly.NewCollector()
	libraryAlbums := make(map[string]*Album)

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
	exportLibrary("atoyexporter")
}
