package handlers

import (
	"bytes"
	"errors"
	"html/template"
	"io/fs"
	"net/http"
	"strconv"
	"strings"

	"swansea/covers"
	"swansea/lookup"
	"swansea/store"
)

type UI struct {
	store        *store.Books
	tmpls        *template.Template
	covers       *covers.Store
	metadataPath string
}

func NewUI(s *store.Books, assets fs.FS, cv *covers.Store, metadataPath string) (*UI, error) {
	funcMap := template.FuncMap{
		"join":      strings.Join,
		"joinLines": func(ss []string) string { return strings.Join(ss, "\n") },
		"year": func(date string) string {
			if len(date) >= 4 {
				return date[:4]
			}
			return date
		},
		"add": func(a, b int) int { return a + b },
		"sub": func(a, b int) int { return a - b },
	}
	tmpls, err := template.New("").Funcs(funcMap).ParseFS(assets,
		"templates/index.html",
		"templates/partials/*.html",
	)
	if err != nil {
		return nil, err
	}
	return &UI{store: s, tmpls: tmpls, covers: cv, metadataPath: metadataPath}, nil
}

func (h *UI) Register(mux *http.ServeMux, assets fs.FS) {
	static, _ := fs.Sub(assets, "static")
	mux.Handle("GET /static/", http.StripPrefix("/static/", http.FileServerFS(static)))
	if h.metadataPath != "" {
		mux.Handle("GET /metadata/", http.StripPrefix("/metadata/", http.FileServer(http.Dir(h.metadataPath))))
	}

	mux.HandleFunc("GET /", h.index)
	mux.HandleFunc("GET /ui/books", h.books)
	mux.HandleFunc("GET /ui/books/add", h.addForm)
	mux.HandleFunc("POST /ui/books", h.create)
	mux.HandleFunc("GET /ui/books/{id}/edit", h.editForm)
	mux.HandleFunc("PUT /ui/books/{id}", h.update)
	mux.HandleFunc("DELETE /ui/books/{id}", h.delete)
	mux.HandleFunc("GET /ui/filters", h.filters)
	mux.HandleFunc("GET /ui/lookup/{isbn}", h.lookupPreview)
	mux.HandleFunc("POST /ui/books/isbn/{isbn}", h.addByISBN)
}

func (h *UI) maybeDownloadCover(in *store.BookInput, bookID int64) {
	key := in.ISBN
	if key == "" {
		key = strconv.FormatInt(bookID, 10)
	}
	in.CoverURL = h.covers.Download(in.CoverURL, key)
}

func (h *UI) render(w http.ResponseWriter, name string, data any) {
	var buf bytes.Buffer
	if err := h.tmpls.ExecuteTemplate(&buf, name, data); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	buf.WriteTo(w)
}

func (h *UI) index(w http.ResponseWriter, r *http.Request) {
	if r.URL.Path != "/" {
		http.NotFound(w, r)
		return
	}
	h.render(w, "index.html", nil)
}

type booksPageData struct {
	Books       []*store.Book
	Page        int
	TotalPages  int
	HasPrev     bool
	HasNext     bool
	FilterField string
	FilterValue string
}

func (h *UI) buildPage(page int, filterField, filterValue string) (booksPageData, error) {
	if page < 1 {
		page = 1
	}
	offset := (page - 1) * store.PageSize

	var books []*store.Book
	var total int
	var err error

	if filterField != "" && filterValue != "" {
		books, err = h.store.ListFiltered(filterField, filterValue, store.PageSize, offset)
		if err != nil {
			return booksPageData{}, err
		}
		total, err = h.store.CountFiltered(filterField, filterValue)
	} else {
		books, err = h.store.ListPage(store.PageSize, offset)
		if err != nil {
			return booksPageData{}, err
		}
		total, err = h.store.Count()
	}
	if err != nil {
		return booksPageData{}, err
	}

	totalPages := (total + store.PageSize - 1) / store.PageSize
	if totalPages < 1 {
		totalPages = 1
	}
	return booksPageData{
		Books:       books,
		Page:        page,
		TotalPages:  totalPages,
		HasPrev:     page > 1,
		HasNext:     page < totalPages,
		FilterField: filterField,
		FilterValue: filterValue,
	}, nil
}

func (h *UI) booksPage(page int) (booksPageData, error) {
	return h.buildPage(page, "", "")
}

func (h *UI) books(w http.ResponseWriter, r *http.Request) {
	page, _ := strconv.Atoi(r.URL.Query().Get("page"))
	var filterField, filterValue string
	for _, f := range []string{"category", "author", "year"} {
		if v := r.URL.Query().Get(f); v != "" {
			filterField = f
			filterValue = v
			break
		}
	}
	data, err := h.buildPage(page, filterField, filterValue)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	h.render(w, "books.html", data)
}

func (h *UI) filters(w http.ResponseWriter, r *http.Request) {
	fv, err := h.store.FilterValues()
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	h.render(w, "filters.html", fv)
}

func (h *UI) addForm(w http.ResponseWriter, r *http.Request) {
	h.render(w, "add_modal.html", nil)
}

func (h *UI) create(w http.ResponseWriter, r *http.Request) {
	in := parseBookForm(r)
	book, err := h.store.Create(in)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	h.maybeDownloadCover(&in, book.ID)
	if in.CoverURL != book.CoverURL {
		if updated, err := h.store.Update(book.ID, in); err == nil {
			_ = updated
		}
	}
	data, err := h.booksPage(1)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	w.Header().Set("HX-Trigger", "closeModal")
	h.render(w, "books.html", data)
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
	h.maybeDownloadCover(&in, id)
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

type scanPreviewData struct {
	*store.BookInput
	Error  string
	Source string
}

func (h *UI) lookupPreview(w http.ResponseWriter, r *http.Request) {
	isbn := r.PathValue("isbn")
	source := r.URL.Query().Get("source")
	result, err := lookup.For(source)(isbn)
	if err != nil {
		h.render(w, "scan_preview.html", scanPreviewData{Error: err.Error(), Source: source})
		return
	}
	h.render(w, "scan_preview.html", scanPreviewData{BookInput: result, Source: source})
}

func (h *UI) addByISBN(w http.ResponseWriter, r *http.Request) {
	isbn := r.PathValue("isbn")
	in, err := lookup.For(r.URL.Query().Get("source"))(isbn)
	if err != nil {
		http.Error(w, err.Error(), http.StatusNotFound)
		return
	}
	h.maybeDownloadCover(in, 0) // key will be ISBN from in.ISBN
	if _, err := h.store.Create(*in); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	data, err := h.booksPage(1)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	w.Header().Set("HX-Trigger", "closeScanner")
	h.render(w, "books.html", data)
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
