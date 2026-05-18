package lookup

import "swansea/store"

// MusicFor returns the lookup function for the given source name.
// Only gnudb is supported; source is reserved for future additional sources.
func MusicFor(source string) func(string) (*store.MusicAlbumInput, error) {
	return GnudbByArtistTitle
}
