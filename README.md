# Swansea Library

A personal book library server. Add books manually, by typing an ISBN, or by scanning a barcode with a USB scanner or your phone's camera. Metadata is fetched from Google Books or Open Library. Cover images are cached locally.

## Features

- **ISBN scanning** — USB barcode scanners and camera-based scanning (Chrome 88+, Safari 17+)
- **Multiple metadata sources** — Google Books and Open Library; selectable per scan
- **Local cover caching** — cover images are downloaded and served from the host, not fetched remotely on every load
- **Pagination** — 25 books per page
- **Browse by facet** — filter by category, author, or publication year via the sidebar; each filter has a bookmarkable URL (`/category/Fiction`, `/author/Hemingway`, `/year/1952`)
- **Fuzzy search** — type-ahead title and author search powered by SQLite FTS5

## Running with Docker

```bash
cp .env.example .env
# Edit .env and add your GOOGLE_BOOKS_API_KEY
docker compose up --build
```

The app is available at **http://localhost:8099**.

## Environment variables

| Variable | Default | Description |
|---|---|---|
| `PORT` | `8080` | Port the server listens on inside the container |
| `DB_PATH` | `/data/swansea.db` | Path to the SQLite database file |
| `METADATA_PATH` | `/metadata` | Root directory for cached cover images |
| `GOOGLE_BOOKS_API_KEY` | — | Google Books API key (required to avoid 429s) |

Copy `.env.example` to `.env` and fill in your key. Both Docker Compose (via `env_file`) and local dev pick it up automatically.

## Running locally (without Docker)

```bash
DB_PATH=/tmp/swansea.db METADATA_PATH=/tmp/swansea-meta go run .
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
| `GET` | `/` | Main library page |
| `GET` | `/category/{value}` | Bookmarkable filtered view by category |
| `GET` | `/author/{value}` | Bookmarkable filtered view by author |
| `GET` | `/year/{value}` | Bookmarkable filtered view by year |
| `GET` | `/ui/books?page=N` | Book grid fragment (page N, 25 per page) |
| `GET` | `/ui/books?category=X` | Filtered book grid fragment |
| `GET` | `/ui/books?author=X` | Filtered book grid fragment |
| `GET` | `/ui/books?year=YYYY` | Filtered book grid fragment |
| `GET` | `/ui/search?q=X&field=title\|author` | Full-text search |
| `GET` | `/ui/filters` | Sidebar with distinct facet values |

The UI uses HTMX and Alpine.js. All interactions happen via partial page swaps — no full page reloads.

---

### JSON API

All request and response bodies are JSON.

#### Books

**List all books**
```
GET /books
→ 200 [ Book, ... ]
```

**Get a book**
```
GET /books/{id}
→ 200 Book
→ 404 if not found
```

**Create a book manually**
```
POST /books
Content-Type: application/json

{
  "title": "The Pragmatic Programmer",
  "authors": ["David Thomas", "Andrew Hunt"],
  "isbn": "9780135957059",
  "publisher": "Addison-Wesley",
  "published_date": "2019-09-13",
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
PUT /books/{id}
Content-Type: application/json

{ ...same shape as POST... }

→ 200 Book
→ 404 if not found
```

**Delete a book**
```
DELETE /books/{id}
→ 204
→ 404 if not found
```

#### ISBN Lookup

**Preview metadata (no save)**
```
GET /lookup/{isbn}?source=google|openlibrary
→ 200 BookInput
→ 404 if not found
```

**Look up by ISBN and save**
```
POST /books/isbn/{isbn}?source=google|openlibrary
→ 201 Book
→ 404 if not found
```

`source` defaults to `google`. Both endpoints strip hyphens from the ISBN automatically.

---

### Book object

```jsonc
{
  "id": 1,
  "isbn": "9780135957059",
  "title": "The Pragmatic Programmer",
  "authors": ["David Thomas", "Andrew Hunt"],
  "publisher": "Addison-Wesley",
  "published_date": "2019-09-13",
  "page_count": 352,
  "cover_url": "/metadata/covers/9780135957059.jpg",
  "categories": ["Software Engineering"],
  "description": "...",
  "created_at": "2026-05-05T18:00:00Z",
  "updated_at": "2026-05-05T18:00:00Z"
}
```
