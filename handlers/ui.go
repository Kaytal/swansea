package handlers

import (
	"errors"
	"html/template"
	"io/fs"
	"net/http"
	"strconv"
	"strings"

	"swansea/lookup"
	"swansea/store"
)

type UI struct {
	store *store.Books
	tmpls *template.Template
}

func NewUI(s *store.Books, assets fs.FS) (*UI, error) {
	funcMap := template.FuncMap{
		"join":      strings.Join,
		"joinLines": func(ss []string) string { return strings.Join(ss, "\n") },
		"year": func(date string) string {
			if len(date) >= 4 {
				return date[:4]
			}
			return date
		},
	}
	tmpls, err := template.New("").Funcs(funcMap).ParseFS(assets,
		"templates/index.html",
		"templates/partials/*.html",
	)
	if err != nil {
		return nil, err
	}
	return &UI{store: s, tmpls: tmpls}, nil
}

func (h *UI) Register(mux *http.ServeMux, assets fs.FS) {
	static, _ := fs.Sub(assets, "static")
	mux.Handle("GET /static/", http.StripPrefix("/static/", http.FileServerFS(static)))

	mux.HandleFunc("GET /", h.index)
	mux.HandleFunc("GET /ui/books", h.books)
	mux.HandleFunc("GET /ui/books/add", h.addForm)
	mux.HandleFunc("POST /ui/books", h.create)
	mux.HandleFunc("GET /ui/books/{id}/edit", h.editForm)
	mux.HandleFunc("PUT /ui/books/{id}", h.update)
	mux.HandleFunc("DELETE /ui/books/{id}", h.delete)
	mux.HandleFunc("GET /ui/lookup/{isbn}", h.lookupPreview)
	mux.HandleFunc("POST /ui/books/isbn/{isbn}", h.addByISBN)
}

func (h *UI) render(w http.ResponseWriter, name string, data any) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	if err := h.tmpls.ExecuteTemplate(w, name, data); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
	}
}

func (h *UI) index(w http.ResponseWriter, r *http.Request) {
	if r.URL.Path != "/" {
		http.NotFound(w, r)
		return
	}
	h.render(w, "index.html", nil)
}

func (h *UI) books(w http.ResponseWriter, r *http.Request) {
	books, err := h.store.List()
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	if books == nil {
		books = []*store.Book{}
	}
	h.render(w, "books.html", books)
}

func (h *UI) addForm(w http.ResponseWriter, r *http.Request) {
	h.render(w, "add_modal.html", nil)
}

func (h *UI) create(w http.ResponseWriter, r *http.Request) {
	in := parseBookForm(r)
	if _, err := h.store.Create(in); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	books, err := h.store.List()
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	w.Header().Set("HX-Trigger", "closeModal")
	h.render(w, "books.html", books)
}

func (h *UI) editForm(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil {
		http.Error(w, "invalid id", http.StatusBadRequest)
		return
	}
	book, err := h.store.Get(id)
	if errors.Is(err, store.ErrNotFound) {
		http.NotFound(w, r)
		return
	}
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	h.render(w, "edit_modal.html", book)
}

func (h *UI) update(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil {
		http.Error(w, "invalid id", http.StatusBadRequest)
		return
	}
	in := parseBookForm(r)
	book, err := h.store.Update(id, in)
	if errors.Is(err, store.ErrNotFound) {
		http.NotFound(w, r)
		return
	}
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	w.Header().Set("HX-Trigger", "closeModal")
	h.render(w, "book_card.html", book)
}

func (h *UI) delete(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil {
		http.Error(w, "invalid id", http.StatusBadRequest)
		return
	}
	if err := h.store.Delete(id); err != nil && !errors.Is(err, store.ErrNotFound) {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	w.WriteHeader(http.StatusOK)
}

func (h *UI) lookupPreview(w http.ResponseWriter, r *http.Request) {
	isbn := r.PathValue("isbn")
	result, err := lookup.ByISBN(isbn)
	if err != nil {
		h.render(w, "scan_preview.html", map[string]string{"Error": err.Error()})
		return
	}
	h.render(w, "scan_preview.html", result)
}

func (h *UI) addByISBN(w http.ResponseWriter, r *http.Request) {
	isbn := r.PathValue("isbn")
	in, err := lookup.ByISBN(isbn)
	if err != nil {
		http.Error(w, err.Error(), http.StatusNotFound)
		return
	}
	if _, err := h.store.Create(*in); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	books, err := h.store.List()
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	w.Header().Set("HX-Trigger", "closeScanner")
	h.render(w, "books.html", books)
}

func parseBookForm(r *http.Request) store.BookInput {
	r.ParseForm()
	pageCount, _ := strconv.Atoi(r.FormValue("page_count"))
	return store.BookInput{
		ISBN:          r.FormValue("isbn"),
		Title:         r.FormValue("title"),
		Authors:       splitLines(r.FormValue("authors")),
		Publisher:     r.FormValue("publisher"),
		PublishedDate: r.FormValue("published_date"),
		Description:   r.FormValue("description"),
		PageCount:     pageCount,
		CoverURL:      r.FormValue("cover_url"),
		Categories:    splitLines(r.FormValue("categories")),
	}
}

func splitLines(s string) []string {
	var out []string
	for _, line := range strings.Split(s, "\n") {
		if line = strings.TrimSpace(line); line != "" {
			out = append(out, line)
		}
	}
	return out
}
