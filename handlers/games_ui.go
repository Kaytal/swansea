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

type GamesUI struct {
	store  *store.VideoGames
	tmpls  *template.Template
	covers *covers.Store
}

func NewGamesUI(s *store.VideoGames, assets fs.FS, cv *covers.Store) (*GamesUI, error) {
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
		"templates/partials/games/*.html",
	)
	if err != nil {
		return nil, err
	}
	return &GamesUI{store: s, tmpls: tmpls, covers: cv}, nil
}

func (h *GamesUI) Register(mux *http.ServeMux) {
	mux.HandleFunc("GET /games", h.page)
	mux.HandleFunc("GET /ui/games", h.games)
	mux.HandleFunc("GET /ui/games/add", h.addForm)
	mux.HandleFunc("POST /ui/games", h.create)
	mux.HandleFunc("GET /ui/games/{id}/edit", h.editForm)
	mux.HandleFunc("PUT /ui/games/{id}", h.update)
	mux.HandleFunc("DELETE /ui/games/{id}", h.deleteGame)
	mux.HandleFunc("GET /ui/games/search", h.search)
	mux.HandleFunc("GET /ui/games/filters", h.filters)
}

func (h *GamesUI) render(w http.ResponseWriter, name string, data any) {
	var buf bytes.Buffer
	if err := h.tmpls.ExecuteTemplate(&buf, name, data); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	buf.WriteTo(w)
}

func (h *GamesUI) page(w http.ResponseWriter, r *http.Request) {
	h.render(w, "index.html", indexData{
		InitialBooksURL:  "/ui/games",
		InitialFiltersURL: "/ui/games/filters",
		ActiveTab:        "games",
	})
}

type gamesPageData struct {
	Games       []*store.VideoGame
	Page        int
	TotalPages  int
	HasPrev     bool
	HasNext     bool
	FilterField string
	FilterValue string
	Query       string
	SearchField string
}

func (h *GamesUI) buildPage(page int, filterField, filterValue string) (gamesPageData, error) {
	if page < 1 {
		page = 1
	}
	offset := (page - 1) * store.PageSize

	var games []*store.VideoGame
	var total int
	var err error

	if filterField != "" && filterValue != "" {
		games, err = h.store.ListFiltered(filterField, filterValue, store.PageSize, offset)
		if err != nil {
			return gamesPageData{}, err
		}
		total, err = h.store.CountFiltered(filterField, filterValue)
	} else {
		games, err = h.store.ListPage(store.PageSize, offset)
		if err != nil {
			return gamesPageData{}, err
		}
		total, err = h.store.Count()
	}
	if err != nil {
		return gamesPageData{}, err
	}

	totalPages := (total + store.PageSize - 1) / store.PageSize
	if totalPages < 1 {
		totalPages = 1
	}
	return gamesPageData{
		Games:       games,
		Page:        page,
		TotalPages:  totalPages,
		HasPrev:     page > 1,
		HasNext:     page < totalPages,
		FilterField: filterField,
		FilterValue: filterValue,
	}, nil
}

func (h *GamesUI) games(w http.ResponseWriter, r *http.Request) {
	page, _ := strconv.Atoi(r.URL.Query().Get("page"))
	var filterField, filterValue string
	for _, f := range []string{"genre", "platform", "year"} {
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
	h.render(w, "games.html", data)
}

func (h *GamesUI) search(w http.ResponseWriter, r *http.Request) {
	q := strings.TrimSpace(r.URL.Query().Get("q"))
	if q == "" {
		h.games(w, r)
		return
	}
	field := r.URL.Query().Get("field")
	switch field {
	case "title", "developer":
		// valid
	default:
		field = ""
	}
	page, _ := strconv.Atoi(r.URL.Query().Get("page"))
	if page < 1 {
		page = 1
	}
	offset := (page - 1) * store.PageSize

	games, err := h.store.Search(q, field, store.PageSize, offset)
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
	h.render(w, "games.html", gamesPageData{
		Games:       games,
		Page:        page,
		TotalPages:  totalPages,
		HasPrev:     page > 1,
		HasNext:     page < totalPages,
		Query:       q,
		SearchField: field,
	})
}

func (h *GamesUI) filters(w http.ResponseWriter, r *http.Request) {
	fv, err := h.store.FilterValues()
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	h.render(w, "games_filters.html", fv)
}

func (h *GamesUI) addForm(w http.ResponseWriter, r *http.Request) {
	source := r.URL.Query().Get("source")
	query := r.URL.Query().Get("q")
	if source != "" && query != "" {
		result, err := lookup.GameFor(source)(query)
		if err == nil {
			h.render(w, "games_add_modal.html", result)
			return
		}
		log.Printf("game lookup %s %q: %v", source, query, err)
	}
	h.render(w, "games_add_modal.html", nil)
}

func (h *GamesUI) create(w http.ResponseWriter, r *http.Request) {
	in := parseGameForm(w, r)
	if in.Title == "" {
		http.Error(w, "title is required", http.StatusBadRequest)
		return
	}
	game, err := h.store.Create(in)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	if in.CoverURL != "" {
		go func() {
			key := strconv.FormatInt(game.ID, 10)
			local := h.covers.Download(in.CoverURL, "game-"+key)
			if local != in.CoverURL {
				if err := h.store.UpdateCoverURL(game.ID, local); err != nil {
					log.Printf("covers: update game %d: %v", game.ID, err)
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
	h.render(w, "games.html", data)
}

func (h *GamesUI) editForm(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil {
		http.Error(w, "invalid id", http.StatusBadRequest)
		return
	}
	game, err := h.store.Get(id)
	if errors.Is(err, store.ErrNotFound) {
		http.NotFound(w, r)
		return
	}
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	h.render(w, "games_edit_modal.html", game)
}

func (h *GamesUI) update(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil {
		http.Error(w, "invalid id", http.StatusBadRequest)
		return
	}
	in := parseGameForm(w, r)
	if in.Title == "" {
		http.Error(w, "title is required", http.StatusBadRequest)
		return
	}
	game, err := h.store.Update(id, in)
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
			key := strconv.FormatInt(game.ID, 10)
			local := h.covers.Download(in.CoverURL, "game-"+key)
			if local != in.CoverURL {
				if err := h.store.UpdateCoverURL(game.ID, local); err != nil {
					log.Printf("covers: update game %d: %v", game.ID, err)
				}
			}
		}()
	}
	w.Header().Set("HX-Trigger", "closeModal")
	h.render(w, "game_card.html", game)
}

func (h *GamesUI) deleteGame(w http.ResponseWriter, r *http.Request) {
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

func parseGameForm(w http.ResponseWriter, r *http.Request) store.VideoGameInput {
	r.Body = http.MaxBytesReader(w, r.Body, 1<<20)
	r.ParseForm()
	return store.VideoGameInput{
		Title:       r.FormValue("title"),
		Platform:    r.FormValue("platform"),
		Developers:  splitLines(r.FormValue("developers")),
		Publisher:   r.FormValue("publisher"),
		ReleaseDate: r.FormValue("release_date"),
		Description: r.FormValue("description"),
		Genres:      splitLines(r.FormValue("genres")),
		CoverURL:    r.FormValue("cover_url"),
		Rating:      r.FormValue("rating"),
	}
}
