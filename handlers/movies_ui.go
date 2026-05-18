package handlers

import (
	"bytes"
	"errors"
	"html/template"
	"io/fs"
	"log"
	"net/http"
	"net/url"
	"strconv"
	"strings"

	"swansea/covers"
	"swansea/lookup"
	"swansea/store"
)

type MoviesUI struct {
	store  *store.Movies
	tmpls  *template.Template
	covers *covers.Store
}

func NewMoviesUI(s *store.Movies, assets fs.FS, cv *covers.Store) (*MoviesUI, error) {
	funcMap := template.FuncMap{
		"join":      strings.Join,
		"joinLines": func(ss []string) string { return strings.Join(ss, "\n") },
		"year": func(date string) string {
			if len(date) >= 4 {
				return date[:4]
			}
			return date
		},
		"add":      func(a, b int) int { return a + b },
		"sub":      func(a, b int) int { return a - b },
		"urlquery": url.QueryEscape,
	}
	tmpls, err := template.New("").Funcs(funcMap).ParseFS(assets,
		"templates/index.html",
		"templates/partials/movies/*.html",
	)
	if err != nil {
		return nil, err
	}
	return &MoviesUI{store: s, tmpls: tmpls, covers: cv}, nil
}

func (h *MoviesUI) Register(mux *http.ServeMux) {
	mux.HandleFunc("GET /movies", h.page)
	mux.HandleFunc("GET /ui/movies", h.movies)
	mux.HandleFunc("GET /ui/movies/add", h.addForm)
	mux.HandleFunc("POST /ui/movies", h.create)
	mux.HandleFunc("GET /ui/movies/{id}/edit", h.editForm)
	mux.HandleFunc("PUT /ui/movies/{id}", h.update)
	mux.HandleFunc("DELETE /ui/movies/{id}", h.deleteMovie)
	mux.HandleFunc("GET /ui/movies/search", h.search)
	mux.HandleFunc("GET /ui/movies/filters", h.filters)
}

func (h *MoviesUI) render(w http.ResponseWriter, name string, data any) {
	var buf bytes.Buffer
	if err := h.tmpls.ExecuteTemplate(&buf, name, data); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	buf.WriteTo(w)
}

func (h *MoviesUI) page(w http.ResponseWriter, r *http.Request) {
	h.render(w, "index.html", indexData{
		InitialBooksURL:  "/ui/movies",
		InitialFiltersURL: "/ui/movies/filters",
		ActiveTab:        "movies",
	})
}

type moviesPageData struct {
	Movies      []*store.Movie
	Page        int
	TotalPages  int
	HasPrev     bool
	HasNext     bool
	FilterField string
	FilterValue string
	Query       string
	SearchField string
}

func (h *MoviesUI) buildPage(page int, filterField, filterValue string) (moviesPageData, error) {
	if page < 1 {
		page = 1
	}
	offset := (page - 1) * store.PageSize

	var movies []*store.Movie
	var total int
	var err error

	if filterField != "" && filterValue != "" {
		movies, err = h.store.ListFiltered(filterField, filterValue, store.PageSize, offset)
		if err != nil {
			return moviesPageData{}, err
		}
		total, err = h.store.CountFiltered(filterField, filterValue)
	} else {
		movies, err = h.store.ListPage(store.PageSize, offset)
		if err != nil {
			return moviesPageData{}, err
		}
		total, err = h.store.Count()
	}
	if err != nil {
		return moviesPageData{}, err
	}

	totalPages := (total + store.PageSize - 1) / store.PageSize
	if totalPages < 1 {
		totalPages = 1
	}
	return moviesPageData{
		Movies:      movies,
		Page:        page,
		TotalPages:  totalPages,
		HasPrev:     page > 1,
		HasNext:     page < totalPages,
		FilterField: filterField,
		FilterValue: filterValue,
	}, nil
}

func (h *MoviesUI) movies(w http.ResponseWriter, r *http.Request) {
	page, _ := strconv.Atoi(r.URL.Query().Get("page"))
	var filterField, filterValue string
	for _, f := range []string{"genre", "director", "year"} {
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
	h.render(w, "movies.html", data)
}

func (h *MoviesUI) search(w http.ResponseWriter, r *http.Request) {
	q := strings.TrimSpace(r.URL.Query().Get("q"))
	if q == "" {
		h.movies(w, r)
		return
	}
	field := r.URL.Query().Get("field")
	switch field {
	case "title", "director":
		// valid
	default:
		field = ""
	}
	page, _ := strconv.Atoi(r.URL.Query().Get("page"))
	if page < 1 {
		page = 1
	}
	offset := (page - 1) * store.PageSize

	movies, err := h.store.Search(q, field, store.PageSize, offset)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	total, err := h.store.SearchCount(q, field)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	totalPages := (total + store.PageSize - 1) / store.PageSize
	if totalPages < 1 {
		totalPages = 1
	}
	h.render(w, "movies.html", moviesPageData{
		Movies:      movies,
		Page:        page,
		TotalPages:  totalPages,
		HasPrev:     page > 1,
		HasNext:     page < totalPages,
		Query:       q,
		SearchField: field,
	})
}

func (h *MoviesUI) filters(w http.ResponseWriter, r *http.Request) {
	fv, err := h.store.FilterValues()
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	h.render(w, "movies_filters.html", fv)
}

func (h *MoviesUI) addForm(w http.ResponseWriter, r *http.Request) {
	source := r.URL.Query().Get("source")
	query := r.URL.Query().Get("q")
	if source != "" && query != "" {
		result, err := lookup.MovieFor(source)(query)
		if err == nil {
			h.render(w, "movies_add_modal.html", result)
			return
		}
		log.Printf("movie lookup %s %q: %v", source, query, err)
	}
	h.render(w, "movies_add_modal.html", nil)
}

func (h *MoviesUI) create(w http.ResponseWriter, r *http.Request) {
	in := parseMovieForm(w, r)
	if in.Title == "" {
		http.Error(w, "title is required", http.StatusBadRequest)
		return
	}
	movie, err := h.store.Create(in)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	if in.CoverURL != "" {
		go func() {
			key := strconv.FormatInt(movie.ID, 10)
			local := h.covers.Download(in.CoverURL, "movie-"+key)
			if local != in.CoverURL {
				if err := h.store.UpdateCoverURL(movie.ID, local); err != nil {
					log.Printf("covers: update movie %d: %v", movie.ID, err)
				}
			}
		}()
	}
	data, err := h.buildPage(1, "", "")
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	w.Header().Set("HX-Trigger", "closeModal")
	h.render(w, "movies.html", data)
}

func (h *MoviesUI) editForm(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil {
		http.Error(w, "invalid id", http.StatusBadRequest)
		return
	}
	movie, err := h.store.Get(id)
	if errors.Is(err, store.ErrNotFound) {
		http.NotFound(w, r)
		return
	}
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	h.render(w, "movies_edit_modal.html", movie)
}

func (h *MoviesUI) update(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil {
		http.Error(w, "invalid id", http.StatusBadRequest)
		return
	}
	in := parseMovieForm(w, r)
	if in.Title == "" {
		http.Error(w, "title is required", http.StatusBadRequest)
		return
	}
	movie, err := h.store.Update(id, in)
	if errors.Is(err, store.ErrNotFound) {
		http.NotFound(w, r)
		return
	}
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	if in.CoverURL != "" {
		go func() {
			key := strconv.FormatInt(movie.ID, 10)
			local := h.covers.Download(in.CoverURL, "movie-"+key)
			if local != in.CoverURL {
				if err := h.store.UpdateCoverURL(movie.ID, local); err != nil {
					log.Printf("covers: update movie %d: %v", movie.ID, err)
				}
			}
		}()
	}
	w.Header().Set("HX-Trigger", "closeModal")
	h.render(w, "movie_card.html", movie)
}

func (h *MoviesUI) deleteMovie(w http.ResponseWriter, r *http.Request) {
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

func parseMovieForm(w http.ResponseWriter, r *http.Request) store.MovieInput {
	r.Body = http.MaxBytesReader(w, r.Body, 1<<20)
	r.ParseForm()
	runtime, _ := strconv.Atoi(r.FormValue("runtime"))
	tmdbID, _ := strconv.ParseInt(r.FormValue("tmdb_id"), 10, 64)
	return store.MovieInput{
		Title:       r.FormValue("title"),
		Directors:   splitLines(r.FormValue("directors")),
		Studio:      r.FormValue("studio"),
		ReleaseDate: r.FormValue("release_date"),
		Description: r.FormValue("description"),
		Runtime:     runtime,
		Genres:      splitLines(r.FormValue("genres")),
		CoverURL:    r.FormValue("cover_url"),
		TmdbID:      tmdbID,
	}
}
