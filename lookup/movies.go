package lookup

import "swansea/store"

// MovieFor returns the lookup function for the given source name.
// Unknown sources default to TMDB.
func MovieFor(source string) func(string) (*store.MovieInput, error) {
	return TMDBByTitle
}
