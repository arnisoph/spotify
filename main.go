package main

import (
	"flag"
	"fmt"
	"log"
	"net/http"
	"os"

	"github.com/zmb3/spotify"
)

//var userID = flag.String("user", "", "the Spotify user ID to look up")
var playlistID = flag.String("playlist", "", "Playlist ID to crawl for artists")

const redirectURI = "http://localhost:8080/callback"

var (
	auth  = spotify.NewAuthenticator(redirectURI, spotify.ScopeUserFollowModify)
	ch    = make(chan *spotify.Client)
	state = "abc123"
)

func AppendArtistIfMissing(artists []spotify.ID, newArtists []spotify.SimpleArtist) []spotify.ID {

	// In the beginning, there was nothing.
	if len(artists) == 0 {

		for _, newArtist := range newArtists {
			artists = append(artists, newArtist.ID)
		}

	} else {
		var duplicate bool
		for _, newArtist := range newArtists {
			for _, artist := range artists {
				if artist == newArtist.ID {
					duplicate = true
					break
				}
			}

			if !duplicate {
				for _, newArtist := range newArtists {
					artists = append(artists, newArtist.ID)
				}
			}
		}
	}

	return artists
}

func main() {
	flag.Parse()

	//if *userID == "" {
	//	fmt.Fprintf(os.Stderr, "Error: missing user ID\n")
	//	flag.Usage()
	//	return
	//}

	if *playlistID == "" {
		fmt.Fprintf(os.Stderr, "Error: missing playlist ID\n")
		flag.Usage()
		return
	}

	var client *spotify.Client

	http.HandleFunc("/callback", completeAuth)

	http.HandleFunc("/follow/", func(w http.ResponseWriter, r *http.Request) {
		if err := followArtists(client); err != nil {
			log.Fatal(err)
		}
	})

	http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		log.Println("Got request for:", r.URL.String())
	})

	go func() {
		url := auth.AuthURL(state)
		fmt.Println("Please log in to Spotify by visiting the following page in your browser:\n", url)

		// wait for auth to complete
		client = <-ch

		// use the client to make calls that require authorization
		user, err := client.CurrentUser()
		if err != nil {
			log.Fatal(err)
		}
		fmt.Println("You are logged in as:", user.ID)
	}()

	http.ListenAndServe(":8080", nil)
}

func completeAuth(w http.ResponseWriter, r *http.Request) {
	//config := &clientcredentials.Config{
	//	ClientID:     os.Getenv("SPOTIFY_ID"),
	//	ClientSecret: os.Getenv("SPOTIFY_SECRET"),
	//	TokenURL:     spotify.TokenURL,
	//}
	//token, err := config.Token(context.Background())
	//if err != nil {
	//	log.Fatalf("couldn't get token: %v", err)
	//}
	//client := spotify.Authenticator{}.NewClient(token)

	tok, err := auth.Token(state, r)
	if err != nil {
		http.Error(w, "Couldn't get token", http.StatusForbidden)
		log.Fatal(err)
	}
	if st := r.FormValue("state"); st != state {
		http.NotFound(w, r)
		log.Fatalf("State mismatch: %s != %s\n", st, state)
	}
	// use the token to get an authenticated client
	client := auth.NewClient(tok)
	w.Header().Set("Content-Type", "text/html")
	fmt.Fprintf(w, "Login Completed!")
	ch <- &client
}

func followArtists(client *spotify.Client) error {

	playlist, err := client.GetPlaylistTracks(spotify.ID(*playlistID))
	if err != nil {
		return err
	}

	var artists []spotify.ID
	//log.Printf("Playlist has %d total tracks", playlist.Total)
	for page := 1; ; page++ {
		//log.Printf("  Page %d has %d tracks", page, len(playlist.Tracks))

		for _, track := range playlist.Tracks {
			artists = AppendArtistIfMissing(artists, track.Track.Artists)
		}

		err = client.NextPage(playlist)
		if err == spotify.ErrNoMorePages {
			break
		}
		if err != nil {
			return err
		}
	}

	log.Printf("Nice, found %d total artists!", len(artists))
	const batchSize = 40
	for page := 0; ; page++ {
		log.Printf("Follow artists of page %d", page)

		start := page * batchSize
		end := start + batchSize

		if end > len(artists) {
			end = len(artists)
		}

		//fmt.Println(start, end, artists[start:end])
		if err := client.FollowArtist(artists[start:end]...); err != nil {
			return err
		}

		if end == len(artists) {
			break
		}
	}

	log.Println("Done")
	return nil
}
