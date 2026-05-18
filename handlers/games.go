package handlers

import (
	"errors"
	"net/http"

	"swansea/store"
)

type Games struct {
	store *store.VideoGames
}

func NewGames(s *store.VideoGames) *Games {
	return &Games{store: s}
}

func (h *Games) Register(mux *http.ServeMux) {
	mux.HandleFunc("GET /api/games", h.list)
	mux.HandleFunc("POST /api/games", h.create)
	mux.HandleFunc("GET /api/games/{id}", h.get)
	mux.HandleFunc("PUT /api/games/{id}", h.update)
	mux.HandleFunc("DELETE /api/games/{id}", h.delete)
}

func (h *Games) list(w http.ResponseWriter, r *http.Request) {
	games, err := h.store.List()
	if err != nil {
		writeError(w, http.StatusInternalServerError, err)
		return
	}
	if games == nil {
		games = []*store.VideoGame{}
	}
	writeJSON(w, http.StatusOK, games)
}

func (h *Games) get(w http.ResponseWriter, r *http.Request) {
	id, ok := pathID(w, r)
	if !ok {
		return
	}
	game, err := h.store.Get(id)
	if errors.Is(err, store.ErrNotFound) {
		writeError(w, http.StatusNotFound, err)
		return
	}
	if err != nil {
		writeError(w, http.StatusInternalServerError, err)
		return
	}
	writeJSON(w, http.StatusOK, game)
}

func (h *Games) create(w http.ResponseWriter, r *http.Request) {
	var in store.VideoGameInput
	if !decodeBody(w, r, &in) {
		return
	}
	if in.Title == "" {
		writeError(w, http.StatusBadRequest, errors.New("title is required"))
		return
	}
	game, err := h.store.Create(in)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err)
		return
	}
	writeJSON(w, http.StatusCreated, game)
}

func (h *Games) update(w http.ResponseWriter, r *http.Request) {
	id, ok := pathID(w, r)
	if !ok {
		return
	}
	var in store.VideoGameInput
	if !decodeBody(w, r, &in) {
		return
	}
	if in.Title == "" {
		writeError(w, http.StatusBadRequest, errors.New("title is required"))
		return
	}
	game, err := h.store.Update(id, in)
	if errors.Is(err, store.ErrNotFound) {
		writeError(w, http.StatusNotFound, err)
		return
	}
	if err != nil {
		writeError(w, http.StatusInternalServerError, err)
		return
	}
	writeJSON(w, http.StatusOK, game)
}

func (h *Games) delete(w http.ResponseWriter, r *http.Request) {
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
