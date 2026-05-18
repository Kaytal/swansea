# Swansea Library

A personal media library server for Books, Video Games, Movies, and Music. Add items manually, by typing metadata, or (for books) by scanning a barcode. Metadata is fetched from external sources. Cover images are cached locally.

## Features

- **Four media types** — Books, Video Games, Movies, and Music, each with its own data model, filters, and metadata source
- **ISBN scanning** — USB barcode scanners and camera-based scanning (Chrome 88+, Safari 17+) for books
- **Metadata lookup** — Google Books / Open Library (books), ScreenScraper.fr (games), TMDB (movies), gnudb.org / CDDB (music)
- **Local cover caching** — cover images are downloaded and served from the host, not fetched remotely on every load
- **Pagination** — 25 items per page per type
- **Browse by facet** — filter by category, author, year, platform, genre, director, artist, and more via the sidebar
- **Fuzzy search** — full-text search powered by SQLite FTS5 for all four types

## Running with Docker

```bash
cp .env.example .env
# Edit .env and add your API keys
docker compose up --build
```

The app is available at **http://localhost:8099**.

## Environment variables

| Variable | Default | Description |
|---|---|---|
| `PORT` | `8080` | Port the server listens on inside the container |
| `DB_PATH` | `/data/swansea.db` | Path to the SQLite database file |
| `METADATA_PATH` | `/metadata` | Root directory for cached cover images |
| `GOOGLE_BOOKS_API_KEY` | — | Google Books API key (required to avoid 429s on book lookups) |
| `SCREENSCRAPER_USERNAME` | — | ScreenScraper.fr account username (video game metadata) |
| `SCREENSCRAPER_PASSWORD` | — | ScreenScraper.fr account password (video game metadata) |
| `TMDB_API_KEY` | — | The Movie Database API key (movie metadata) |

Copy `.env.example` to `.env` and fill in your keys. Both Docker Compose (via `env_file`) and local dev pick it up automatically.

## Running locally (without Docker)

```bash
DB_PATH=/tmp/swansea.db METADATA_PATH=/tmp/swansea-meta \
  GOOGLE_BOOKS_API_KEY=... TMDB_API_KEY=... \
  SCREENSCRAPER_USERNAME=... SCREENSCRAPER_PASSWORD=... \
  go run .
# Server available at http://localhost:8080
```

## Data persistence

In Docker, the SQLite database is bind-mounted from `./db/` on the host, and cover images from `./metadata/`. Both survive container rebuilds. To wipe everything and start fresh:

```bash
docker compose down
rm -rf ./db ./metadata
docker compose up --build
```

---

## HTTP Endpoints

### UI (browser)

| Method | Path | Description |
|---|---|---|
| `GET` | `/` | Main library page (Books tab) |
| `GET` | `/games` | Main library page (Video Games tab) |
| `GET` | `/movies` | Main library page (Movies tab) |
| `GET` | `/music` | Main library page (Music tab) |
| `GET` | `/category/{value}` | Bookmarkable filtered view by book category |
| `GET` | `/author/{value}` | Bookmarkable filtered view by book author |
| `GET` | `/year/{value}` | Bookmarkable filtered view by book year |
| `GET` | `/ui/books?page=N&category=X&author=X&year=YYYY` | Book grid fragment |
| `GET` | `/ui/search?q=X&field=title\|author` | Book full-text search |
| `GET` | `/ui/filters` | Book filter sidebar |
| `GET` | `/ui/games?page=N&platform=X&genre=X&year=YYYY` | Games grid fragment |
| `GET` | `/ui/games/search?q=X` | Games full-text search |
| `GET` | `/ui/games/filters` | Games filter sidebar |
| `GET` | `/ui/movies?page=N&genre=X&director=X&year=YYYY` | Movies grid fragment |
| `GET` | `/ui/movies/search?q=X` | Movies full-text search |
| `GET` | `/ui/movies/filters` | Movies filter sidebar |
| `GET` | `/ui/music?page=N&genre=X&artist=X&year=YYYY` | Music grid fragment |
| `GET` | `/ui/music/search?q=X` | Music full-text search |
| `GET` | `/ui/music/filters` | Music filter sidebar |

The UI uses HTMX and Alpine.js. All interactions happen via partial page swaps — no full page reloads. The active tab is persisted in `localStorage`.

---

### JSON API

All request and response bodies are JSON.

#### Books

**List / filter books**
```
GET /api/books?page=N&category=X&author=X&year=YYYY&q=X
→ 200 { items: [Book, ...], total: N }
```

**Get a book**
```
GET /api/books/{id}
→ 200 Book
→ 404 if not found
```

**Create a book**
```
POST /api/books
Content-Type: application/json

{
  "title": "The Pragmatic Programmer",
  "authors": ["David Thomas", "Andrew Hunt"],
  "isbn": "9780135957059",
  "publisher": "Addison-Wesley",
  "published_date": "2019",
  "page_count": 352,
  "cover_url": "https://...",
  "categories": ["Software Engineering"],
  "description": "..."
}

→ 201 Book
```

`title` is the only required field.

**Update a book**
```
PUT /api/books/{id}
Content-Type: application/json
{ ...same shape as POST... }
→ 200 Book
→ 404 if not found
```

**Delete a book**
```
DELETE /api/books/{id}
→ 204
→ 404 if not found
```

**ISBN lookup (preview, no save)**
```
GET /lookup/{isbn}?source=google|openlibrary
→ 200 BookInput
→ 404 if not found
```

**ISBN lookup and save**
```
POST /api/books/isbn/{isbn}?source=google|openlibrary
→ 201 Book
→ 404 if not found
```

`source` defaults to `google`. Both endpoints strip hyphens automatically.

---

#### Video Games

**List / filter games**
```
GET /api/games?page=N&platform=X&genre=X&year=YYYY&q=X
→ 200 { items: [VideoGame, ...], total: N }
```

**Get a game**
```
GET /api/games/{id}
→ 200 VideoGame
→ 404 if not found
```

**Create a game**
```
POST /api/games
Content-Type: application/json

{
  "title": "The Legend of Zelda: Breath of the Wild",
  "platform": "Nintendo Switch",
  "developers": ["Nintendo EPD"],
  "genres": ["Action-Adventure"],
  "release_year": 2017,
  "cover_url": "https://...",
  "description": "..."
}

→ 201 VideoGame
```

`title` is the only required field.

**Update a game**
```
PUT /api/games/{id}
Content-Type: application/json
{ ...same shape as POST... }
→ 200 VideoGame
→ 404 if not found
```

**Delete a game**
```
DELETE /api/games/{id}
→ 204
→ 404 if not found
```

---

#### Movies

**List / filter movies**
```
GET /api/movies?page=N&genre=X&director=X&year=YYYY&q=X
→ 200 { items: [Movie, ...], total: N }
```

**Get a movie**
```
GET /api/movies/{id}
→ 200 Movie
→ 404 if not found
```

**Create a movie**
```
POST /api/movies
Content-Type: application/json

{
  "title": "Dune",
  "directors": ["Denis Villeneuve"],
  "genres": ["Sci-Fi"],
  "release_year": 2021,
  "runtime": 155,
  "cover_url": "https://...",
  "description": "..."
}

→ 201 Movie
```

`title` is the only required field.

**Update a movie**
```
PUT /api/movies/{id}
Content-Type: application/json
{ ...same shape as POST... }
→ 200 Movie
→ 404 if not found
```

**Delete a movie**
```
DELETE /api/movies/{id}
→ 204
→ 404 if not found
```

---

#### Music

**List / filter albums**
```
GET /api/music?page=N&genre=X&artist=X&year=YYYY&q=X
→ 200 { items: [MusicAlbum, ...], total: N }
```

**Get an album**
```
GET /api/music/{id}
→ 200 MusicAlbum
→ 404 if not found
```

**Create an album**
```
POST /api/music
Content-Type: application/json

{
  "title": "OK Computer",
  "artists": ["Radiohead"],
  "genres": ["Alternative Rock"],
  "release_year": 1997,
  "track_count": 12,
  "cover_url": "https://...",
  "description": "..."
}

→ 201 MusicAlbum
```

`title` is the only required field.

**Update an album**
```
PUT /api/music/{id}
Content-Type: application/json
{ ...same shape as POST... }
→ 200 MusicAlbum
→ 404 if not found
```

**Delete an album**
```
DELETE /api/music/{id}
→ 204
→ 404 if not found
```

---

## Response shapes

### Book

```jsonc
{
  "id": 1,
  "isbn": "9780135957059",
  "title": "The Pragmatic Programmer",
  "authors": ["David Thomas", "Andrew Hunt"],
  "publisher": "Addison-Wesley",
  "published_date": "2019",
  "page_count": 352,
  "cover_url": "/metadata/covers/9780135957059.jpg",
  "categories": ["Software Engineering"],
  "description": "...",
  "created_at": "2026-05-05T18:00:00Z",
  "updated_at": "2026-05-05T18:00:00Z"
}
```

### VideoGame

```jsonc
{
  "id": 1,
  "title": "The Legend of Zelda: Breath of the Wild",
  "platform": "Nintendo Switch",
  "developers": ["Nintendo EPD"],
  "genres": ["Action-Adventure"],
  "release_year": 2017,
  "cover_url": "/metadata/covers/zelda-botw.jpg",
  "description": "...",
  "created_at": "2026-05-05T18:00:00Z",
  "updated_at": "2026-05-05T18:00:00Z"
}
```

### Movie

```jsonc
{
  "id": 1,
  "title": "Dune",
  "directors": ["Denis Villeneuve"],
  "genres": ["Sci-Fi"],
  "release_year": 2021,
  "runtime": 155,
  "cover_url": "/metadata/covers/dune-2021.jpg",
  "description": "...",
  "created_at": "2026-05-05T18:00:00Z",
  "updated_at": "2026-05-05T18:00:00Z"
}
```

### MusicAlbum

```jsonc
{
  "id": 1,
  "title": "OK Computer",
  "artists": ["Radiohead"],
  "genres": ["Alternative Rock"],
  "release_year": 1997,
  "track_count": 12,
  "cover_url": "/metadata/covers/ok-computer.jpg",
  "description": "...",
  "created_at": "2026-05-05T18:00:00Z",
  "updated_at": "2026-05-05T18:00:00Z"
}
```
