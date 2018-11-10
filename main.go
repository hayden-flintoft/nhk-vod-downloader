package main

import (
	"context"
	"io"
	"io/ioutil"
	"log"
	"net/http"
	"os"
	"strings"

	"github.com/chromedp/chromedp"
)

func main() {
	const nhkLink = "https://www3.nhk.or.jp/nhkworld/en/vod/directtalk/2058304/"

	key, err := getKey(nhkLink)
	if err != nil {
		log.Fatalln(err)
	}

	playlist, err := getRightPlaylist(key)
	if err != nil {
		log.Panicln(err)
	}

	tsLinks, err := getTSLinks(playlist)
	if err != nil {
		log.Panicln(err)
	}

	err = downloadVideoFragments(tsLinks, key)
	if err != nil {
		log.Panicln(err)
	}

}

func downloadVideoFragments(links []string, tmpFolder string) (error) {
	for _, i := range links[0:2] {
		res, err := http.Get(i)
		if err != nil {
			return err
		}
		defer res.Body.Close()

		splitURL := strings.Split(i, "/")
		fileName := splitURL[len(splitURL)-1]

		os.Mkdir(".\\" + tmpFolder, 0755)
		file, err := os.Create(tmpFolder + "\\" + fileName)
		if err != nil {
			return err
		}
		defer file.Close()

		_, err = io.Copy(file, res.Body)
		if err != nil {
			return err
		}

		return nil
	}

	return nil
}

func getTSLinks(playlist string) ([]string, error) {
	res, err := http.Get(playlist)
	if err != nil {
		return nil, err
	}
	defer res.Body.Close()

	rawPlaylistData, err := ioutil.ReadAll(res.Body)
	if err != nil {
		return nil, err
	}

	listColumns := strings.Split(string(rawPlaylistData), "\n")

	var filteredFiles []string
	for _, i := range listColumns {
		if strings.HasPrefix(i, "https://") {
			filteredFiles = append(filteredFiles, i)
		}
	}

	return filteredFiles, nil
}

func getRightPlaylist(key string) (string, error) {
	playlistList := "https://player.ooyala.com/hls/player/all/" + key + ".m3u8"
	res, err := http.Get(playlistList)
	if err != nil {
		return "", err
	}
	defer res.Body.Close()

	listData, err := ioutil.ReadAll(res.Body)
	if err != nil {
		return "", err
	}

	m3u8List := strings.Split(strings.Trim(string(listData), "\n"), "\n")
	return m3u8List[len(m3u8List)-1], nil
}

func getKey(nhkLink string) (string, error) {
	ctxt, cancel := context.WithCancel(context.Background())
	defer cancel()

	c, err := chromedp.New(
		ctxt,
		chromedp.WithLog(func(string, ...interface{}) {}),
		chromedp.WithRunnerOptions(
			//runner.Flag("headless", true),
			//runner.Flag("disable-gpu", true),
		),
	)
	if err != nil {
		return "", err
	}

	var key string
	err = c.Run(ctxt, getVideoID(nhkLink, &key))

	err = c.Shutdown(ctxt)
	if err != nil {
		return "", err
	}

	err = c.Wait()
	if err != nil {
		return "", err
	}

	return key, nil
}

func getVideoID(nhkLink string, attributes *string) chromedp.Tasks {
	var ok bool
	return chromedp.Tasks{
		chromedp.Navigate(nhkLink),
		chromedp.WaitVisible(`#movie-area-detail`, chromedp.ByID, ),
		chromedp.AttributeValue("#movie-area-detail", "data-id", attributes, &ok, chromedp.ByID),
		chromedp.Stop(),
	}
}