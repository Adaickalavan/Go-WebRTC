package handler

import (
	"bytes"
	"encoding/json"
	"errors"
	"html/template"
	"log"
	"net/http"
)

//RespondWithError is a HTTP reply with error message
func RespondWithError(w http.ResponseWriter, code int, msg string) {
	RespondWithJSON(w, code, map[string]string{"error": msg})
}

//RespondWithJSON is a HTTP reply with JSON
func RespondWithJSON(w http.ResponseWriter, code int, payload interface{}) {
	response, err := json.MarshalIndent(payload, "", " ")
	if err != nil {
		http.Error(w, "HTTP 500: Internal Server Error", http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(code)
	w.Write(response)
}

// Push the given resource to the client.
func Push(w http.ResponseWriter, resource string) {
	err := errors.New("Push is not supported")
	pusher, ok := w.(http.Pusher)
	if ok {
		if err = pusher.Push(resource, nil); err == nil {
			return
		}
	}
	log.Printf("Push warning: %v\n", err)
}

// Render a template, or server error.
func Render(w http.ResponseWriter, r *http.Request, tpl *template.Template, data interface{}) {
	buf := new(bytes.Buffer)
	if err := tpl.Execute(buf, data); err != nil {
		log.Printf("\nRender error: %v\n", err)
		RespondWithError(w, http.StatusInternalServerError, "ERROR: Template render error.")
		return
	}
	w.Write(buf.Bytes())
}
