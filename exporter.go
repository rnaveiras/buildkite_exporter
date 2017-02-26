package main

import (
	"sync"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/common/log"
	"gopkg.in/buildkite/go-buildkite.v2/buildkite"
)

type Exporter struct {
	mutex        sync.RWMutex
	client       *buildkite.Client
	duration     prometheus.Gauge
	error        prometheus.Gauge
	totalScrapes prometheus.Counter
	scrapeErrors *prometheus.CounterVec
	builds       *prometheus.GaugeVec
	agents       prometheus.Gauge
	up           prometheus.Gauge
}

func NewExporter(timeout time.Duration) (*Exporter, error) {
	config, err := buildkite.NewTokenConfig(*buildkiteToken, false)

	if err != nil {
		log.Fatalln("buildkite token failed: %s", err)
	}

	httpclient := config.Client()
	httpclient.Timeout = timeout

	client := buildkite.NewClient(httpclient)

	return &Exporter{
		client: client,
		duration: prometheus.NewGauge(prometheus.GaugeOpts{
			Namespace: namespace,
			Name:      "last_scrape_duration_seconds",
			Help:      "Duration of the last scrape of metrics from Buildkite.",
		}),
		totalScrapes: prometheus.NewCounter(prometheus.CounterOpts{
			Namespace: namespace,
			Name:      "scrapes_total",
			Help:      "Total number of times Buildkite was scraped for metrics.",
		}),
		scrapeErrors: prometheus.NewCounterVec(prometheus.CounterOpts{
			Namespace: namespace,
			Name:      "scrape_errors_total",
			Help:      "Total number of times an error occurred scraping Buildkite.",
		}, []string{"collector"}),
		error: prometheus.NewGauge(prometheus.GaugeOpts{
			Namespace: namespace,
			Name:      "last_scrape_error",
			Help:      "Whether the last scrape of metrics from Buildkite resulted in an error (1 for error, 0 for success).",
		}),
		up: prometheus.NewGauge(prometheus.GaugeOpts{
			Namespace: namespace,
			Name:      "up",
			Help:      "Whether the Buildkite is up.",
		}),
		builds: prometheus.NewGaugeVec(prometheus.GaugeOpts{
			Namespace: namespace,
			Name:      "builds",
			Help:      "Number of Buids",
		}, []string{"state"},
		),
		agents: prometheus.NewGauge(prometheus.GaugeOpts{
			Namespace: namespace,
			Name:      "agents",
			Help:      "Number of Agents",
		}),
	}, nil
}

// Describe implements prometheus.Collector.
func (e *Exporter) Describe(ch chan<- *prometheus.Desc) {
	ch <- e.duration.Desc()
	ch <- e.error.Desc()
	ch <- e.totalScrapes.Desc()
	ch <- e.up.Desc()
	ch <- e.agents.Desc()

	e.scrapeErrors.Describe(ch)
	e.builds.Describe(ch)
}

func (e *Exporter) Collect(ch chan<- prometheus.Metric) {
	e.mutex.Lock() // To protect metrics from concurrent collects.
	defer e.mutex.Unlock()

	// Reset GaguesVec
	e.builds.Reset()

	e.scrape()

	e.builds.Collect(ch)
	e.scrapeErrors.Collect(ch)

	ch <- e.agents
	ch <- e.duration
	ch <- e.totalScrapes
	ch <- e.error
	ch <- e.up
}

func (e *Exporter) scrape() {
	var err error
	e.totalScrapes.Inc()
	defer func(begun time.Time) {
		e.duration.Set(time.Since(begun).Seconds())
		if err == nil {
			e.error.Set(0)
		} else {
			e.error.Set(1)
		}
	}(time.Now())

	err = e.scrapeBuilds()
	e.scrapeAgents()
	e.up.Set(1)
}

func (e *Exporter) scrapeBuilds() error {
	states := []string{"running", "scheduled", "passed", "failed", "blocked", "canceled",
		"canceling", "skipped", "not_run", "finished"}

	for _, state := range states {
		e.builds.WithLabelValues(state).Set(0)
	}

	options := &buildkite.BuildsListOptions{}

	builds, response, err := e.client.Builds.ListByOrg(*buildkiteOrgName, options)
	if err != nil {
		e.scrapeErrors.WithLabelValues("builds").Inc()
		log.With("collector", "builds").Errorln(err)
		return err
	}
	defer response.Body.Close()

	for _, build := range builds {
		e.builds.With(prometheus.Labels{"state": *build.State}).Inc()
	}

	return nil
}

func (e *Exporter) scrapeAgents() error {
	options := &buildkite.AgentListOptions{}

	agents, response, err := e.client.Agents.List(*buildkiteOrgName, options)
	if err != nil {
		e.scrapeErrors.WithLabelValues("agents").Inc()
		log.With("collector", "agents").Errorln(err)
		return err
	}
	defer response.Body.Close()

	num_agents := len(agents)
	e.agents.Set(float64(num_agents))

	return nil
}
