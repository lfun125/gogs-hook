package main

import (
	"bytes"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"io/ioutil"
	"log"
	"net/http"
	"path"
	"strings"

	yaml "gopkg.in/yaml.v2"
)

type Model struct {
	Listen     string `yaml:"listen"`
	JenkinsUrl string `yaml:"jenkins_url"`
	Repository map[string]struct {
		Branches map[string]string `json:"branches"` // branch -> jenkins job
	} `yaml:"repository"`
}

type Push struct {
	Ref        string `json:"ref"`
	Repository struct {
		Name     string `json:"name"`
		FullName string `json:"full_name"`
	} `json:"repository"`
}

func main() {
	var (
		configFile string
	)
	flag.StringVar(&configFile, "f", "./config.yml", "config file")
	flag.Parse()
	configRaw, err := ioutil.ReadFile(configFile)
	if err != nil {
		log.Fatalln(err)
	}
	var Config Model
	if err := yaml.Unmarshal(configRaw, &Config); err != nil {
		log.Fatalln(err)
	}
	http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("X-Gogs-Event") != "push" {
			return
		}
		var data Push
		requestRaw, err := ioutil.ReadAll(r.Body)
		if err != nil {
			w.WriteHeader(http.StatusBadRequest)
			echo(w, err)
			return
		}
		if err := json.NewDecoder(bytes.NewReader(requestRaw)).Decode(&data); err != nil {
			w.WriteHeader(http.StatusBadRequest)
			echo(w, err)
			return
		}
		repository, ok := Config.Repository[data.Repository.FullName]
		if !ok {
			w.WriteHeader(http.StatusInternalServerError)
			echo(w, `Not config this repository`)
			return
		}
		var job string
		for b, j := range repository.Branches {
			if path.Base(data.Ref) == b {
				job = j
				break
			}
		}
		if job == "" {
			w.WriteHeader(http.StatusOK)
			echo(w, `This branch does not need to be released`)
			return
		}
		requestUrl := fmt.Sprintf("%s/gogs-webhook/?job=%s", strings.TrimRight(Config.JenkinsUrl, "/"), job)
		req, err := http.NewRequest(r.Method, requestUrl, bytes.NewReader(requestRaw))
		if err != nil {
			w.WriteHeader(http.StatusInternalServerError)
			echo(w, err)
			return
		}
		for _, k := range []string{"User-Agent", "Content-Type", "X-Gogs-Delivery", "X-Gogs-Event", "X-Gogs-Signature"} {
			req.Header.Set(k, r.Header.Get(k))
		}
		resp, err := http.DefaultClient.Do(req)
		if err != nil {
			w.WriteHeader(http.StatusInternalServerError)
			echo(w, err)
			return
		}
		w.WriteHeader(resp.StatusCode)
		for k := range resp.Header {
			w.Header().Set(k, resp.Header.Get(k))
		}
		if _, err := io.Copy(w, resp.Body); err != nil {
			w.WriteHeader(http.StatusInternalServerError)
			echo(w, err)
			return
		}
	})
	log.Fatalln(http.ListenAndServe(Config.Listen, nil))
}

func echo(w http.ResponseWriter, v interface{}) {
	if _, err := w.Write([]byte(fmt.Sprintf("%v", v))); err != nil {
		log.Println(err)
	}
}
