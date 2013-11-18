package server

import (
	"facette/backend"
	"facette/library"
	"github.com/gorilla/mux"
	"html/template"
	"log"
	"net/http"
	"os"
	"path"
	"strings"
)

func (server *Server) browseHandleCollection(writer http.ResponseWriter, request *http.Request,
	tmpl *template.Template) error {
	type collectionData struct {
		*library.Collection
		Parent string
	}

	var (
		data struct {
			Collection *collectionData
			Request    *http.Request
		}
		err  error
		item interface{}
	)

	// Set template data
	data.Collection = &collectionData{Collection: &library.Collection{}}

	data.Collection.ID = mux.Vars(request)["collection"]

	if item, err = server.Library.GetItem(data.Collection.ID,
		library.LibraryItemCollection); err != nil {
		return err
	}

	data.Collection.Collection = item.(*library.Collection)

	if request.FormValue("q") != "" {
		data.Collection.Collection = server.Library.FilterCollection(data.Collection.Collection, request.FormValue("q"))
	}

	if data.Collection.Collection.Parent != nil {
		data.Collection.Parent = data.Collection.Collection.Parent.ID
	} else {
		data.Collection.Parent = "null"
	}

	// Execute template
	if tmpl, err = tmpl.ParseFiles(
		path.Join(server.Config.BaseDir, "share", "html", "layout.html"),
		path.Join(server.Config.BaseDir, "share", "html", "common", "element.html"),
		path.Join(server.Config.BaseDir, "share", "html", "common", "graph.html"),
		path.Join(server.Config.BaseDir, "share", "html", "browse", "layout.html"),
		path.Join(server.Config.BaseDir, "share", "html", "browse", "collection.html"),
	); err != nil {
		return err
	}

	data.Request = request

	return tmpl.Execute(writer, data)
}

func (server *Server) browseHandleIndex(writer http.ResponseWriter, request *http.Request,
	tmpl *template.Template) error {
	var (
		err error
	)

	// Execute template
	if tmpl, err = tmpl.ParseFiles(
		path.Join(server.Config.BaseDir, "share", "html", "layout.html"),
		path.Join(server.Config.BaseDir, "share", "html", "common", "element.html"),
		path.Join(server.Config.BaseDir, "share", "html", "browse", "layout.html"),
		path.Join(server.Config.BaseDir, "share", "html", "browse", "index.html"),
	); err != nil {
		return err
	}

	return tmpl.Execute(writer, nil)
}

func (server *Server) browseHandleSearch(writer http.ResponseWriter, request *http.Request) {
	var (
		chunks []string
		data   struct {
			Count       int
			Request     *http.Request
			Sources     []*backend.Source
			Collections []*library.Collection
		}
		err  error
		tmpl *template.Template
	)

	if request.Method != "GET" && request.Method != "HEAD" {
		server.handleResponse(writer, http.StatusMethodNotAllowed)
		return
	}

	// Perform search
	if request.FormValue("q") != "" {
		for _, chunk := range strings.Split(strings.ToLower(request.FormValue("q")), " ") {
			chunks = append(chunks, strings.Trim(chunk, " \t"))
		}

		for _, origin := range server.Catalog.Origins {
			for _, source := range origin.Sources {
				for _, chunk := range chunks {
					if strings.Index(strings.ToLower(source.Name), chunk) == -1 {
						goto nextOrigin
					}
				}

				data.Sources = append(data.Sources, source)
			nextOrigin:
			}
		}

		for _, collection := range server.Library.Collections {
			for _, chunk := range chunks {
				if strings.Index(strings.ToLower(collection.Name), chunk) == -1 {
					goto nextCollection
				}
			}

			data.Collections = append(data.Collections, collection)
		nextCollection:
		}
	}

	data.Count = len(data.Sources) + len(data.Collections)

	data.Request = request

	// Execute template
	if tmpl, err = template.New("layout.html").Funcs(template.FuncMap{
		"eq":   templateEqual,
		"ne":   templateNotEqual,
		"dump": templateDumpMap,
		"hl":   templateHighlight,
	}).ParseFiles(
		path.Join(server.Config.BaseDir, "share", "html", "layout.html"),
		path.Join(server.Config.BaseDir, "share", "html", "common", "element.html"),
		path.Join(server.Config.BaseDir, "share", "html", "browse", "layout.html"),
		path.Join(server.Config.BaseDir, "share", "html", "browse", "search.html"),
	); err == nil {
		err = tmpl.Execute(writer, data)
	}

	if err != nil {
		if os.IsNotExist(err) {
			log.Println("ERROR: " + err.Error())
			server.handleResponse(writer, http.StatusNotFound)
		} else {
			log.Println("ERROR: " + err.Error())
			server.handleResponse(writer, http.StatusInternalServerError)
		}
	}
}

func (server *Server) browseHandleSource(writer http.ResponseWriter, request *http.Request,
	tmpl *template.Template) error {
	var (
		data struct {
			Collection *library.Collection
			Request    *http.Request
		}
		err        error
		sourceName string
	)

	sourceName = mux.Vars(request)["source"]

	if data.Collection, err = server.Library.GetCollectionTemplate(sourceName); err != nil {
		return err
	}

	if request.FormValue("q") != "" {
		data.Collection = server.Library.FilterCollection(data.Collection, request.FormValue("q"))
	}

	// Execute template
	if tmpl, err = tmpl.ParseFiles(
		path.Join(server.Config.BaseDir, "share", "html", "layout.html"),
		path.Join(server.Config.BaseDir, "share", "html", "common", "element.html"),
		path.Join(server.Config.BaseDir, "share", "html", "common", "graph.html"),
		path.Join(server.Config.BaseDir, "share", "html", "browse", "layout.html"),
		path.Join(server.Config.BaseDir, "share", "html", "browse", "collection.html"),
	); err != nil {
		return err
	}

	data.Request = request

	return tmpl.Execute(writer, data)
}

func (server *Server) browseHandle(writer http.ResponseWriter, request *http.Request) {
	var (
		err  error
		tmpl *template.Template
	)

	// Redirect to browse
	if request.URL.Path == "/" {
		http.Redirect(writer, request, URLBrowsePath+"/", 301)
		return
	}

	// Return template data
	tmpl = template.New("layout.html").Funcs(template.FuncMap{
		"eq":   templateEqual,
		"ne":   templateNotEqual,
		"dump": templateDumpMap,
	})

	// Execute template
	if mux.Vars(request)["source"] != "" {
		err = server.browseHandleSource(writer, request, tmpl)
	} else if mux.Vars(request)["collection"] != "" {
		err = server.browseHandleCollection(writer, request, tmpl)
	} else {
		err = server.browseHandleIndex(writer, request, tmpl)
	}

	if err != nil {
		if os.IsNotExist(err) {
			log.Println("ERROR: " + err.Error())
			server.handleResponse(writer, http.StatusNotFound)
		} else {
			log.Println("ERROR: " + err.Error())
			server.handleResponse(writer, http.StatusInternalServerError)
		}
	}
}
