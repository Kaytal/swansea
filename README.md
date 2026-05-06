# Swansea Library

A personal book library server. Add books manually, by typing an ISBN, or by scanning a barcode with a USB scanner or your phone's camera. Metadata is pulled from the Google Books API.

## Running with Docker

```bash
docker compose up --build
```

The app is available at **http://localhost:8099**. The SQLite database is stored in a named Docker volume (`sqlite_data`) and persists across container rebuilds.

### Environment variables

| Variable | Default | Description |
|---|---|---|
| `PORT` | `8080` | Port the server listens on inside the container |
| `DB_PATH` | `/data/swansea.db` | Path to the SQLite database file |

### Running locally (without Docker)

```bash
DB_PATH=/tmp/swansea.db go run .
# Server available at http://localhost:8080
```

---

## HTTP Endpoints

### UI (browser)

| Method | Path | Description |
|---|---|---|
| `GET` | `/` | Main library page |

The UI uses HTMX and Alpine.js. All interactions (add, edit, delete, scan) happen via partial page swaps — no full page reloads.

---

### JSON API

All request and response bodies are JSON. Successful responses use the status codes listed; errors return `{"error": "..."}`.

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

Replaces all fields. Send the full book payload.

**Delete a book**
```
DELETE /books/{id}
→ 204
→ 404 if not found
```

#### ISBN Lookup

**Preview metadata from Google Books (no save)**
```
GET /lookup/{isbn}
→ 200 BookInput
→ 404 if ISBN not found
```

Strips hyphens automatically. Returns the same shape as the `POST /books` request body so the response can be forwarded directly.

**Look up by ISBN and save**
```
POST /books/isbn/{isbn}
→ 201 Book
→ 404 if ISBN not found on Google Books
```

Fetches metadata from Google Books and creates the book in one step.

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
  "cover_url": "https://...",
  "categories": ["Software Engineering"],
  "description": "...",
  "created_at": "2026-05-05T18:00:00Z",
  "updated_at": "2026-05-05T18:00:00Z"
}
```

---

## Docker details

```yaml
# docker-compose.yml (summary)
services:
  server:
    build: .
    ports:
      - "8099:8080"   # external:internal
    volumes:
      - sqlite_data:/data

volumes:
  sqlite_data:
```

The image uses a two-stage build: a `golang:1.24-alpine` builder stage compiles a static binary, which is then copied into a minimal `alpine:3.21` runtime image.

To wipe the database and start fresh:
```bash
docker compose down -v   # removes the sqlite_data volume
docker compose up --build
```
