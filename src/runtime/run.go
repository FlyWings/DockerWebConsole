package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"time"
	"html/template"
	"io/ioutil"
	"log"
	"myssh"
	"net"
	"net/http"
	"regexp"
	"strings"
	"github.com/skratchdot/open-golang/open"
	"os/exec"
	"os"
)

type Port struct {
	IP          string
	PrivatePort int
	PublicPort  int
	Type        string
}

type Label struct {
}

type HostConfig struct {
}

type Container struct {
	Id         string
	Names      []string
	Image      string
	Command    string
	Created    int
	Ports      []Port
	Labels     Label
	Status     string
	HostConfig HostConfig
}

var (
	addr = flag.Bool("addr", false, "find open address and print to final-port.txt")

	host = "localhost"

	ssh = &myssh.MakeConfig{
		User:     "root",
		Password: "Robfrut.512",
		Server:   host,
		Port:     "22",
	}

	resources_path = "../resources/"
	templates_path = resources_path + "templates/"
	text_path = resources_path +  "text/"

	templates = template.Must(template.ParseFiles("../resources/web/view.html", "../resources/web/index.html", "../resources/web/contact.html"))

	validPath = regexp.MustCompile("^/(view|contact)/([a-zA-Z0-9]+)$")
)

type Page struct {
	Title string
	Body  []byte
	Containers map[string]Container
}

func (p *Page) save() error {
	filename := p.Title + ".txt"
	return ioutil.WriteFile(text_path + filename, p.Body, 0600)
}

func loadPage(title string) (*Page, error) {
	filename := title + ".txt"
	body, err := ioutil.ReadFile(text_path + filename)
	if err != nil {
		return nil, err
	}
	return &Page{Title: title, Body: body}, nil
}

func viewHandler(w http.ResponseWriter, r *http.Request, title string) {
	p, err := loadPage(title)
	if err != nil {
		http.NotFound(w, r)
		return
	}

	c := make(chan bool)
	go func() {
		id := r.FormValue("id")
		cmd := "gotty --once -w -p 9999 docker exec -ti " + id + " bash"
		out, err := ssh.Run(cmd)
		if err != nil {
			panic("Can't run remote command: " + err.Error() + out)
		}
		c <- true
	}()

	time.Sleep(2 * time.Second)

	open.RunWith("http://"+ host +":9999", "firefox")

	// wait for the blocking function to finish if it hasn't already
	<-c

	renderTemplate(w, "view", p)
}

func contactHandlerNil(w http.ResponseWriter, r *http.Request) {

	renderTemplate(w, "contact", &Page{})
}

func contactHandler(w http.ResponseWriter, r *http.Request, title string) {
	if title == "send" {
		/*
			TODO: send data
		*/

		http.Redirect(w, r, "/", 301)
	} else {
		http.NotFound(w, r)
	}
}


func renderTemplateNil(w http.ResponseWriter, tmpl string) {
	err := templates.ExecuteTemplate(w, tmpl+".html", nil)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
	}
}

func renderTemplate(w http.ResponseWriter, tmpl string, p *Page) {
	err := templates.ExecuteTemplate(w, tmpl+".html", p)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
	}
}

func makeHandler(fn func(http.ResponseWriter, *http.Request, string)) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		m := validPath.FindStringSubmatch(r.URL.Path)
		if m == nil {
			http.NotFound(w, r)
			return
		}
		fn(w, r, m[2])
	}
}

func printCommand(cmd *exec.Cmd) {
  fmt.Printf("==> Executing: %s\n", strings.Join(cmd.Args, " "))
}

func printError(err error) {
  if err != nil {
    os.Stderr.WriteString(fmt.Sprintf("==> Error: %s\n", err.Error()))
  }
}

func printOutput(outs []byte) {
  if len(outs) > 0 {
    fmt.Printf("==> Output: %s\n", string(outs))
  }
}

func homeHandler(w http.ResponseWriter, r *http.Request) {

	containers := make(map[string]Container)

	/*
	Tener> -H unix:///var/run/docker.sock -H tcp://0.0.0.0:4243
	*/
	//command := "echo -e 'GET /containers/json HTTP/1.0' | nc -U /var/run/docker.sock | awk 'END{print}'"
	url := "http://" + host + ":4243/containers/json"

	response, err := http.Get(url)

	if err != nil {
		printError(err)
  }
	defer response.Body.Close()
	body, err := ioutil.ReadAll(response.Body)

  fmt.Println(string(body))

	// Handle errors
	if err != nil {
		panic("Can't run remote command: " + err.Error())
	} else {

		dec := json.NewDecoder(strings.NewReader(string(body)))

		// read open bracket
		_, err := dec.Token()
		if err != nil {
			log.Fatal(err)
		}

		// while the array contains values
		for dec.More() {
			var m Container

			// decode an array value (Message)
			err := dec.Decode(&m)
			if err != nil {
				log.Fatal(err)
			}

			containers[m.Id[0:10]] = m
		}
	}

	p := &Page{Containers: containers}

	renderTemplate(w, "index", p)
}

func main() {
	flag.Parse()
	http.HandleFunc("/", homeHandler)
	http.HandleFunc("/contact", contactHandlerNil)

	http.HandleFunc("/view/", makeHandler(viewHandler))
	http.HandleFunc("/contact/", makeHandler(contactHandler))
	http.Handle("/resources/", http.StripPrefix("/resources/", http.FileServer(http.Dir("../resources/web"))))

	if *addr {
		l, err := net.Listen("tcp", "127.0.0.1:0")
		if err != nil {
			log.Fatal(err)
		}
		err = ioutil.WriteFile("final-port.txt", []byte(l.Addr().String()), 0644)
		if err != nil {
			log.Fatal(err)
		}
		s := &http.Server{}
		s.Serve(l)
		return
	}

	http.ListenAndServe(":8080", nil)
}
