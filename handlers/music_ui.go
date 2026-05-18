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

type MusicUI struct {
	store  *store.MusicAlbums
	tmpls  *template.Template
	covers *covers.Store
}

func NewMusicUI(s *store.MusicAlbums, assets fs.FS, cv *covers.Store) (*MusicUI, error) {
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
		"templates/partials/music/*.html",
	)
	if err != nil {
		return nil, err
	}
	return &MusicUI{store: s, tmpls: tmpls, covers: cv}, nil
}

func (h *MusicUI) Register(mux *http.ServeMux) {
	mux.HandleFunc("GET /music", h.page)
	mux.HandleFunc("GET /ui/music", h.music)
	mux.HandleFunc("GET /ui/music/add", h.addForm)
	mux.HandleFunc("POST /ui/music", h.create)
	mux.HandleFunc("GET /ui/music/{id}/edit", h.editForm)
	mux.HandleFunc("PUT /ui/music/{id}", h.update)
	mux.HandleFunc("DELETE /ui/music/{id}", h.deleteAlbum)
	mux.HandleFunc("GET /ui/music/search", h.search)
	mux.HandleFunc("GET /ui/music/filters", h.filters)
}

func (h *MusicUI) render(w http.ResponseWriter, name string, data any) {
	var buf bytes.Buffer
	if err := h.tmpls.ExecuteTemplate(&buf, name, data); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	buf.WriteTo(w)
}

func (h *MusicUI) page(w http.ResponseWriter, r *http.Request) {
	h.render(w, "index.html", indexData{
		InitialBooksURL:  "/ui/music",
		InitialFiltersURL: "/ui/music/filters",
		ActiveTab:        "music",
	})
}

type musicPageData struct {
	Albums      []*store.MusicAlbum
	Page        int
	TotalPages  int
	HasPrev     bool
	HasNext     bool
	FilterField string
	FilterValue string
	Query       string
	SearchField string
}

func (h *MusicUI) buildPage(page int, filterField, filterValue string) (musicPageData, error) {
	if page < 1 {
		page = 1
	}
	offset := (page - 1) * store.PageSize

	var albums []*store.MusicAlbum
	var total int
	var err error

	if filterField != "" && filterValue != "" {
		albums, err = h.store.ListFiltered(filterField, filterValue, store.PageSize, offset)
		if err != nil {
			return musicPageData{}, err
		}
		total, err = h.store.CountFiltered(filterField, filterValue)
	} else {
		albums, err = h.store.ListPage(store.PageSize, offset)
		if err != nil {
			return musicPageData{}, err
		}
		total, err = h.store.Count()
	}
	if err != nil {
		return musicPageData{}, err
	}

	totalPages := (total + store.PageSize - 1) / store.PageSize
	if totalPages < 1 {
		totalPages = 1
	}
	return musicPageData{
		Albums:      albums,
		Page:        page,
		TotalPages:  totalPages,
		HasPrev:     page > 1,
		HasNext:     page < totalPages,
		FilterField: filterField,
		FilterValue: filterValue,
	}, nil
}

func (h *MusicUI) music(w http.ResponseWriter, r *http.Request) {
	page, _ := strconv.Atoi(r.URL.Query().Get("page"))
	var filterField, filterValue string
	for _, f := range []string{"genre", "artist", "year"} {
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
	h.render(w, "music.html", data)
}

func (h *MusicUI) search(w http.ResponseWriter, r *http.Request) {
	q := strings.TrimSpace(r.URL.Query().Get("q"))
	if q == "" {
		h.music(w, r)
		return
	}
	field := r.URL.Query().Get("field")
	switch field {
	case "title", "artist":
		// valid
	default:
		field = ""
	}
	page, _ := strconv.Atoi(r.URL.Query().Get("page"))
	if page < 1 {
		page = 1
	}
	offset := (page - 1) * store.PageSize

	albums, err := h.store.Search(q, field, store.PageSize, offset)
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
	h.render(w, "music.html", musicPageData{
		Albums:      albums,
		Page:        page,
		TotalPages:  totalPages,
		HasPrev:     page > 1,
		HasNext:     page < totalPages,
		Query:       q,
		SearchField: field,
	})
}

func (h *MusicUI) filters(w http.ResponseWriter, r *http.Request) {
	fv, err := h.store.FilterValues()
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	h.render(w, "music_filters.html", fv)
}

func (h *MusicUI) addForm(w http.ResponseWriter, r *http.Request) {
	source := r.URL.Query().Get("source")
	query := r.URL.Query().Get("q")
	if source != "" && query != "" {
		result, err := lookup.MusicFor(source)(query)
		if err == nil {
			h.render(w, "music_add_modal.html", result)
			return
		}
		log.Printf("music lookup %s %q: %v", source, query, err)
	}
	h.render(w, "music_add_modal.html", nil)
}

func (h *MusicUI) create(w http.ResponseWriter, r *http.Request) {
	in := parseMusicForm(w, r)
	if in.Title == "" {
		http.Error(w, "title is required", http.StatusBadRequest)
		return
	}
	album, err := h.store.Create(in)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	if in.CoverURL != "" {
		go func() {
			key := strconv.FormatInt(album.ID, 10)
			local := h.covers.Download(in.CoverURL, "music-"+key)
			if local != in.CoverURL {
				if err := h.store.UpdateCoverURL(album.ID, local); err != nil {
					log.Printf("covers: update album %d: %v", album.ID, err)
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
	h.render(w, "music.html", data)
}

func (h *MusicUI) editForm(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil {
		http.Error(w, "invalid id", http.StatusBadRequest)
		return
	}
	album, err := h.store.Get(id)
	if errors.Is(err, store.ErrNotFound) {
		http.NotFound(w, r)
		return
	}
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	h.render(w, "music_edit_modal.html", album)
}

func (h *MusicUI) update(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil {
		http.Error(w, "invalid id", http.StatusBadRequest)
		return
	}
	in := parseMusicForm(w, r)
	if in.Title == "" {
		http.Error(w, "title is required", http.StatusBadRequest)
		return
	}
	album, err := h.store.Update(id, in)
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
			key := strconv.FormatInt(album.ID, 10)
			local := h.covers.Download(in.CoverURL, "music-"+key)
			if local != in.CoverURL {
				if err := h.store.UpdateCoverURL(album.ID, local); err != nil {
					log.Printf("covers: update album %d: %v", album.ID, err)
				}
			}
		}()
	}
	w.Header().Set("HX-Trigger", "closeModal")
	h.render(w, "music_card.html", album)
}

func (h *MusicUI) deleteAlbum(w http.ResponseWriter, r *http.Request) {
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

func parseMusicForm(w http.ResponseWriter, r *http.Request) store.MusicAlbumInput {
	r.Body = http.MaxBytesReader(w, r.Body, 1<<20)
	r.ParseForm()
	trackCount, _ := strconv.Atoi(r.FormValue("track_count"))
	return store.MusicAlbumInput{
		Title:         r.FormValue("title"),
		Artists:       splitLines(r.FormValue("artists")),
		Label:         r.FormValue("label"),
		ReleaseDate:   r.FormValue("release_date"),
		Description:   r.FormValue("description"),
		Genres:        splitLines(r.FormValue("genres")),
		TrackCount:    trackCount,
		CoverURL:      r.FormValue("cover_url"),
		CatalogNumber: r.FormValue("catalog_number"),
	}
}
