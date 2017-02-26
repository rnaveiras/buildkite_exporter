package main

import (
	"flag"
	"fmt"
	"net/http"
	"os"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/common/log"
	"github.com/prometheus/common/version"
)

var (
	showVersion      = flag.Bool("version", false, "Print version information.")
	buildkiteToken   = flag.String("buildkite.token", os.Getenv("BUILDKITE_TOKEN"), "Buildkite API token [BUILDKITE_TOKEN]")
	buildkiteOrgName = flag.String("buildkite.orgname", os.Getenv("BUILDKITE_ORGNAME"), "Buildkite Organization Name [BUILDKITE_ORGNAME]")
	buildkiteTimeout = flag.Duration("buildkite.timeout", 5*time.Second, "Timeout on HTTP requests to Buildkite API")
	listenAddress    = flag.String("web.listen-address", ":9209", "Address on which to expose metrics and web interface.")
	metricsPath      = flag.String("web.telemetry-path", "/metrics", "Path under which to expose metrics.")
)

const (
	namespace = "buildkite"
	exporter  = "exporter"
)

var landingPage = []byte(`<html>
<head><title>Buildkite exporter</title></head>
<body>
<h1>Buildkite exporter</h1>
<p><a href='` + *metricsPath + `'>Metrics</a></p>
</body>
</html>
`)

func main() {
	flag.Parse()

	if *showVersion {
		fmt.Fprintln(os.Stdout, version.Print("buildkite_exporter"))
		os.Exit(0)
	}

	exporter, err := NewExporter(*buildkiteTimeout)
	if err != nil {
		log.Fatal(err)
	}

	log.Infoln("Starting buildkite_exporter", version.Info())
	log.Infoln("Build context", version.BuildContext())

	prometheus.MustRegister(exporter)
	prometheus.MustRegister(version.NewCollector("buildkite"))

	http.Handle(*metricsPath, prometheus.Handler())
	http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		w.Write(landingPage)
	})

	log.Infoln("Listening on", *listenAddress)
	if err := http.ListenAndServe(*listenAddress, nil); err != nil {
		log.Fatalf("Error starting HTTP server: %s", err)
	}
}
