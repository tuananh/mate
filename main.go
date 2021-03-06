package main

import (
	log "github.com/Sirupsen/logrus"
	"gopkg.in/alecthomas/kingpin.v2"

	"github.com/zalando-incubator/mate/consumers"
	"github.com/zalando-incubator/mate/controller"
	"github.com/zalando-incubator/mate/producers"
)

var params struct {
	producer string
	consumer string
	debug    bool
	syncOnly bool
}

var version = "Unknown"

func init() {
	kingpin.Flag("producer", "The endpoints producer to use.").Required().StringVar(&params.producer)
	kingpin.Flag("consumer", "The endpoints consumer to use.").Required().StringVar(&params.consumer)
	kingpin.Flag("debug", "Enable debug logging.").BoolVar(&params.debug)
	kingpin.Flag("sync-only", "Disable event watcher").BoolVar(&params.syncOnly)
}

func main() {
	kingpin.Version(version)
	kingpin.Parse()

	if params.debug {
		log.SetLevel(log.DebugLevel)
	}

	p, err := producers.New(params.producer)
	if err != nil {
		log.Fatalf("Error creating producer: %v", err)
	}

	c, err := consumers.NewSynced(params.consumer)
	if err != nil {
		log.Fatalf("Error creating consumer: %v", err)
	}

	opts := &controller.Options{
		SyncOnly: params.syncOnly,
	}
	ctrl := controller.New(p, c, opts)
	errors := ctrl.Run()

	go func() {
		for {
			log.Error(<-errors)
		}
	}()

	ctrl.Wait()
}
