package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"io/ioutil"
	"log"
	"net/http"
	"os"
	"os/exec"
	"os/signal"
	"strconv"
	"strings"
	"syscall"
)

//GitlabRepository represents repository information from the webhook
type GitlabRepository struct {
	Name, Url, Description, Home string
}

//Commit represents commit information from the webhook
type Commit struct {
	Id, Message, Timestamp, Url string
	Author                      Author
}

//Author represents author information from the webhook
type Author struct {
	Name, Email string
}

//Webhook represents push information from the webhook
type Webhook struct {
	Before, After, Ref, User_name string
	User_id, Project_id           int
	Repository                    GitlabRepository
	Commits                       []Commit
	Total_commits_count           int
}

//ConfigRepository represents a repository from the config file
type ConfigRepository struct {
	Name     string
	Commands []string
}

//Config represents the config file
type Config struct {
	Logfile      string
	Address      string
	Port         int64
	Repositories []ConfigRepository
}

func PanicIf(err error, what ...string) {
	if err != nil {
		if len(what) == 0 {
			panic(err)
		}

		panic(errors.New(err.Error() + what[0]))
	}
}

var config Config
var configFile string

func main() {
	args := os.Args

	sigc := make(chan os.Signal, 1)
	signal.Notify(sigc, syscall.SIGHUP)

	go func() {
		<-sigc
		config = loadConfig(configFile)
		log.Println("config reloaded")
	}()

	//if we have a "real" argument we take this as conf path to the config file
	if len(args) > 1 {
		configFile = args[1]
	} else {
		configFile = "config.json"
	}

	//load config
	config := loadConfig(configFile)

	//open log file
	writer, err := os.OpenFile(config.Logfile, os.O_RDWR|os.O_APPEND|os.O_CREATE, 0666)
	PanicIf(err)

	//close logfile on exit
	defer func() {
		writer.Close()
	}()

	//setting logging output
	log.SetOutput(writer)

	//setting handler
	os.Chdir("/home/yang")
	http.HandleFunc("/", hookHandler)

	address := config.Address + ":" + strconv.FormatInt(config.Port, 10)

	log.Println("Listening on " + address)

	//starting server
	err = http.ListenAndServe(address, nil)
	if err != nil {
		log.Println(err)
	}
}

func loadConfig(configFile string) Config {
	var file, err = os.Open(configFile)
	PanicIf(err)

	// close file on exit and check for its returned error
	defer func() {
		err := file.Close()
		PanicIf(err)
	}()

	buffer := make([]byte, 1024)
	count := 0

	count, err = file.Read(buffer)
	PanicIf(err)

	err = json.Unmarshal(buffer[:count], &config)
	PanicIf(err)

	return config
}

func hookHandler(w http.ResponseWriter, r *http.Request) {
	defer func() {
		if r := recover(); r != nil {
			log.Println(r)
		}
	}()

	var hook Webhook

	//read request body
	var data, err = ioutil.ReadAll(r.Body)
	PanicIf(err, "while reading request")

	//unmarshal request body
	err = json.Unmarshal(data, &hook)
	PanicIf(err, "while unmarshaling request")

	branch := strings.Split(hook.Ref, "/")[2]
	url := hook.Repository.Url

	fmt.Println(url)
	fmt.Println(branch)

	//find matching config for repository name
	exec.Command("/usr/bin/git", "clone", url).Output()
}
