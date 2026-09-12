// Command faker is a long-running fake Starr queue for manual Unpackerr runs.
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
	app := flag.String("app", starrfake.AppSonarr, "sonarr, radarr, lidarr, or readarr")
	flag.Parse()

	appName := strings.ToLower(*app)
	switch appName {
	case starrfake.AppSonarr, starrfake.AppRadarr, starrfake.AppLidarr, starrfake.AppReadarr:
	default:
		fmt.Fprintf(os.Stderr, "unknown -app %q\n", *app)
		os.Exit(2)
	}

	fake := starrfake.New(appName, *key)
	log.Printf("fake %s queue on http://%s/api/%s/queue (X-Api-Key %s)",
		appName, *listen, fake.APIVersion(), *key)
	log.Printf("mutators: POST /debug/add  POST /debug/complete/{id}  POST /debug/drop/{id}")

	if err := fake.ListenAndServe(*listen); err != nil {
		log.Fatal(err)
	}
}
