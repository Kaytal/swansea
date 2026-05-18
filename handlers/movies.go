package handlers

import (
	"errors"
	"net/http"

	"swansea/store"
)

type Movies struct {
	store *store.Movies
}

func NewMovies(s *store.Movies) *Movies {
	return &Movies{store: s}
}

func (h *Movies) Register(mux *http.ServeMux) {
	mux.HandleFunc("GET /api/movies", h.list)
	mux.HandleFunc("POST /api/movies", h.create)
	mux.HandleFunc("GET /api/movies/{id}", h.get)
	mux.HandleFunc("PUT /api/movies/{id}", h.update)
	mux.HandleFunc("DELETE /api/movies/{id}", h.delete)
}

func (h *Movies) list(w http.ResponseWriter, r *http.Request) {
	movies, err := h.store.List()
	if err != nil {
		writeError(w, http.StatusInternalServerError, err)
		return
	}
	if movies == nil {
		movies = []*store.Movie{}
	}
	writeJSON(w, http.StatusOK, movies)
}

func (h *Movies) get(w http.ResponseWriter, r *http.Request) {
	id, ok := pathID(w, r)
	if !ok {
		return
	}
	movie, err := h.store.Get(id)
	if errors.Is(err, store.ErrNotFound) {
		writeError(w, http.StatusNotFound, err)
		return
	}
	if err != nil {
		writeError(w, http.StatusInternalServerError, err)
		return
	}
	writeJSON(w, http.StatusOK, movie)
}

func (h *Movies) create(w http.ResponseWriter, r *http.Request) {
	var in store.MovieInput
	if !decodeBody(w, r, &in) {
		return
	}
	if in.Title == "" {
		writeError(w, http.StatusBadRequest, errors.New("title is required"))
		return
	}
	movie, err := h.store.Create(in)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err)
		return
	}
	writeJSON(w, http.StatusCreated, movie)
}

func (h *Movies) update(w http.ResponseWriter, r *http.Request) {
	id, ok := pathID(w, r)
	if !ok {
		return
	}
	var in store.MovieInput
	if !decodeBody(w, r, &in) {
		return
	}
	if in.Title == "" {
		writeError(w, http.StatusBadRequest, errors.New("title is required"))
		return
	}
	movie, err := h.store.Update(id, in)
	if errors.Is(err, store.ErrNotFound) {
		writeError(w, http.StatusNotFound, err)
		return
	}
	if err != nil {
		writeError(w, http.StatusInternalServerError, err)
		return
	}
	writeJSON(w, http.StatusOK, movie)
}

func (h *Movies) delete(w http.ResponseWriter, r *http.Request) {
	id, ok := pathID(w, r)
	if !ok {
		return
	}
	if err := h.store.Delete(id); errors.Is(err, store.ErrNotFound) {
		writeError(w, http.StatusNotFound, err)
		return
	} else if err != nil {
		writeError(w, http.StatusInternalServerError, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}
