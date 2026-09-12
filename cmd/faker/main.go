// Command faker is a long-running fake Starr queue for manual Unpackerr runs.
// One listener serves /sonarr /radarr /lidarr /readarr (API v3 or v1 under each).
package main

import (
	"flag"
	"fmt"
	"log"
	"os"
	"strings"

	"github.com/Unpackerr/unpackerr-inttest/internal/starrfake"
)

func main() {
	listen := flag.String("listen", "127.0.0.1:8989", "listen address")
	key := flag.String("key", "unpackerr-inttest-starr-key-32ch", "X-Api-Key Unpackerr must send")
	flag.Parse()

	hub := starrfake.NewHub(*key)
	bases := make([]string, 0, 4)

	for _, app := range starrfake.Apps() {
		bases = append(bases, "/"+app)
	}

	log.Printf("fake Starr apps on http://%s%s (X-Api-Key %s)", *listen, strings.Join(bases, ","), *key)
	log.Printf("unpackerr urls: http://%s/sonarr  http://%s/radarr  http://%s/lidarr  http://%s/readarr",
		*listen, *listen, *listen, *listen)
	log.Printf("GET / for index; mutators: POST /{app}/debug/add  complete/{id}  drop/{id}")

	if err := hub.ListenAndServe(*listen); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
