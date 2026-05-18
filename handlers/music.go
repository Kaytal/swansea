package handlers

import (
	"errors"
	"net/http"

	"swansea/store"
)

type Music struct {
	store *store.MusicAlbums
}

func NewMusic(s *store.MusicAlbums) *Music {
	return &Music{store: s}
}

func (h *Music) Register(mux *http.ServeMux) {
	mux.HandleFunc("GET /api/music", h.list)
	mux.HandleFunc("POST /api/music", h.create)
	mux.HandleFunc("GET /api/music/{id}", h.get)
	mux.HandleFunc("PUT /api/music/{id}", h.update)
	mux.HandleFunc("DELETE /api/music/{id}", h.delete)
}

func (h *Music) list(w http.ResponseWriter, r *http.Request) {
	albums, err := h.store.List()
	if err != nil {
		writeError(w, http.StatusInternalServerError, err)
		return
	}
	if albums == nil {
		albums = []*store.MusicAlbum{}
	}
	writeJSON(w, http.StatusOK, albums)
}

func (h *Music) get(w http.ResponseWriter, r *http.Request) {
	id, ok := pathID(w, r)
	if !ok {
		return
	}
	album, err := h.store.Get(id)
	if errors.Is(err, store.ErrNotFound) {
		writeError(w, http.StatusNotFound, err)
		return
	}
	if err != nil {
		writeError(w, http.StatusInternalServerError, err)
		return
	}
	writeJSON(w, http.StatusOK, album)
}

func (h *Music) create(w http.ResponseWriter, r *http.Request) {
	var in store.MusicAlbumInput
	if !decodeBody(w, r, &in) {
		return
	}
	if in.Title == "" {
		writeError(w, http.StatusBadRequest, errors.New("title is required"))
		return
	}
	album, err := h.store.Create(in)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err)
		return
	}
	writeJSON(w, http.StatusCreated, album)
}

func (h *Music) update(w http.ResponseWriter, r *http.Request) {
	id, ok := pathID(w, r)
	if !ok {
		return
	}
	var in store.MusicAlbumInput
	if !decodeBody(w, r, &in) {
		return
	}
	if in.Title == "" {
		writeError(w, http.StatusBadRequest, errors.New("title is required"))
		return
	}
	album, err := h.store.Update(id, in)
	if errors.Is(err, store.ErrNotFound) {
		writeError(w, http.StatusNotFound, err)
		return
	}
	if err != nil {
		writeError(w, http.StatusInternalServerError, err)
		return
	}
	writeJSON(w, http.StatusOK, album)
}

func (h *Music) delete(w http.ResponseWriter, r *http.Request) {
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
