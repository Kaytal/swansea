package lookup

import "swansea/store"

// MovieFor returns the lookup function for the given source name.
// Only TMDB is supported; source is reserved for future additional sources.
func MovieFor(source string) func(string) (*store.MovieInput, error) {
	return TMDBByTitle
}
