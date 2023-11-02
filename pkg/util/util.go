package util

import (
	"cloud.google.com/go/storage"
	"context"
	"crypto/md5"
	"crypto/sha1"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"github.com/google/go-github/v27/github"
	go_logger "github.com/pefish/go-logger"
	"google.golang.org/api/option"
	"hash"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path"
	"regexp"
	"runtime"
	"selenium-server/pkg/global"
	"strings"
	"sync"
)

type file struct {
	url      string
	name     string
	hash     string
	hashType string // default is sha256
	rename   []string
	browser  bool
}

var files = []file{
	{
		// https://console.cloud.google.com/storage/browser/selenium-release
		url: "https://selenium-release.storage.googleapis.com/3.141/selenium-server-standalone-3.141.59.jar",
		//url:  "https://storage.cloud.google.com/selenium-release/3.9/selenium-server-standalone-3.9.1.jar",
		name: "selenium-server.jar",
		// TODO(minusnine): reimplement hashing so that it is less annoying for maintenance.
		// hash: "acf71b77d1b66b55db6fb0bed6d8bae2bbd481311bcbedfeff472c0d15e8f3cb",
	},
	//{
	//	url:    "https://saucelabs.com/downloads/sc-4.5.4-linux.tar.gz",
	//	name:   "sauce-connect.tar.gz",
	//	rename: []string{"sc-4.5.4-linux", "sauce-connect"},
	//},
}

func addLatestGithubRelease(ctx context.Context, owner, repo, assetName, localFileName string) error {
	client := github.NewClient(nil)

	rel, _, err := client.Repositories.GetLatestRelease(ctx, owner, repo)
	if err != nil {
		return err
	}
	assetNameRE, err := regexp.Compile(assetName)
	if err != nil {
		return fmt.Errorf("invalid asset name regular expression %q: %s", assetName, err)
	}
	for _, a := range rel.Assets {
		if !assetNameRE.MatchString(a.GetName()) {
			continue
		}
		u := a.GetBrowserDownloadURL()
		if u == "" {
			return fmt.Errorf("%s does not have a download URL", a.GetName())
		}
		files = append(files, file{
			name: localFileName,
			url:  u,
		})
		return nil
	}

	return fmt.Errorf("Release for %s not found at http://github.com/%s/%s/releases", assetName, owner, repo)
}

func addChromeDriver(ctx context.Context, latestChromeBuild string) error {
	prefix := ""
	suffix := ""
	switch runtime.GOOS {
	case "windows":
		prefix = "Win_x64"
		suffix = "win32"
	case "darwin":
		prefix = "Mac"
		suffix = "mac64"
	case "linux":
		prefix = "Linux_x64"
		suffix = "linux64"
	}
	// Bucket URL: https://console.cloud.google.com/storage/browser/chromium-browser-snapshots/?pli=1
	storageBktName := "chromium-browser-snapshots"
	chromeDriverName := "chromedriver"
	gcsPath := fmt.Sprintf("gs://%s/", storageBktName)
	client, err := storage.NewClient(ctx, option.WithHTTPClient(http.DefaultClient))
	if err != nil {
		return fmt.Errorf("cannot create a storage client for downloading the chrome browser: %v", err)
	}
	bkt := client.Bucket(storageBktName)
	if latestChromeBuild == "" {
		lastChangeFile := fmt.Sprintf("%s/LAST_CHANGE", prefix)
		r, err := bkt.Object(lastChangeFile).NewReader(ctx)
		if err != nil {
			return fmt.Errorf("cannot create a reader for %s%s file: %v", gcsPath, lastChangeFile, err)
		}
		defer r.Close()
		// Read the last change file content for the latest build directory name
		data, err := io.ReadAll(r)
		if err != nil {
			return fmt.Errorf("cannot read from %s%s file: %v", gcsPath, lastChangeFile, err)
		}
		latestChromeBuild = string(data)
		//go_logger.Logger.InfoF("latestChromeBuild: %s", latestChromeBuild)
	}
	filename := fmt.Sprintf("%s_%s.zip", chromeDriverName, suffix)
	latestChromeDriverPackage := path.Join(prefix, latestChromeBuild, filename)
	//go_logger.Logger.InfoF("file: %s", latestChromeDriverPackage)
	cpAttrs, err := bkt.Object(latestChromeDriverPackage).Attrs(ctx)
	if err != nil {
		return fmt.Errorf("cannot get the chrome driver package %s%s attrs: %v", gcsPath, latestChromeDriverPackage, err)
	}
	files = append(files, file{
		name: filename,
		url:  cpAttrs.MediaLink,
		rename: []string{
			fmt.Sprintf("%s_%s/%s", chromeDriverName, suffix, chromeDriverName),
			chromeDriverName,
		},
	})
	return nil
}

func addFirefoxDriver(ctx context.Context, version string) error {
	return addLatestGithubRelease(
		ctx,
		"mozilla",
		"geckodriver",
		"geckodriver-.*linux64.tar.gz",
		"geckodriver.tar.gz",
	)
}

func handleFile(file file, downloadBrowsers bool) error {
	if file.browser && !downloadBrowsers {
		go_logger.Logger.InfoF("Skipping %q because --download_browser is not set.", file.name)
		return nil
	}
	if file.hash != "" && fileSameHash(file) {
		go_logger.Logger.InfoF("Skipping file %q which has already been downloaded.", file.name)
	} else {
		go_logger.Logger.InfoF("Downloading %q from %q", file.name, file.url)
		if err := downloadFile(file); err != nil {
			return err
		}
	}

	switch path.Ext(file.name) {
	case ".zip":
		go_logger.Logger.InfoF("Unzipping %q", file.name)
		if err := exec.Command("unzip", "-o", file.name).Run(); err != nil {
			return fmt.Errorf("Error unzipping %q: %v", file.name, err)
		}
		os.RemoveAll(file.name)
	case ".gz":
		go_logger.Logger.InfoF("Unzipping %q", file.name)
		if err := exec.Command("tar", "-xzf", file.name).Run(); err != nil {
			return fmt.Errorf("Error unzipping %q: %v", file.name, err)
		}
		os.RemoveAll(file.name)
	case ".bz2":
		go_logger.Logger.InfoF("Unzipping %q", file.name)
		if err := exec.Command("tar", "-xjf", file.name).Run(); err != nil {
			return fmt.Errorf("Error unzipping %q: %v", file.name, err)
		}
		os.RemoveAll(file.name)
	}
	if rename := file.rename; len(rename) == 2 {
		go_logger.Logger.InfoF("Renaming %q to %q", rename[0], rename[1])
		os.RemoveAll(rename[1])
		if err := os.Rename(rename[0], rename[1]); err != nil {
			return fmt.Errorf("Error renaming %q to %q: %v", rename[0], rename[1], err)
		}
		arr := strings.Split(rename[0], "/")
		if len(arr) > 1 {
			os.RemoveAll(arr[0])
		}
	}
	return nil
}

func downloadFile(file file) (err error) {
	f, err := os.Create(file.name)
	if err != nil {
		return fmt.Errorf("error creating %q: %v", file.name, err)
	}
	defer func() {
		if closeErr := f.Close(); closeErr != nil && err == nil {
			err = fmt.Errorf("error closing %q: %v", file.name, err)
		}
	}()

	resp, err := http.Get(file.url)
	if err != nil {
		return fmt.Errorf("%s: error downloading %q: %v", file.name, file.url, err)
	}
	defer resp.Body.Close()
	if file.hash != "" {
		var h hash.Hash
		switch strings.ToLower(file.hashType) {
		case "md5":
			h = md5.New()
		case "sha1":
			h = sha1.New()
		default:
			h = sha256.New()
		}
		if _, err := io.Copy(io.MultiWriter(f, h), resp.Body); err != nil {
			return fmt.Errorf("%s: error downloading %q: %v", file.name, file.url, err)
		}
		if h := hex.EncodeToString(h.Sum(nil)); h != file.hash {
			return fmt.Errorf("%s: got %s hash %q, want %q", file.name, file.hashType, h, file.hash)
		}
	} else {
		if _, err := io.Copy(f, resp.Body); err != nil {
			return fmt.Errorf("%s: error downloading %q: %v", file.name, file.url, err)
		}
	}
	return nil
}

func fileSameHash(file file) bool {
	if _, err := os.Stat(file.name); err != nil {
		return false
	}
	var h hash.Hash
	switch strings.ToLower(file.hashType) {
	case "md5":
		h = md5.New()
	default:
		h = sha256.New()
	}
	f, err := os.Open(file.name)
	if err != nil {
		return false
	}
	defer f.Close()

	if _, err := io.Copy(h, f); err != nil {
		return false
	}

	sum := hex.EncodeToString(h.Sum(nil))
	if sum != file.hash {
		go_logger.Logger.WarnF("File %q: got hash %q, expect hash %q", file.name, sum, file.hash)
		return false
	}
	return true
}

func FileExists(filename string) bool {
	info, err := os.Stat(filename)
	if os.IsNotExist(err) {
		return false
	}
	return !info.IsDir()
}

func DownloadDeps(ctx context.Context, browserName, driverBuildVersion string) error {

	switch browserName {
	case string(global.Browser_Firefox):
		if err := addFirefoxDriver(ctx, driverBuildVersion); err != nil {
			go_logger.Logger.ErrorF("Unable to download firefox driver: %v", err)
		}
	case string(global.Browser_Chrome):
		if err := addChromeDriver(ctx, driverBuildVersion); err != nil {
			go_logger.Logger.ErrorF("Unable to download chrome driver: %v", err)
		}
	}

	var wg sync.WaitGroup
	for _, file := range files {
		wg.Add(1)
		file := file
		go func() {
			if err := handleFile(file, true); err != nil {
				go_logger.Logger.ErrorF("Error handling %s: %s", file.name, err)
				os.Exit(1)
			}
			wg.Done()
		}()
	}
	wg.Wait()
	return nil
}
