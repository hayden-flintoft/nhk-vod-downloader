package main

import (
	"context"
	"io"
	"io/ioutil"
	"log"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/chromedp/chromedp"
)

func main() {
	// This will tell use the total execution time after we finish.
	defer timer(time.Now())

	// This is a fixed link for testing purposes. Remove this when you can, please.
	const nhkLink = "https://www3.nhk.or.jp/nhkworld/en/vod/directtalk/2058304/"

	// First, we need to get the video ID from NHK's website.
	log.Println("Obtaining the video ID...")
	key, err := getKey(nhkLink)
	if err != nil {
		log.Fatalln(err)
	}

	// Then, we need to obtain the playlist of the video fragments of highest resolution.
	log.Println("Getting the best quality playlist...")
	playlist, err := getRightPlaylist(key)
	if err != nil {
		log.Panicln(err)
	}

	// Once we have the playlist, we need to actually get the video file links.
	log.Println("Finding all TS video file links...")
	tsLinks, err := getTSLinks(playlist)
	if err != nil {
		log.Panicln(err)
	}

	// Here, we download all TS fragment files.
	log.Println("Downloading all video fragments...")
	var fileLocations []string
	err = downloadVideoFragments(&fileLocations, tsLinks, key)
	if err != nil {
		log.Panicln(err)
	}

	// After downloading everything, all we need to do it merge all small files.
	log.Println("Merging everything into the final file...")
	err = mergeVideoFragments(&fileLocations, key)
	if err != nil {
		log.Panicln(err)
	}
}

// We use this function to merge all TS files into a big one.
func mergeVideoFragments(fileLocations *[]string, key string) error {
	// Create a new file and open it for appending.
	finalFile, err := os.OpenFile(".\\"+key+".ts", os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		return err
	}
	defer finalFile.Close()

	// We'll run the following code for each video fragment.
	for _, i := range *fileLocations {
		// Read the video fragment file content.
		smallFileContents, err := ioutil.ReadFile(i)
		if err != nil {
			return err
		}

		// Append the fragment data to the final file.
		_, err = finalFile.Write(smallFileContents)
		if err != nil {
			return err
		}
	}

	return nil
}

// Here, we actually download the TS files.
func downloadVideoFragments(fileLocations *[]string, links []string, tmpFolder string) error {
	// Create a new folder for saving the files.
	os.Mkdir(".\\"+tmpFolder, 0755)

	for _, i := range links {
		// Get the video from the URL.
		res, err := http.Get(i)
		if err != nil {
			return err
		}

		// Get the filename because we'll need it to save the video.
		splitURL := strings.Split(i, "/")
		fileName := splitURL[len(splitURL)-1]
		fullFileLocation := ".\\" + tmpFolder + "\\" + fileName

		// Create a new file.
		file, err := os.Create(fullFileLocation)
		if err != nil {
			return err
		}

		// Save the video into the new file.
		_, err = io.Copy(file, res.Body)
		if err != nil {
			return err
		}

		// Add the file path to a list - we'll need that later.
		*fileLocations = append(*fileLocations, fullFileLocation)

		// Close the file and the connection.
		file.Close()
		res.Body.Close()
	}

	return nil
}

// Function getTSLinks loads the playlist file and retrieves a list of all TS video fragments. Hooray.
func getTSLinks(playlist string) ([]string, error) {
	// First, let us load the contents of the playlist file.
	res, err := http.Get(playlist)
	if err != nil {
		return nil, err
	}
	defer res.Body.Close()

	rawPlaylistData, err := ioutil.ReadAll(res.Body)
	if err != nil {
		return nil, err
	}

	// Next, we split the playlist by the new line character.
	listColumns := strings.Split(string(rawPlaylistData), "\n")

	// Some lines contain the video fragment links, some contain rubbish info we don't need.
	// Let's collect only the lines we need, since this is the only thing that makes sense.
	var filteredFiles []string
	for _, i := range listColumns {
		if strings.HasPrefix(i, "https://") {
			filteredFiles = append(filteredFiles, i)
		}
	}

	return filteredFiles, nil
}

// There are multiple video qualities for each video. This function obtains the playlist for the right one.
func getRightPlaylist(key string) (string, error) {
	// First, we get the playlists for all possible qualities from Ooyala.
	// Fun fact: Ooyala is apparently a service that hosts video files and lets people stream from their servers.
	// Wait, that wasn't fun. At least it was a fact, though.
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

	// The playlist at the bottom at the file is the one of the highest resolution.
	// We take this one because we obviously like good quality video.
	m3u8List := strings.Split(strings.Trim(string(listData), "\n"), "\n")
	return m3u8List[len(m3u8List)-1], nil
}

// We need to get a video ID so we know what to download.
// NHK doesn't load the video onto the website right away, instead they use JS to load it afterwards.
// Because of this, the source HTML file is void of any useful information and I was unable to use goquery,
// my favourite package for scraping from the web. How dare they.
// I had to use chromedp, which is still quite cool, but I feel like it's an overkill for this specific usage.
// It's also a bit broken. If you use the headless mode, you can't shut it down properly, or at least I wasn't able to
// figure it out since there is just about zero useful documentation about it on Google. If you're reading this and
// know how to help, do contact me - I've been dying to know this for quite some time now.
func getKey(nhkLink string) (string, error) {

	// First, we define a Context. I'm note 100% sure what this is, but it seemed interesting enough
	// on the documentation page and chromedp needs it to work.
	ctxt, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Start a new chromedp instance.
	c, err := chromedp.New(
		ctxt,
		chromedp.WithLog(func(string, ...interface{}) {}),
		// We've got some commented headless flags here. Uncomment them if you have a death wish.
		// (Just kidding. Don't uncomment them under any circumstances. Do contact somebody if you have a death wish though.)
		chromedp.WithRunnerOptions(
			//runner.Flag("headless", true),
			//runner.Flag("disable-gpu", true),
		),
	)
	if err != nil {
		return "", err
	}

	// Define a key variable - we will store the video ID here. Then, run the Chrome tasks.
	var key string
	err = c.Run(ctxt, chromeTasks(nhkLink, &key))

	// When we're all done, let Uncle Chrome know it's time to go.
	err = c.Shutdown(ctxt)
	if err != nil {
		return "", err
	}

	// Wait for Uncle Chrome to go.
	err = c.Wait()
	if err != nil {
		return "", err
	}

	return key, nil
}

// These are the tasks we use to obtain the video ID from NHK.
func chromeTasks(nhkLink string, attributes *string) chromedp.Tasks {
	var ok bool
	return chromedp.Tasks{
		// Go to the actual video webpage.
		chromedp.Navigate(nhkLink),

		// Wait until NHK decides to load the video. This may actually take a few seconds.
		// If it takes more than a few seconds, refresh the page in the Chrome window and wait some more. It may help.
		chromedp.WaitVisible(`#movie-area-detail`, chromedp.ByID, ),

		// When the video loads, get the ID from the video div.
		chromedp.AttributeValue("#movie-area-detail", "data-id", attributes, &ok, chromedp.ByID),

		chromedp.Stop(),
	}
}

// Time measuring stuff.
func timer(s time.Time) {
	elapsed := time.Since(s)
	log.Printf("Execution time: %s", elapsed)
}
