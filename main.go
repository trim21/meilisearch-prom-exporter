package main

import (
	"fmt"
	"log"
	"net/http"
	"os"

	"github.com/meilisearch/meilisearch-go"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"github.com/sirupsen/logrus"
	"github.com/spf13/pflag"
	"golang.org/x/sync/errgroup"
)

const taskEnqueue = "enqueued"
const taskFinished = "succeeded"
const taskProcessing = "processing"

var addr = pflag.String("listen.address", ":8080", "The address to listen on for HTTP requests.")
var meilisearchURL = pflag.String("meili.url", "http://127.0.0.1:7700", "The address to listen on for HTTP requests.")
var meilisearchKey = pflag.String("meili.key", "", "The address to listen on for HTTP requests.")
var meilisearchIndex = pflag.String("meili.index", "subjects", "The address to listen on for HTTP requests.")

func main() {
	if err := start(); err != nil {
		fmt.Printf("%+v\n", err)
		os.Exit(1)
	}
}

func start() error {
	pflag.Parse()

	client := meilisearch.NewClient(meilisearch.ClientConfig{
		Host:   *meilisearchURL,
		APIKey: *meilisearchKey,
	})

	exp := &exporter{client: client}

	prometheus.DefaultRegisterer.MustRegister(exp)
	// prometheus.MustRegister(taskProcessDelay)

	http.Handle("/metrics", promhttp.Handler())

	log.Fatal(http.ListenAndServe(*addr, nil))
	return nil
}

type exporter struct {
	client *meilisearch.Client
}

func (e *exporter) Describe(descs chan<- *prometheus.Desc) {}

func (e *exporter) Collect(ch chan<- prometheus.Metric) {
	enqueued, processing, done, err := e.fetchTasks()
	if err != nil {
		logrus.Error("failed to fetch current task", err)
	}

	e.processTasks(ch, enqueued, processing, done)
}

var taskProcessDelay = prometheus.NewGauge(prometheus.GaugeOpts{
	Namespace: "meilisearch",
	Name:      "task_delay_seconds",
	Help:      "meilisearch processing task delay",
})

var taskUnfnishedCount = prometheus.NewGauge(prometheus.GaugeOpts{
	Namespace: "meilisearch",
	Name:      "task_unfinished_count",
	Help:      "meilisearch enqueued tasks",
})

func (e *exporter) processTasks(ch chan<- prometheus.Metric, enqueued *meilisearch.Task, processing *meilisearch.Task, done *meilisearch.Task) {
	if processing != nil {
		d := processing.StartedAt.Sub(processing.EnqueuedAt)
		taskProcessDelay.Set(d.Seconds())
	} else {
		if enqueued == nil {
			taskProcessDelay.Set(0)
		}
	}

	if enqueued != nil && done != nil {
		taskUnfnishedCount.Set(float64(enqueued.UID - done.UID))
		ch <- taskUnfnishedCount
	}

	ch <- taskProcessDelay
}

func (e *exporter) fetchTasks() (enqueued *meilisearch.Task, processing *meilisearch.Task, done *meilisearch.Task, err error) {
	var g = errgroup.Group{}

	g.Go(func() error {
		tasks, err := e.client.Index(*meilisearchIndex).GetTasks(&meilisearch.TasksQuery{
			Limit:  1,
			Status: []string{taskProcessing},
		})
		if err != nil {
			return err
		}

		if len(tasks.Results) != 0 {
			top := tasks.Results[0]
			processing = &top
		}

		return nil
	})

	g.Go(func() error {
		tasks, err := e.client.Index(*meilisearchIndex).GetTasks(&meilisearch.TasksQuery{
			Limit:  1,
			Status: []string{taskEnqueue},
		})
		if err != nil {
			return err
		}

		if len(tasks.Results) != 0 {
			top := tasks.Results[0]
			enqueued = &top
		}

		return nil
	})

	g.Go(func() error {
		tasks, err := e.client.Index(*meilisearchIndex).GetTasks(&meilisearch.TasksQuery{
			Limit:  1,
			Status: []string{taskFinished},
		})
		if err != nil {
			return err
		}

		if len(tasks.Results) != 0 {
			top := tasks.Results[0]
			done = &top
		}

		return nil
	})

	err = g.Wait()

	return
}
