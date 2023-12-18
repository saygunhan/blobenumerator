package main

import (
	"encoding/xml"
	"flag"
	"fmt"
	"io/ioutil"
	"net"
	"net/http"
	"os"
	"strings"
	"sync"
	"time"
)

var (
	base     string
	wordlist string
	verbose  bool
	output   string
	blobUrls = map[string]string{
		"onmicrosoft.com":             "Microsoft Hosted Domain",
		"scm.azurewebsites.net":       "App Services - Management",
		"azurewebsites.net":           "App Services",
		"p.azurewebsites.net":         "App Services",
		"cloudapp.net":                "App Services",
		"file.core.windows.net":       "Storage Accounts - Files",
		"blob.core.windows.net":       "Storage Accounts - Blobs",
		"queue.core.windows.net":      "Storage Accounts - Queues",
		"table.core.windows.net":      "Storage Accounts - Tables",
		"mail.protection.outlook.com": "Email",
		"sharepoint.com":              "SharePoint",
		"redis.cache.windows.net":     "Databases-Redis",
		"documents.azure.com":         "Databases-Cosmos DB",
		"database.windows.net":        "Databases-MSSQL",
		"vault.azure.net":             "Key Vaults",
		"azureedge.net":               "CDN",
		"search.windows.net":          "Search Appliance",
		"azure-api.net":               "API Services",
		"azurecr.io":                  "Azure Container Registry",
		"servicebus.windows.net":      "Service Bus",
	}
	keysList         = make([]string, 0, len(blobUrls))
	permutations     = make([]string, 0)
	activeAccounts   = make([]string, 0)
	activeContainers = make([]string, 0)
	foundFiles       = make([]string, 0)
	mutex            = &sync.Mutex{}
)

type Blob struct {
	Name            string `xml:"Name"`
	URL             string `xml:"Url"`
	LastModified    string `xml:"LastModified"`
	Etag            string `xml:"Etag"`
	Size            int    `xml:"Size"`
	ContentType     string `xml:"ContentType"`
	ContentEncoding string `xml:"ContentEncoding"`
	ContentLanguage string `xml:"ContentLanguage"`
}
type EnumerationResults struct {
	ContainerName string `xml:"ContainerName,attr"`
	Blobs         struct {
		Blob []Blob `xml:"Blob"`
	} `xml:"Blobs"`
	NextMarker string `xml:"NextMarker"`
}

func init() {
	for key := range blobUrls {
		keysList = append(keysList, key)
	}
}

func isDomainActive(domain string) string {
	_, err := net.LookupHost(domain)
	if err == nil {
		return domain
	}
	return ""
}

func accountCheck() {
	var wg sync.WaitGroup
	for _, blobURL := range keysList {
		naked := base + "." + blobURL
		ip := isDomainActive(naked)
		if ip != "" {
			fmt.Printf("The domain %s is active with IP address %s.\n", naked, ip)
			activeAccounts = append(activeAccounts, naked)
		} else {
			if !verbose {
				fmt.Printf("The domain %s is not active.\n", naked)
			}
		}
	}

	for _, blobURL := range keysList {
		for _, perm := range permutations {
			wg.Add(1)
			go func(blobURL, perm string) {
				defer wg.Done()
				full := base + perm + "." + blobURL
				ip := isDomainActive(full)
				if ip != "" {
					fmt.Printf("The domain %s is active with IP address %s.\n", full, ip)
					activeAccounts = append(activeAccounts, full)
				} else if verbose {
					fmt.Printf("The domain %s is not active.\n", full)
				}
			}(blobURL, perm)
		}
	}
	wg.Wait()
}

func containerFiles(uri string) {
	response, err := http.Get(uri + "?restype=container&comp=list")
	if err != nil {
		fmt.Println(err)
		return
	}
	defer response.Body.Close()

	if response.StatusCode == 200 {
		body, _ := ioutil.ReadAll(response.Body)
		var root struct {
			Blobs struct {
				Blob []struct {
					Name string `xml:"Name"`
				} `xml:"Blob"`
			} `xml:"Blobs"`
		}
		if err := xml.Unmarshal(body, &root); err == nil {
			for _, blob := range root.Blobs.Blob {
				fileURI := uri + "/" + blob.Name
				if !strings.HasSuffix(uri, "/") {
					fileURI = uri + "/" + blob.Name
				}
				foundFiles = append(foundFiles, fileURI)
			}
		}
	}
}

func containerCheck() {
	var wg sync.WaitGroup
	for _, account := range activeAccounts {
		for _, perm := range permutations {
			wg.Add(1)
			go func(account, perm string) {
				defer wg.Done()
				dir := account + "/" + perm
				uri := "https://" + dir + "?restype=container&comp=list"
				response, err := http.Get(uri)
				if err != nil {
					fmt.Println(err)
					return
				}
				defer response.Body.Close()

				if response.StatusCode == 200 {
					withLock(func() {
						activeContainers = append(activeContainers, "https://"+dir)
					})
				} else if verbose {
					fmt.Printf("%s is not active\n", uri)
				}
			}(account, perm)
		}
	}
	wg.Wait()
}

func withLock(fn func()) {
	var mu sync.Mutex
	mu.Lock()
	fn()
	mu.Unlock()
}

func start() {
	fmt.Println("Starting")
	accountCheck()
	fmt.Println("########## Active Accounts ##########")
	fmt.Println(activeAccounts)

	containerCheck()
	fmt.Println("########## Active Containers ##########")
	fmt.Println(activeContainers)

	fmt.Println("########## Files Found ##########")
	for _, uri := range activeContainers {
		containerFiles(uri)
	}
	fmt.Println(foundFiles)

	if output != "" {
		f, err := os.OpenFile(output, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
		if err != nil {
			fmt.Println(err)
			return
		}
		defer f.Close()

		// Write active accounts to the output file
		for _, account := range activeAccounts {
			f.WriteString("bucket: " + account + "\n")
		}

		// Write active containers to the output file
		for _, container := range activeContainers {
			f.WriteString("url: " + container + "\n")
		}

		// Write found files to the output file
		for _, file := range foundFiles {
			f.WriteString("data: " + file + "\n")
		}
	}
}

func main() {
	flag.StringVar(&base, "b", "", "The URL to check.")
	flag.StringVar(&wordlist, "w", "", "The wordlist for permutation. By default, it will use perm.txt.")
	flag.BoolVar(&verbose, "v", false, "Verbosity.")
	flag.StringVar(&output, "o", "", "Output to a file.")
	flag.Parse()

	if base == "" {
		fmt.Println("Please provide the base URL using the -b flag.")
		return
	}

	if wordlist == "" {
		wordlist = "perm.txt"
		fmt.Println("Using default wordlist")
	}

	contents, err := ioutil.ReadFile(wordlist)
	if err != nil {
		fmt.Println(err)
		return
	}
	permutations = []string{}
	lines := strings.Split(string(contents), "\n")
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line != "" {
			permutations = append(permutations, line)
		}
	}

	start_time := time.Now()
	start()
	elapsed_time := time.Since(start_time)
	fmt.Printf("Script execution time: %s\n", elapsed_time)
}
