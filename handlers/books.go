package handlers

import (
	"encoding/json"
	"errors"
	"net/http"
	"strconv"

	"swansea/lookup"
	"swansea/store"
)

type Books struct {
	store *store.Books
}

func NewBooks(s *store.Books) *Books {
	return &Books{store: s}
}

func (h *Books) Register(mux *http.ServeMux) {
	mux.HandleFunc("GET /books", h.list)
	mux.HandleFunc("POST /books", h.create)
	mux.HandleFunc("GET /books/{id}", h.get)
	mux.HandleFunc("PUT /books/{id}", h.update)
	mux.HandleFunc("DELETE /books/{id}", h.delete)
	mux.HandleFunc("GET /lookup/{isbn}", h.lookupISBN)
	mux.HandleFunc("POST /books/isbn/{isbn}", h.addByISBN)
}

func (h *Books) list(w http.ResponseWriter, r *http.Request) {
	books, err := h.store.List()
	if err != nil {
		writeError(w, http.StatusInternalServerError, err)
		return
	}
	if books == nil {
		books = []*store.Book{}
	}
	writeJSON(w, http.StatusOK, books)
}

func (h *Books) get(w http.ResponseWriter, r *http.Request) {
	id, ok := pathID(w, r)
	if !ok {
		return
	}
	book, err := h.store.Get(id)
	if errors.Is(err, store.ErrNotFound) {
		writeError(w, http.StatusNotFound, err)
		return
	}
	if err != nil {
		writeError(w, http.StatusInternalServerError, err)
		return
	}
	writeJSON(w, http.StatusOK, book)
}

func (h *Books) create(w http.ResponseWriter, r *http.Request) {
	var in store.BookInput
	if !decodeBody(w, r, &in) {
		return
	}
	if in.Title == "" {
		writeError(w, http.StatusBadRequest, errors.New("title is required"))
		return
	}
	book, err := h.store.Create(in)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err)
		return
	}
	writeJSON(w, http.StatusCreated, book)
}

func (h *Books) update(w http.ResponseWriter, r *http.Request) {
	id, ok := pathID(w, r)
	if !ok {
		return
	}
	var in store.BookInput
	if !decodeBody(w, r, &in) {
		return
	}
	if in.Title == "" {
		writeError(w, http.StatusBadRequest, errors.New("title is required"))
		return
	}
	book, err := h.store.Update(id, in)
	if errors.Is(err, store.ErrNotFound) {
		writeError(w, http.StatusNotFound, err)
		return
	}
	if err != nil {
		writeError(w, http.StatusInternalServerError, err)
		return
	}
	writeJSON(w, http.StatusOK, book)
}

func (h *Books) delete(w http.ResponseWriter, r *http.Request) {
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

func (h *Books) lookupISBN(w http.ResponseWriter, r *http.Request) {
	isbn := r.PathValue("isbn")
	result, err := lookup.For(r.URL.Query().Get("source"))(isbn)
	if errors.Is(err, lookup.ErrNotFound) {
		writeError(w, http.StatusNotFound, err)
		return
	}
	if err != nil {
		writeError(w, http.StatusBadGateway, err)
		return
	}
	writeJSON(w, http.StatusOK, result)
}

func (h *Books) addByISBN(w http.ResponseWriter, r *http.Request) {
	isbn := r.PathValue("isbn")
	in, err := lookup.For(r.URL.Query().Get("source"))(isbn)
	if errors.Is(err, lookup.ErrNotFound) {
		writeError(w, http.StatusNotFound, err)
		return
	}
	if err != nil {
		writeError(w, http.StatusBadGateway, err)
		return
	}
	book, err := h.store.Create(*in)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err)
		return
	}
	writeJSON(w, http.StatusCreated, book)
}

func pathID(w http.ResponseWriter, r *http.Request) (int64, bool) {
	id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil {
		writeError(w, http.StatusBadRequest, errors.New("invalid id"))
		return 0, false
	}
	return id, true
}

func decodeBody(w http.ResponseWriter, r *http.Request, v any) bool {
	r.Body = http.MaxBytesReader(w, r.Body, 1<<20)
	if err := json.NewDecoder(r.Body).Decode(v); err != nil {
		writeError(w, http.StatusBadRequest, err)
		return false
	}
	return true
}

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(v)
}

func writeError(w http.ResponseWriter, status int, err error) {
	writeJSON(w, status, map[string]string{"error": err.Error()})
}
