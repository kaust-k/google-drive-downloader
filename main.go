package main

import (
	"encoding/json"
	"fmt"
	"io"
	"io/ioutil"
	"log"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"time"
)

var (
	rootDir string
	args    []string
	ct      time.Time

	monthNames = map[int]string{1: "JAN", 2: "FEB", 3: "MAR", 4: "APR",
		5: "MAY", 6: "JUN", 7: "JUL", 8: "AUG",
		9: "SEP", 10: "OCT", 11: "NOV", 12: "DEC"}
)

const (
	googleKey                  = "<google-key-here>"
	googleDriveFileListAPIURL  = "https://www.googleapis.com/drive/v3/files"
	googleDriveFileDownloadURL = "https://www.googleapis.com/drive/v3/files/%s?alt=media&key=%s"
	googleDataFolderID         = "<google-drive-folder-id>"
)


type gFile struct {
	ID       string `json:"id"`
	Name     string `json:"name"`
	MimeType string `json:"mimeType"`
}

type gFolder struct {
	Files []gFile `json:"files"`
}

func main() {
	args = os.Args[1:]
	if len(args) > 0 {
		rootDir = args[0]
	} else {
		rootDir = "<default-dir>"
	}

	ct = time.Now()

	folder := getFileList(googleDataFolderID)
	if folder == nil {
		return
	}

	handleYearsData(folder)
}

func createYearDir(t time.Time) {
	newpath := filepath.Join(rootDir, strconv.Itoa(t.Year()))
	fmt.Println("Creating ", newpath)
	os.MkdirAll(newpath, os.ModePerm)

	newpath = filepath.Join(rootDir, strconv.Itoa(t.Year()-1))
	fmt.Println("Creating ", newpath)
	os.MkdirAll(newpath, os.ModePerm)
}

func getFileList(folderID string) *gFolder {
	req, err := http.NewRequest("GET", googleDriveFileListAPIURL, nil)
	if err != nil {
		log.Print(err)
		os.Exit(1)
	}

	// https://www.googleapis.com/drive/v3/files?q=%271OMbptcweD66JA8u6K1GL6Q_6hw7BacZO%27+in+parents&key=...
	// space is encoded as +
	q := req.URL.Query()
	q.Add("q", fmt.Sprintf("'%s' in parents", folderID))
	q.Add("key", googleKey)

	req.URL.RawQuery = q.Encode()
	fmt.Println(req.URL.String())
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		fmt.Printf("%+v\n\n", err)
		return nil
	}

	// Read body
	b, err := ioutil.ReadAll(resp.Body)
	defer resp.Body.Close()
	if err != nil {
		fmt.Printf("%+v\n\n", err)
		return nil
	}

	// Unmarshal
	var msg gFolder
	err = json.Unmarshal(b, &msg)
	if err != nil {
		fmt.Printf("%+v\n\n", err)
		return nil
	}

	return &msg
}

func handleYearsData(gf *gFolder) {
	var (
		currentYear = ct.Year()
		loopTill    = ct.Year() - 1
		cont        bool
	)

	for currentYear >= loopTill {
		str := strconv.Itoa(currentYear)
		fmt.Println("  Year: ", str)
		if cont = handleMonthsData(getFileList(getIDForName(gf, str))); !cont {
			break
		}
		currentYear--
	}
}

func handleMonthsData(gf *gFolder) bool {
	if gf == nil {
		return false
	}

	var (
		month   string
		newpath string
		cont    bool
	)

	for {
		month = monthNames[int(ct.Month())]
		newpath = filepath.Join(rootDir, strconv.Itoa(ct.Year()), month)
		fmt.Println("Creating ", newpath)
		os.MkdirAll(newpath, os.ModePerm)

		if cont = handleDaysData(getFileList(getIDForName(gf, month))); !cont {
			return false
		}
	}
	return true
}

func handleDaysData(gf *gFolder) bool {
	if gf == nil {
		return false
	}

	var (
		month    = monthNames[int(ct.Month())]
		newpath  string
		filename string
		err      error
	)

	for {
		filename = fmt.Sprintf("%02d%s.zip", ct.Day(), month)
		newpath = filepath.Join(rootDir, strconv.Itoa(ct.Year()), month, filename)
		if _, err = os.Stat(newpath); os.IsNotExist(err) {
			fmt.Println("  File doesn't exist: ", newpath)
			if err = downloadFile(newpath, getIDForName(gf, fmt.Sprintf("%02d.*.zip", ct.Day()))); err != nil {
				fmt.Println("  Error downloading file ", err)
				return false
			} else {
				tt := ct.AddDate(0, 0, -1)
				ttMonth := monthNames[int(tt.Month())]
				ct = tt
				if ttMonth != month {
					return true
				}
			}
		} else {
			return false
		}
	}
}

func getIDForName(gf *gFolder, id string) string {
	r, _ := regexp.Compile(id)
	for _, g := range gf.Files {
		if r.MatchString(g.Name) {
			return g.ID
		}
	}

	return ""
}

func downloadFile(filepath string, fileID string) error {
	if fileID == "" {
		return nil
	}

	// Create the file
	out, err := os.Create(filepath)
	if err != nil {
		return err
	}
	defer out.Close()

	downloadURL, _ := url.Parse(fmt.Sprintf(googleDriveFileDownloadURL, fileID, googleKey))

	// Get the data
	fmt.Println("Downloading ", downloadURL.String())

	resp, err := http.Get(downloadURL.String())
	if err != nil {
		return err
	}

	if resp.StatusCode != 200 {
		return nil
	}

	defer resp.Body.Close()
	_, err = io.Copy(out, resp.Body)
	if err != nil {
		return err
	}

	return nil
}
