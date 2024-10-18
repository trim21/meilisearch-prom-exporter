package main

import (
	"errors"
	"fmt"
	"log"
	"net/http"
	"os"
	"time"

	"github.com/meilisearch/meilisearch-go"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"github.com/sirupsen/logrus"
	"github.com/spf13/pflag"
	"golang.org/x/sync/errgroup"
)

var flag = pflag.FlagSet{}

var addr = flag.String("listen.address", ":8080", "The address to listen on for HTTP requests.")
var meilisearchURL = flag.String("meili.url", "http://127.0.0.1:7700", "The url of meilisearch")
var meilisearchKey = flag.String("meili.key", "", "The api key for meilisearch.")
var meilisearchIndex = flag.String("meili.index", "", "The which index's info to export.")
var meilisearchTimeout = flag.Duration("meili.timeout", 5*time.Second, "meilisearch requests timeout")

func main() {
	if err := start(); err != nil {
		fmt.Printf("%+v\n", err)
		os.Exit(1)
	}
}

func start() error {
	flag.Init(os.Args[0], pflag.ContinueOnError)
	if err := flag.Parse(os.Args[1:]); err != nil {
		if errors.Is(err, pflag.ErrHelp) {
			return nil
		}

		return err
	}

	fmt.Println("create meilisearch client")

	client := meilisearch.New(*meilisearchURL, meilisearch.WithAPIKey(*meilisearchKey), meilisearch.WithCustomClient(&http.Client{Timeout: *meilisearchTimeout}))

	exp := &exporter{client: client}

	prometheus.DefaultRegisterer.MustRegister(exp)
	// prometheus.MustRegister(taskProcessDelay)

	http.Handle("/metrics", promhttp.Handler())

	log.Fatal(http.ListenAndServe(*addr, nil))
	return nil
}

type exporter struct {
	client meilisearch.ServiceManager
}

func (e *exporter) Describe(_ chan<- *prometheus.Desc) {}

func (e *exporter) Collect(ch chan<- prometheus.Metric) {
	enqueued, processing, done, err := e.fetchTasks()
	if err != nil {
		logrus.Error("failed to fetch current task", err)
	}

	e.processTasks(ch, enqueued, processing, done)
}

var processingTaskID = prometheus.NewGauge(prometheus.GaugeOpts{
	Namespace:   "meilisearch",
	Name:        "task_id",
	Help:        "meilisearch processing task delay",
	ConstLabels: map[string]string{"status": string(meilisearch.TaskStatusProcessing)},
})

var finishedTaskID = prometheus.NewGauge(prometheus.GaugeOpts{
	Namespace:   "meilisearch",
	Name:        "task_id",
	Help:        "meilisearch processing task delay",
	ConstLabels: map[string]string{"status": string(meilisearch.TaskStatusSucceeded)},
})

var enqueuedTaskID = prometheus.NewGauge(prometheus.GaugeOpts{
	Namespace:   "meilisearch",
	Name:        "task_id",
	Help:        "meilisearch processing task delay",
	ConstLabels: map[string]string{"status": string(meilisearch.TaskStatusEnqueued)},
})

var taskProcessDelay = prometheus.NewGauge(prometheus.GaugeOpts{
	Namespace: "meilisearch",
	Name:      "task_delay_seconds",
	Help:      "meilisearch processing task delay",
})

var taskUnfinishedCount = prometheus.NewGauge(prometheus.GaugeOpts{
	Namespace: "meilisearch",
	Name:      "task_unfinished_count",
	Help:      "meilisearch enqueued tasks",
})

func (e *exporter) processTasks(ch chan<- prometheus.Metric, enqueued *meilisearch.Task, processing *meilisearch.Task, finished *meilisearch.Task) {
	if processing != nil {
		d := time.Since(processing.EnqueuedAt)
		taskProcessDelay.Set(d.Seconds())
	} else {
		if enqueued == nil {
			taskProcessDelay.Set(0)
		}
	}

	if enqueued != nil && finished != nil {
		taskUnfinishedCount.Set(float64(enqueued.UID - finished.UID))
		ch <- taskUnfinishedCount
	} else {
		taskUnfinishedCount.Set(0)
		ch <- taskUnfinishedCount
	}

	ch <- taskProcessDelay

	if processing != nil {
		processingTaskID.Set(float64(processing.UID))
		ch <- processingTaskID
	}

	if enqueued != nil {
		enqueuedTaskID.Set(float64(enqueued.UID))
		ch <- enqueuedTaskID
	}

	if finished != nil {
		finishedTaskID.Set(float64(finished.UID))
		ch <- finishedTaskID
	}
}

func (e *exporter) fetchTasks() (enqueued *meilisearch.Task, processing *meilisearch.Task, finished *meilisearch.Task, err error) {
	var g = errgroup.Group{}

	g.Go(func() error {
		tasks, err := e.client.Index(*meilisearchIndex).GetTasks(&meilisearch.TasksQuery{
			Limit:    1,
			Statuses: []meilisearch.TaskStatus{meilisearch.TaskStatusProcessing},
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
			Limit:    1,
			Statuses: []meilisearch.TaskStatus{meilisearch.TaskStatusEnqueued},
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
			Limit:    1,
			Statuses: []meilisearch.TaskStatus{meilisearch.TaskStatusSucceeded},
		})
		if err != nil {
			return err
		}

		if len(tasks.Results) != 0 {
			top := tasks.Results[0]
			finished = &top
		}

		return nil
	})

	err = g.Wait()

	return
}
